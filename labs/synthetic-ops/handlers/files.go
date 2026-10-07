package handlers

import (
	"net/http"
	"os"
	"path/filepath"
)

// files serves a file from the public directory by name.
func (s *Server) files(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	p := filepath.Join(s.opts.PublicDir, name) // VULN: path-traversal joined path is never checked to stay inside PublicDir
	data, err := os.ReadFile(p)
	if err != nil {
		http.Error(w, "not found: "+p, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}
