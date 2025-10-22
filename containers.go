package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	imageapi "github.com/docker/docker/api/types/image"
	imageTypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/filters"
	"github.com/gorilla/mux"

)

func ListContainersHandler(w http.ResponseWriter, r *http.Request) {
	cli, err := NewDockerClient()
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Include stopped containers if all=true query provided
	showAll := r.URL.Query().Get("all") == "true"
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: showAll})
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
WriteJSON(w, http.StatusOK, containers)
}

func CreateContainerHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	if req.Image == "" {
WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "image is required"})
		return
	}

	cli, err := NewDockerClient()
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
		if !hasLocal && !ContainsColon(req.Image) {
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
			if !ContainsSlash(req.Image) {
				rc2, secondErr := cli.ImagePull(ctx, "docker.io/library/"+req.Image, pullOpts)
				if secondErr != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
					return
				}
				defer rc2.Close()
				_, _ = io.Copy(io.Discard, rc2)
				// 성공적으로 pull했다면, 실제 사용 이미지명을 보정(:latest 자동)
				if !ContainsColon(req.Image) {
					req.Image = req.Image + ":latest"
				}
			} else {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
WriteJSON(w, http.StatusCreated, resp)
}

func StartContainerHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	cli, err := NewDockerClient()
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	if err := cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
WriteJSON(w, http.StatusOK, map[string]string{"status": "started", "id": id})
}

func StopContainerHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	cli, err := NewDockerClient()
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	t := 10 // seconds
	if err := cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &t}); err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
WriteJSON(w, http.StatusOK, map[string]string{"status": "stopped", "id": id})
}

func RestartContainerHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	cli, err := NewDockerClient()
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if err := cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: nil}); err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return
	}
WriteJSON(w, http.StatusOK, map[string]string{"status": "restarted", "id": id})
}

func DeleteContainerHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	cli, err := NewDockerClient()
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func InspectContainerHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	cli, err := NewDockerClient()
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	info, err := cli.ContainerInspect(ctx, id)
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
WriteJSON(w, http.StatusOK, info)
}

func ContainerLogsHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	cli, err := NewDockerClient()
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
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
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func ExecInContainerHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"}); return
	}
	if len(req.Cmd) == 0 { WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "cmd required"}); return }

	cli, err := NewDockerClient()
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	execCfg := container.ExecOptions{
		Cmd:          req.Cmd,
		AttachStdout: true,
		AttachStderr: true,
	}
	execID, err := cli.ContainerExecCreate(ctx, id, execCfg)
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	attach, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	defer attach.Close()

	out, _ := io.ReadAll(attach.Reader)
WriteJSON(w, http.StatusOK, map[string]any{"output": string(out)})
}

func ContainerStatsHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	cli, err := NewDockerClient()
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Stream=false -> one-shot stats
	rc, err := cli.ContainerStats(ctx, id, false)
	if err != nil { WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return }
	defer rc.Body.Close()

	var s dockerTypes.StatsJSON
	if err := json.NewDecoder(rc.Body).Decode(&s); err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()}); return
	}

	// Calculate CPU % roughly (Docker CLI style simplified)
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemUsage - s.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(s.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	memUsage := float64(s.MemoryStats.Usage)
	memLimit := float64(s.MemoryStats.Limit)
	memPercent := 0.0
	if memLimit > 0 { memPercent = (memUsage / memLimit) * 100.0 }

WriteJSON(w, http.StatusOK, map[string]any{
		"cpu_percent": cpuPercent,
		"mem_usage": memUsage,
		"mem_limit": memLimit,
		"mem_percent": memPercent,
		"pids": s.PidsStats.Current,
		"net": s.Networks,
		"blkio": s.BlkioStats,
	})
}

func PruneStoppedContainersHandler(w http.ResponseWriter, r *http.Request) {
	cli, err := NewDockerClient()
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	report, err := cli.ContainersPrune(ctx, filters.Args{})
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
WriteJSON(w, http.StatusOK, report)
}
