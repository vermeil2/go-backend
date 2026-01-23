package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"time"

	imageapi "github.com/docker/docker/api/types/image"

	"go-backend/types"
	"go-backend/utils"
)

func ListImagesHandler(w http.ResponseWriter, r *http.Request) {
	cli, err := utils.NewDockerClient()
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	images, err := cli.ImageList(ctx, imageapi.ListOptions{})
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	utils.WriteJSON(w, http.StatusOK, images)
}

func BuildImageHandler(w http.ResponseWriter, r *http.Request) {
	var req types.BuildImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, types.ErrorResponse{Error: "invalid JSON body"})
		return
	}
	if req.ImageName == "" || req.Dockerfile == "" {
		utils.WriteJSON(w, http.StatusBadRequest, types.ErrorResponse{Error: "image_name and dockerfile are required"})
		return
	}

	// Create temp Dockerfile
	tmpFile, err := os.CreateTemp("", "Dockerfile_*.tmp")
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
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
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"output":  string(output),
			"error":   err.Error(),
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"output":  string(output),
		"image":   req.ImageName,
	})
}
