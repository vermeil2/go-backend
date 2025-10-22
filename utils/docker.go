package utils

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/client"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func NewDockerClient() (*client.Client, error) {
	// Works with Docker Desktop on Windows/macOS/Linux using env or defaults
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

// containsColon returns true if image reference contains a tag delimiter ':' (not counting digest '@').
func ContainsColon(ref string) bool {
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

func ContainsSlash(ref string) bool {
	for i := 0; i < len(ref); i++ {
		if ref[i] == '/' {
			return true
		}
	}
	return false
}

