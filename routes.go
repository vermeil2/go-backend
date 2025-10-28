package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func routes() http.Handler {
	r := mux.NewRouter()
	api := r.PathPrefix("/go").Subrouter()
	
	// Container endpoints
	api.HandleFunc("/containers", ListContainersHandler).Methods(http.MethodGet)
	api.HandleFunc("/containers", CreateContainerHandler).Methods(http.MethodPost)
	api.HandleFunc("/containers/{id}/start", StartContainerHandler).Methods(http.MethodPost)
	api.HandleFunc("/containers/{id}/stop", StopContainerHandler).Methods(http.MethodPost)
	api.HandleFunc("/containers/{id}/restart", RestartContainerHandler).Methods(http.MethodPost)
	api.HandleFunc("/containers/{id}", DeleteContainerHandler).Methods(http.MethodDelete)
	api.HandleFunc("/containers/{id}/inspect", InspectContainerHandler).Methods(http.MethodGet)
	api.HandleFunc("/containers/{id}/logs", ContainerLogsHandler).Methods(http.MethodGet)
	api.HandleFunc("/containers/{id}/exec", ExecInContainerHandler).Methods(http.MethodPost)
	api.HandleFunc("/containers/{id}/stats", ContainerStatsHandler).Methods(http.MethodGet)
	api.HandleFunc("/containers/prune", PruneStoppedContainersHandler).Methods(http.MethodPost)
	
	// Image endpoints
	api.HandleFunc("/images", ListImagesHandler).Methods(http.MethodGet)
	api.HandleFunc("/images/build", BuildImageHandler).Methods(http.MethodPost)
	api.HandleFunc("/images/{ref}", DeleteImageHandler).Methods(http.MethodDelete)

	// Compose endpoints
	api.HandleFunc("/compose/files", ComposeListFilesHandler).Methods(http.MethodGet)
	api.HandleFunc("/compose/files", ComposeUploadFileHandler).Methods(http.MethodPost)
	api.HandleFunc("/compose/file", ComposeGetFileHandler).Methods(http.MethodGet)
	api.HandleFunc("/compose/up", ComposeUpHandler).Methods(http.MethodPost)
	api.HandleFunc("/compose/down", ComposeDownHandler).Methods(http.MethodPost)
	api.HandleFunc("/compose/ps", ComposePsHandler).Methods(http.MethodPost)
	api.HandleFunc("/compose/logs", ComposeLogsHandler).Methods(http.MethodPost)
	api.HandleFunc("/compose/scale", ComposeScaleHandler).Methods(http.MethodPost)

	// Volume endpoints
	api.HandleFunc("/volumes", ListVolumesHandler).Methods(http.MethodGet)
	api.HandleFunc("/volumes", CreateVolumeHandler).Methods(http.MethodPost)
	api.HandleFunc("/volumes/{name}", InspectVolumeHandler).Methods(http.MethodGet)
	api.HandleFunc("/volumes/{name}", DeleteVolumeHandler).Methods(http.MethodDelete)
	api.HandleFunc("/volumes/prune", PruneVolumesHandler).Methods(http.MethodPost)
	api.HandleFunc("/volumes/{name}/browse", BrowseVolumeHandler).Methods(http.MethodGet)
	
	// File save endpoints for practice pages
	api.HandleFunc("/api/save-compose", SaveComposeFileHandler).Methods(http.MethodPost)
	api.HandleFunc("/api/save-nginx", SaveNginxFileHandler).Methods(http.MethodPost)
	
	return r
}
