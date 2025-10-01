package main

import (
    "context"
    "encoding/json"
    "io"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "github.com/docker/docker/api/types/container"
    imageapi "github.com/docker/docker/api/types/image"
    "os/exec"
    imageTypes "github.com/docker/docker/api/types/image"
    "github.com/docker/docker/api/types/filters"
    "github.com/docker/docker/client"
    "github.com/gorilla/mux"
    "github.com/rs/cors"
    "fmt"
)

type errorResponse struct {
    Error string `json:"error"`
}

type createContainerRequest struct {
    Image string   `json:"image"`
    Name  string   `json:"name"`
    Cmd   []string `json:"cmd"`
    Env   []string `json:"env"`
    Platform string `json:"platform"` // e.g., "linux/amd64" (optional)
}

type buildImageRequest struct {
    ImageName   string `json:"image_name"`
    Dockerfile  string `json:"dockerfile"`      // Dockerfile content
    ContextPath string `json:"context_path"`    // default "." (server-side path)
    Platform    string `json:"platform"`        // optional, e.g., linux/amd64
}

type composeFileItem struct {
    Name string `json:"name"`
    Path string `json:"path"`
}

type composeFileUploadRequest struct {
    Name    string `json:"name"`
    Content string `json:"content"`
}

type composeRunRequest struct {
    FilePath string            `json:"file_path"` // absolute or server-relative
    WorkDir  string            `json:"work_dir"`  // optional; defaults to file dir
    Env      map[string]string `json:"env"`       // optional
    Args     []string          `json:"args"`      // optional extra args
}

type composeScaleRequest struct {
    FilePath string `json:"file_path"`
    WorkDir  string `json:"work_dir"`
    Service  string `json:"service"`
    Replicas int    `json:"replicas"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(v)
}

func newDockerClient() (*client.Client, error) {
    // Works with Docker Desktop on Windows/macOS/Linux using env or defaults
    return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

func listContainersHandler(w http.ResponseWriter, r *http.Request) {
    cli, err := newDockerClient()
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    // Include stopped containers if all=true query provided
    showAll := r.URL.Query().Get("all") == "true"
    containers, err := cli.ContainerList(ctx, container.ListOptions{All: showAll})
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, containers)
}

func createContainerHandler(w http.ResponseWriter, r *http.Request) {
    var req createContainerRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
        return
    }
    if req.Image == "" {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: "image is required"})
        return
    }

    cli, err := newDockerClient()
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
    defer cancel()

    // 1) 로컬에 이미지가 있는지 먼저 확인 (있으면 pull 스킵)
    hasLocal := false
    {
        args := filters.NewArgs()
        // reference 필터는 태그 포함 문자열로 매칭됩니다.
        args.Add("reference", req.Image)
        imgs, err := cli.ImageList(ctx, imageapi.ListOptions{Filters: args})
        if err == nil && len(imgs) > 0 {
            hasLocal = true
        }
        // 태그가 생략된 경우 :latest로도 한 번 더 확인
        if !hasLocal && !containsColon(req.Image) {
            args2 := filters.NewArgs()
            args2.Add("reference", req.Image+":latest")
            imgs2, err2 := cli.ImageList(ctx, imageapi.ListOptions{Filters: args2})
            if err2 == nil && len(imgs2) > 0 {
                hasLocal = true
                req.Image = req.Image + ":latest"
            }
        }
    }

    // 2) 로컬에 없을 때만 pull 시도
    if !hasLocal {
        pullOpts := imageTypes.PullOptions{}
        if req.Platform != "" { pullOpts.Platform = req.Platform }
        rc, err := cli.ImagePull(ctx, req.Image, pullOpts)
        if err != nil {
            // 슬래시가 없는 단순 이름이면 library 프리픽스도 시도
            if !containsSlash(req.Image) {
                rc2, secondErr := cli.ImagePull(ctx, "docker.io/library/"+req.Image, pullOpts)
                if secondErr != nil {
                    writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
                    return
                }
                defer rc2.Close()
                _, _ = io.Copy(io.Discard, rc2)
                // 성공적으로 pull했다면, 실제 사용 이미지명을 보정(:latest 자동)
                if !containsColon(req.Image) {
                    req.Image = req.Image + ":latest"
                }
            } else {
                writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
                return
            }
        } else {
            defer rc.Close()
            _, _ = io.Copy(io.Discard, rc)
        }
    }

    resp, err := cli.ContainerCreate(
        ctx,
        &container.Config{Image: req.Image, Cmd: req.Cmd, Env: req.Env, Tty: false},
        &container.HostConfig{},
        nil,
        nil,
        req.Name,
    )
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    writeJSON(w, http.StatusCreated, resp)
}

// containsColon returns true if image reference contains a tag delimiter ':' (not counting digest '@').
func containsColon(ref string) bool {
    for i := 0; i < len(ref); i++ {
        if ref[i] == ':' {
            return true
        }
        if ref[i] == '@' { // digest case, stop early
            return false
        }
    }
    return false
}

func containsSlash(ref string) bool {
    for i := 0; i < len(ref); i++ {
        if ref[i] == '/' {
            return true
        }
    }
    return false
}

func startContainerHandler(w http.ResponseWriter, r *http.Request) {
    id := mux.Vars(r)["id"]
    cli, err := newDockerClient()
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
    defer cancel()

    if err := cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "started", "id": id})
}

func stopContainerHandler(w http.ResponseWriter, r *http.Request) {
    id := mux.Vars(r)["id"]
    cli, err := newDockerClient()
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
    defer cancel()

    t := 10 // seconds
    if err := cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &t}); err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "stopped", "id": id})
}

func restartContainerHandler(w http.ResponseWriter, r *http.Request) {
    id := mux.Vars(r)["id"]
    cli, err := newDockerClient()
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
    defer cancel()

    if err := cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: nil}); err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "restarted", "id": id})
}

func deleteContainerHandler(w http.ResponseWriter, r *http.Request) {
    id := mux.Vars(r)["id"]
    cli, err := newDockerClient()
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
    defer cancel()

    if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func inspectContainerHandler(w http.ResponseWriter, r *http.Request) {
    id := mux.Vars(r)["id"]
    cli, err := newDockerClient()
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
    defer cancel()

    info, err := cli.ContainerInspect(ctx, id)
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    writeJSON(w, http.StatusOK, info)
}

func containerLogsHandler(w http.ResponseWriter, r *http.Request) {
    id := mux.Vars(r)["id"]
    cli, err := newDockerClient()
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
    defer cancel()

    // query: tail, stdout, stderr
    tail := r.URL.Query().Get("tail")
    if tail == "" { tail = "200" }
    showStdout := r.URL.Query().Get("stdout") != "false"
    showStderr := r.URL.Query().Get("stderr") != "false"

    opts := container.LogsOptions{ShowStdout: showStdout, ShowStderr: showStderr, Tail: tail}
    rc, err := cli.ContainerLogs(ctx, id, opts)
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    defer rc.Close()
    b, _ := io.ReadAll(rc)
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write(b)
}

type execRequest struct {
    Cmd []string `json:"cmd"`
}

func execInContainerHandler(w http.ResponseWriter, r *http.Request) {
    id := mux.Vars(r)["id"]
    var req execRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"}); return
    }
    if len(req.Cmd) == 0 { writeJSON(w, http.StatusBadRequest, errorResponse{Error: "cmd required"}); return }

    cli, err := newDockerClient()
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
    defer cancel()

    execCfg := container.ExecOptions{
        Cmd:          req.Cmd,
        AttachStdout: true,
        AttachStderr: true,
    }
    execID, err := cli.ContainerExecCreate(ctx, id, execCfg)
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    attach, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    defer attach.Close()

    out, _ := io.ReadAll(attach.Reader)
    writeJSON(w, http.StatusOK, map[string]any{"output": string(out)})
}
func pruneStoppedContainersHandler(w http.ResponseWriter, r *http.Request) {
    cli, err := newDockerClient()
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
    defer cancel()

    report, err := cli.ContainersPrune(ctx, filters.Args{})
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, report)
}

func listImagesHandler(w http.ResponseWriter, r *http.Request) {
    cli, err := newDockerClient()
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    defer cli.Close()

    ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
    defer cancel()

    images, err := cli.ImageList(ctx, imageapi.ListOptions{})
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, images)
}

// --- Compose helpers ---
func composeBaseDir() (string, error) {
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

func safeJoin(base, name string) (string, error) {
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

func buildImageHandler(w http.ResponseWriter, r *http.Request) {
    var req buildImageRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
        return
    }
    if req.ImageName == "" || req.Dockerfile == "" {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: "image_name and dockerfile are required"})
        return
    }

    // Create temp Dockerfile
    tmpFile, err := os.CreateTemp("", "Dockerfile_*.tmp")
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
        return
    }
    tmpPath := tmpFile.Name()
    _, _ = tmpFile.WriteString(req.Dockerfile)
    _ = tmpFile.Close()
    defer os.Remove(tmpPath)

    ctxPath := req.ContextPath
    if ctxPath == "" {
        ctxPath = "."
    }

    // Use docker CLI for build to leverage local context and ignore rules
    cmd := exec.Command("docker", "build", "-t", req.ImageName, "-f", tmpPath, ctxPath)
    if req.Platform != "" {
        cmd = exec.Command("docker", "build", "--platform", req.Platform, "-t", req.ImageName, "-f", tmpPath, ctxPath)
    }

    output, err := cmd.CombinedOutput()
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, map[string]any{
            "success": false,
            "output":  string(output),
            "error":   err.Error(),
        })
        return
    }

    writeJSON(w, http.StatusOK, map[string]any{
        "success": true,
        "output":  string(output),
        "image":   req.ImageName,
    })
}

func composeListFilesHandler(w http.ResponseWriter, r *http.Request) {
    base, err := composeBaseDir()
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    entries, err := os.ReadDir(base)
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    items := []composeFileItem{}
    for _, e := range entries {
        if e.IsDir() { continue }
        name := e.Name()
        if filepath.Ext(name) == ".yml" || filepath.Ext(name) == ".yaml" {
            p := filepath.Join(base, name)
            items = append(items, composeFileItem{Name: name, Path: p})
        }
    }
    writeJSON(w, http.StatusOK, items)
}

func composeUploadFileHandler(w http.ResponseWriter, r *http.Request) {
    var req composeFileUploadRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"}); return
    }
    if req.Name == "" || req.Content == "" {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: "name and content required"}); return
    }
    base, err := composeBaseDir()
    if err != nil { writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return }
    dest, err := safeJoin(base, req.Name)
    if err != nil { writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); return }
    if err := os.WriteFile(dest, []byte(req.Content), 0o644); err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()}); return
    }
    writeJSON(w, http.StatusOK, map[string]any{"path": dest})
}

func composeRun(w http.ResponseWriter, subcmd string, req composeRunRequest) {
    filePath := req.FilePath
    if filePath == "" {
        writeJSON(w, http.StatusBadRequest, errorResponse{Error: "file_path required"}); return
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
        writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "output": string(out), "error": err.Error()}); return
    }
    writeJSON(w, http.StatusOK, map[string]any{"success": true, "output": string(out)})
}

func composeUpHandler(w http.ResponseWriter, r *http.Request) {
    var req composeRunRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"}); return }
    req.Args = append(req.Args, "-d")
    composeRun(w, "up", req)
}

func composeDownHandler(w http.ResponseWriter, r *http.Request) {
    var req composeRunRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"}); return }
    composeRun(w, "down", req)
}

func composePsHandler(w http.ResponseWriter, r *http.Request) {
    var req composeRunRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"}); return }
    composeRun(w, "ps", req)
}

func composeLogsHandler(w http.ResponseWriter, r *http.Request) {
    var req composeRunRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"}); return }
    // 기본 tail 200줄
    if len(req.Args) == 0 { req.Args = []string{"--no-color", "--tail", "200"} }
    composeRun(w, "logs", req)
}

func composeScaleHandler(w http.ResponseWriter, r *http.Request) {
    var req composeScaleRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"}); return }
    if req.Service == "" || req.Replicas < 0 { writeJSON(w, http.StatusBadRequest, errorResponse{Error: "service and replicas required"}); return }
    runReq := composeRunRequest{FilePath: req.FilePath, WorkDir: req.WorkDir}
    runReq.Args = []string{"--no-recreate", "--detach", "--scale", fmt.Sprintf("%s=%d", req.Service, req.Replicas)}
    composeRun(w, "up", runReq)
}
func routes() http.Handler {
    r := mux.NewRouter()
    api := r.PathPrefix("/go").Subrouter()
    api.HandleFunc("/containers", listContainersHandler).Methods(http.MethodGet)
    api.HandleFunc("/containers", createContainerHandler).Methods(http.MethodPost)
    api.HandleFunc("/containers/{id}/start", startContainerHandler).Methods(http.MethodPost)
    api.HandleFunc("/containers/{id}/stop", stopContainerHandler).Methods(http.MethodPost)
    api.HandleFunc("/containers/{id}/restart", restartContainerHandler).Methods(http.MethodPost)
    api.HandleFunc("/containers/{id}", deleteContainerHandler).Methods(http.MethodDelete)
    api.HandleFunc("/containers/{id}/inspect", inspectContainerHandler).Methods(http.MethodGet)
    api.HandleFunc("/containers/{id}/logs", containerLogsHandler).Methods(http.MethodGet)
    api.HandleFunc("/containers/{id}/exec", execInContainerHandler).Methods(http.MethodPost)
    api.HandleFunc("/containers/prune", pruneStoppedContainersHandler).Methods(http.MethodPost)
    api.HandleFunc("/images", listImagesHandler).Methods(http.MethodGet)
    api.HandleFunc("/images/build", buildImageHandler).Methods(http.MethodPost)

    // Compose endpoints
    api.HandleFunc("/compose/files", composeListFilesHandler).Methods(http.MethodGet)
    api.HandleFunc("/compose/files", composeUploadFileHandler).Methods(http.MethodPost)
    api.HandleFunc("/compose/up", composeUpHandler).Methods(http.MethodPost)
    api.HandleFunc("/compose/down", composeDownHandler).Methods(http.MethodPost)
    api.HandleFunc("/compose/ps", composePsHandler).Methods(http.MethodPost)
    api.HandleFunc("/compose/logs", composeLogsHandler).Methods(http.MethodPost)
    api.HandleFunc("/compose/scale", composeScaleHandler).Methods(http.MethodPost)
    return r
}

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8081"
    }

    handler := routes()
    c := cors.New(cors.Options{
        AllowedOrigins:   []string{"http://localhost:3000", "http://127.0.0.1:3000"},
        AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodOptions},
        AllowedHeaders:   []string{"*"},
        AllowCredentials: true,
    })

    srv := &http.Server{
        Addr:              ":" + port,
        Handler:           c.Handler(handler),
        ReadTimeout:       15 * time.Second,
        ReadHeaderTimeout: 15 * time.Second,
        WriteTimeout:      60 * time.Second,
        IdleTimeout:       60 * time.Second,
    }

    log.Printf("Go Docker backend listening on %s", srv.Addr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("server error: %v", err)
    }
}


