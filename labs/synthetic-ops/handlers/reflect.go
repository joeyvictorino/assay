package handlers

import (
	"fmt"
	"net/http"
)

// htmlSearch renders a search page that echoes the term.
func (s *Server) htmlSearch(w http.ResponseWriter, r *http.Request) {
	term := r.URL.Query().Get("term")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<!doctype html><html><head><title>Search</title></head><body><h1>Search</h1><p>Results for: %s</p><ul><li>Widget</li><li>Gadget</li></ul></body></html>", term) // VULN: xss-reflected query parameter echoed unescaped
}
