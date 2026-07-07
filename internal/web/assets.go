package web

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed ui
var consoleAssets embed.FS

func (s *server) serveSPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		s.writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}
	assets, err := fs.Sub(consoleAssets, "ui")
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "internal", "Console assets unavailable")
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}
	content, err := fs.ReadFile(assets, name)
	if err != nil {
		name = "index.html"
		content, err = fs.ReadFile(assets, name)
	}
	if err != nil {
		s.writeError(w, r, http.StatusNotFound, "not_found", "Not found")
		return
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if name == "index.html" {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(content)
	}
}
