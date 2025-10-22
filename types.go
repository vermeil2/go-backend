package main

import (
	"time"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type CreateContainerRequest struct {
	Image    string   `json:"image"`
	Name     string   `json:"name"`
	Cmd      []string `json:"cmd"`
	Env      []string `json:"env"`
	Platform string   `json:"platform"` // e.g., "linux/amd64" (optional)
}

type BuildImageRequest struct {
	ImageName   string `json:"image_name"`
	Dockerfile  string `json:"dockerfile"`      // Dockerfile content
	ContextPath string `json:"context_path"`    // default "." (server-side path)
	Platform    string `json:"platform"`        // optional, e.g., linux/amd64
}

type ComposeFileItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type ComposeFileUploadRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ComposeRunRequest struct {
	FilePath string            `json:"file_path"` // absolute or server-relative
	WorkDir  string            `json:"work_dir"`  // optional; defaults to file dir
	Env      map[string]string `json:"env"`       // optional
	Args     []string          `json:"args"`      // optional extra args
}

type ComposeScaleRequest struct {
	FilePath string `json:"file_path"`
	WorkDir  string `json:"work_dir"`
	Service  string `json:"service"`
	Replicas int    `json:"replicas"`
}

type ExecRequest struct {
	Cmd []string `json:"cmd"`
}

// Volume file system browsing
type VolumeFileInfo struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	IsDir       bool      `json:"is_dir"`
	Size        int64     `json:"size"`
	Mode        string    `json:"mode"`
	ModTime     time.Time `json:"mod_time"`
	Permissions string    `json:"permissions"`
}

