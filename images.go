package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"time"

	imageapi "github.com/docker/docker/api/types/image"
	"github.com/gorilla/mux"

)

func ListImagesHandler(w http.ResponseWriter, r *http.Request) {
	cli, err := NewDockerClient()
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	images, err := cli.ImageList(ctx, imageapi.ListOptions{})
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
WriteJSON(w, http.StatusOK, images)
}

func BuildImageHandler(w http.ResponseWriter, r *http.Request) {
	var req BuildImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid JSON body"})
		return
	}
	if req.ImageName == "" || req.Dockerfile == "" {
WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "image_name and dockerfile are required"})
		return
	}

	// Create temp Dockerfile
	tmpFile, err := os.CreateTemp("", "Dockerfile_*.tmp")
	if err != nil {
WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
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
WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"output":  string(output),
			"error":   err.Error(),
		})
		return
	}

WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"output":  string(output),
		"image":   req.ImageName,
	})
}

// DELETE /go/images/{ref}?force=true&pruneChildren=true
func DeleteImageHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ref := vars["ref"]
	if ref == "" {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "image ref required"})
		return
	}
	cli, err := NewDockerClient()
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	force := r.URL.Query().Get("force") == "true"
	pruneChildren := r.URL.Query().Get("pruneChildren") == "true"

	_, err = cli.ImageRemove(ctx, ref, imageapi.RemoveOptions{Force: force, PruneChildren: pruneChildren})
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted", "ref": ref})
}
