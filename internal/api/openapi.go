package api

import (
	"embed"
	"encoding/json"
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

// RootHandler responds on GET / with service links.
func RootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"service":    "lowcode-database",
		"api":        "/v1/",
		"openapi":    "/openapi/openapi.yaml",
		"swagger":    "/swagger/",
		"playground": "https://github.com/solat/lowcode-database-playground",
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
