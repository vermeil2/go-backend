package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go-backend/types"
	"go-backend/utils"
)

type SaveFileRequest struct {
	FileName string `json:"fileName"`
	Content  string `json:"content"`
}

func SaveComposeFileHandler(w http.ResponseWriter, r *http.Request) {
	var req SaveFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, types.ErrorResponse{Error: "Invalid request body"})
		return
	}

	// 파일명 검증
	if req.FileName == "" {
		req.FileName = "docker-compose.yml"
	}

	// 확장자 추가 (없으면 .yml 추가)
	if !strings.HasSuffix(req.FileName, ".yml") && !strings.HasSuffix(req.FileName, ".yaml") {
		req.FileName += ".yml"
	}

	// compose 디렉토리 경로
	composeDir := "compose"
	if err := os.MkdirAll(composeDir, 0755); err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: fmt.Sprintf("Failed to create directory: %v", err)})
		return
	}

	// 파일 저장
	filePath := filepath.Join(composeDir, req.FileName)
	if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: fmt.Sprintf("Failed to save file: %v", err)})
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("File saved successfully: %s", req.FileName),
		"path":    filePath,
	})
}

func SaveNginxFileHandler(w http.ResponseWriter, r *http.Request) {
	var req SaveFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, types.ErrorResponse{Error: "Invalid request body"})
		return
	}

	// 파일명 검증
	if req.FileName == "" {
		req.FileName = "nginx.conf"
	}

	// 확장자 추가 (없으면 .conf 추가)
	if !strings.HasSuffix(req.FileName, ".conf") {
		req.FileName += ".conf"
	}

	// compose 디렉토리 경로 (nginx도 compose 폴더에 저장)
	composeDir := "compose"
	if err := os.MkdirAll(composeDir, 0755); err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: fmt.Sprintf("Failed to create directory: %v", err)})
		return
	}

	// 파일 저장
	filePath := filepath.Join(composeDir, req.FileName)
	if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, types.ErrorResponse{Error: fmt.Sprintf("Failed to save file: %v", err)})
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("File saved successfully: %s", req.FileName),
		"path":    filePath,
	})
}
