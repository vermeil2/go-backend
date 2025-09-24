package main

import (
    "context"
    "encoding/json"
    "io"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/docker/docker/api/types/container"
    imageapi "github.com/docker/docker/api/types/image"
    "os/exec"
    imageTypes "github.com/docker/docker/api/types/image"
    "github.com/docker/docker/api/types/filters"
    "github.com/docker/docker/client"
    "github.com/gorilla/mux"
    "github.com/rs/cors"
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

    // Ensure image exists (pull if missing)
    pullOpts := imageTypes.PullOptions{}
    if req.Platform != "" { pullOpts.Platform = req.Platform }
    rc, err := cli.ImagePull(ctx, req.Image, pullOpts)
    if err != nil {
        // Try with library prefix if simple name
        rc2, secondErr := cli.ImagePull(ctx, "docker.io/library/"+req.Image, pullOpts)
        if secondErr != nil {
            writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
            return
        }
        defer rc2.Close()
        _, _ = io.Copy(io.Discard, rc2)
    } else {
        defer rc.Close()
        _, _ = io.Copy(io.Discard, rc)
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

func routes() http.Handler {
    r := mux.NewRouter()
    api := r.PathPrefix("/go").Subrouter()
    api.HandleFunc("/containers", listContainersHandler).Methods(http.MethodGet)
    api.HandleFunc("/containers", createContainerHandler).Methods(http.MethodPost)
    api.HandleFunc("/containers/{id}/start", startContainerHandler).Methods(http.MethodPost)
    api.HandleFunc("/containers/{id}/stop", stopContainerHandler).Methods(http.MethodPost)
    api.HandleFunc("/containers/{id}", deleteContainerHandler).Methods(http.MethodDelete)
    api.HandleFunc("/containers/prune", pruneStoppedContainersHandler).Methods(http.MethodPost)
    api.HandleFunc("/images", listImagesHandler).Methods(http.MethodGet)
    api.HandleFunc("/images/build", buildImageHandler).Methods(http.MethodPost)
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


