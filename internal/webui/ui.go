package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/store"
)

//go:embed static/*
var static embed.FS

func Mount(mux *http.ServeMux, _ *store.Store, _ func() time.Time) {
	root, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(root))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, root, "index.html")
	})
	mux.Handle("GET /app.js", files)
	mux.Handle("GET /app.css", files)
	mux.Handle("GET /fonts.css", files)
	mux.Handle("GET /kervia.css", files)
	mux.Handle("GET /fonts/", files)
	mux.Handle("GET /brand/", files)
}
