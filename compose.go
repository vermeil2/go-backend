package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

)

// --- Compose helpers ---
func ComposeBaseDir() (string, error) {
	base := os.Getenv("COMPOSE_DIR")
	if base == "" {
		wd, err := os.Getwd()
		if err != nil { return "", err }
		base = filepath.Join(wd, "compose")
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	return base, nil
}

func SafeJoin(base, name string) (string, error) {
	p := filepath.Join(base, name)
	rp, err := filepath.Abs(p)
	if err != nil { return "", err }
	rb, err := filepath.Abs(base)
	if err != nil { return "", err }
	if len(rp) < len(rb) || rp[:len(rb)] != rb {
		return "", fmt.Errorf("path escapes base")
	}
	return rp, nil
}

func ComposeListFilesHandler(w http.ResponseWriter, r *http.Request) {
	base, err := ComposeBaseDir()
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	
	recursive := r.URL.Query().Get("recursive") == "true"
	
	items := []ComposeFileItem{}
	var scan func(string) error
	scan = func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil { return err }
		for _, e := range entries {
			name := e.Name()
			fullPath := filepath.Join(dir, name)
			relPath, _ := filepath.Rel(base, fullPath)
			
			if e.IsDir() {
				if recursive {
					if err := scan(fullPath); err != nil {
						return err
					}
				}
				continue
			}
			
			items = append(items, ComposeFileItem{Name: relPath, Path: fullPath})
		}
		return nil
	}
	if err := scan(base); err != nil {
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, items)
}

func ComposeUploadFileHandler(w http.ResponseWriter, r *http.Request) {
	var req ComposeFileUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"}); return
	}
	if req.Name == "" || req.Content == "" {
WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "name and content required"}); return
	}
	base, err := ComposeBaseDir()
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	dest, err := SafeJoin(base, req.Name)
	if err != nil { WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()}); return }
	if err := os.WriteFile(dest, []byte(req.Content), 0o644); err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return
	}
WriteJSON(w, http.StatusOK, map[string]any{"path": dest})
}

// GET /go/compose/file?path=...
// Returns { name, content }
func ComposeGetFileHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "path required"}); return
	}
	// Ensure the requested path is inside compose base dir
	base, err := ComposeBaseDir()
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	// If user passed absolute path inside base, verify; if only name, join
	var target string
	if filepath.IsAbs(path) {
		target = path
	} else {
		target, err = SafeJoin(base, path)
		if err != nil { WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()}); return }
	}
	// Check containment
	absBase, _ := filepath.Abs(base)
	absTarget, _ := filepath.Abs(target)
	if len(absTarget) < len(absBase) || absTarget[:len(absBase)] != absBase {
WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "path escapes base"}); return
	}
	b, err := os.ReadFile(absTarget)
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	name := filepath.Base(absTarget)
WriteJSON(w, http.StatusOK, map[string]any{"name": name, "content": string(b)})
}

func ComposeRun(w http.ResponseWriter, subcmd string, req ComposeRunRequest) {
	filePath := req.FilePath
	if filePath == "" {
WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "file_path required"}); return
	}
	workDir := req.WorkDir
	if workDir == "" { workDir = filepath.Dir(filePath) }
	args := []string{"compose", "-f", filePath, subcmd}
	if len(req.Args) > 0 { args = append(args, req.Args...) }
	cmd := exec.Command("docker", args...)
	cmd.Dir = workDir
	if len(req.Env) > 0 {
		env := os.Environ()
		for k, v := range req.Env { env = append(env, fmt.Sprintf("%s=%s", k, v)) }
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "output": string(out), "error": err.Error()}); return
	}
WriteJSON(w, http.StatusOK, map[string]any{"success": true, "output": string(out)})
}

func ComposeUpHandler(w http.ResponseWriter, r *http.Request) {
	var req ComposeRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"}); return }
	req.Args = append(req.Args, "-d")
	ComposeRun(w, "up", req)
}

func ComposeDownHandler(w http.ResponseWriter, r *http.Request) {
	var req ComposeRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"}); return }
	ComposeRun(w, "down", req)
}

func ComposePsHandler(w http.ResponseWriter, r *http.Request) {
	var req ComposeRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"}); return }
	ComposeRun(w, "ps", req)
}

func ComposeLogsHandler(w http.ResponseWriter, r *http.Request) {
	var req ComposeRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"}); return }
	// 기본 tail 200줄
	if len(req.Args) == 0 { req.Args = []string{"--no-color", "--tail", "200"} }
	ComposeRun(w, "logs", req)
}

func ComposeScaleHandler(w http.ResponseWriter, r *http.Request) {
	var req ComposeScaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"}); return }
	if req.Service == "" || req.Replicas < 0 { WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "service and replicas required"}); return }
	runReq := ComposeRunRequest{FilePath: req.FilePath, WorkDir: req.WorkDir}
	runReq.Args = []string{"--no-recreate", "--detach", "--scale", fmt.Sprintf("%s=%d", req.Service, req.Replicas)}
	ComposeRun(w, "up", runReq)
}
