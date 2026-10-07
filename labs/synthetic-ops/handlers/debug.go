package handlers

import (
	"fmt"
	"net/http"
)

// debugConfig dumps the running configuration. Every value is fake; the
// marker strings exist so a checker can prove disclosure without any real
// secret being present.
func (s *Server) debugConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{ // VULN: info-disclosure configuration and planted credential marker returned without authentication
		"service":       "synthetic-ops",
		"database_url":  "sqlite://synthetic-ops.db",
		"admin_api_key": SecretMarker,
		"smtp_password": SecretMarker + "-smtp",
		"debug":         true,
		"public_dir":    s.opts.PublicDir,
	})
}

// backup serves a fake database dump from the web root.
func (s *Server) backup(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/sql")
	fmt.Fprintf(w, "-- synthetic-ops nightly backup\n-- marker: %s\nCREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT, password TEXT);\nINSERT INTO users VALUES (1, 'admin', 'admin123');\n", BackupMarker) // VULN: sensitive-file database dump reachable at a guessable path
}

// boom always panics; the middleware turns that into a verbose error page.
func (s *Server) boom(_ http.ResponseWriter, _ *http.Request) {
	var m map[string]int
	m["boom"] = 1 // nil map write, recovered in middleware
}
