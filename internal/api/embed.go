package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed ui/*
var uiFiles embed.FS

// GetUIFileServer returns an HTTP file server for the embedded UI files
func GetUIFileServer() http.Handler {
	// Strip the "ui" prefix so files are served from root
	subFS, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(subFS))
}
