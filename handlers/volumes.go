package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/filters"
	volumeapi "github.com/docker/docker/api/types/volume"
	"github.com/gorilla/mux"

	"go-backend/types"
	"go-backend/utils"
)

// Volume management handlers
func ListVolumesHandler(w http.ResponseWriter, r *http.Request) {
	cli, err := utils.NewDockerClient()
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	volumes, err := cli.VolumeList(ctx, volumeapi.ListOptions{})
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	utils.WriteJSON(w, http.StatusOK, volumes)
}

func InspectVolumeHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	cli, err := utils.NewDockerClient()
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	volume, err := cli.VolumeInspect(ctx, name)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	utils.WriteJSON(w, http.StatusOK, volume)
}

func CreateVolumeHandler(w http.ResponseWriter, r *http.Request) {
	var req volumeapi.CreateOptions
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, types.ErrorResponse{Error: "invalid JSON body"})
		return
	}

	cli, err := utils.NewDockerClient()
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	volume, err := cli.VolumeCreate(ctx, req)
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	utils.WriteJSON(w, http.StatusCreated, volume)
}

func DeleteVolumeHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	cli, err := utils.NewDockerClient()
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := cli.VolumeRemove(ctx, name, true); err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})
}

func PruneVolumesHandler(w http.ResponseWriter, r *http.Request) {
	cli, err := utils.NewDockerClient()
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	report, err := cli.VolumesPrune(ctx, filters.Args{})
	if err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: err.Error()})
		return
	}
	utils.WriteJSON(w, http.StatusOK, report)
}

// Volume file system browsing
func BrowseVolumeHandler(w http.ResponseWriter, r *http.Request) {
	volumeName := mux.Vars(r)["name"]
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	log.Printf("Browsing volume %s at path %s", volumeName, path)

	// Use docker CLI directly for simplicity
	cmd := exec.Command("docker", "run", "--rm", "-v", fmt.Sprintf("%s:/volume", volumeName), "alpine:latest", "ls", "-la", fmt.Sprintf("/volume%s", path))

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Docker command failed: %v, output: %s", err, string(output))
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: fmt.Sprintf("Failed to browse volume: %v", err)})
		return
	}

	log.Printf("Docker output: %s", string(output))

	// Parse ls output
	files := ParseLsOutput(string(output), path)
	log.Printf("Parsed %d files", len(files))

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"path":  path,
		"files": files,
	})
}

func ParseLsOutput(output, currentPath string) []types.VolumeFileInfo {
	log.Printf("Parsing ls output: %s", output)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var files []types.VolumeFileInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "total") {
			continue
		}

		// Parse ls -la output format
		parts := strings.Fields(line)
		if len(parts) < 9 {
			log.Printf("Skipping line with insufficient parts: %s", line)
			continue
		}

		permissions := parts[0]
		sizeStr := parts[4]
		modTimeStr := strings.Join(parts[5:8], " ")
		name := strings.Join(parts[8:], " ")

		// Skip . and .. entries
		if name == "." || name == ".." {
			continue
		}

		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			log.Printf("Failed to parse size %s: %v", sizeStr, err)
			size = 0
		}

		isDir := strings.HasPrefix(permissions, "d")

		// Parse modification time
		modTime, err := time.Parse("Jan 2 15:04", modTimeStr)
		if err != nil {
			log.Printf("Failed to parse time %s: %v", modTimeStr, err)
			modTime = time.Now()
		} else {
			modTime = modTime.AddDate(time.Now().Year(), 0, 0) // Add current year
		}

		filePath := currentPath
		if filePath == "/" {
			filePath = "/" + name
		} else {
			filePath = filePath + "/" + name
		}

		fileInfo := types.VolumeFileInfo{
			Name:        name,
			Path:        filePath,
			IsDir:       isDir,
			Size:        size,
			Mode:        permissions,
			ModTime:     modTime,
			Permissions: permissions,
		}

		log.Printf("Parsed file: %+v", fileInfo)
		files = append(files, fileInfo)
	}

	log.Printf("Total parsed files: %d", len(files))
	return files
}
