package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/cors"
)

// splitAndTrim splits a string by delimiter and trims whitespace from each part
func splitAndTrim(s, delim string) []string {
	parts := strings.Split(s, delim)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	handler := routes()
	// CORS 허용 오리진을 환경 변수에서 읽거나 기본값 사용
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:3000,http://127.0.0.1:3000"
	}
	origins := []string{}
	for _, origin := range splitAndTrim(corsOrigins, ",") {
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	}
	c := cors.New(cors.Options{
		AllowedOrigins:   origins,
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