package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed openapi/*
var openAPIFS embed.FS

func RegisterOpenAPI(mux *http.ServeMux) {
	sub, _ := fs.Sub(openAPIFS, "openapi")
	mux.Handle("/openapi/", http.StripPrefix("/openapi/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/openapi/openapi.yaml", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/swagger/", serveSwaggerUI)
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
}

func serveSwaggerUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := openAPIFS.ReadFile("openapi/swagger.html")
	if err != nil {
		http.Error(w, "swagger ui not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
