package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
)

// SecretMarker is the planted, non-secret marker returned by /debug/config.
const SecretMarker = "SYNTH-SECRET-MARKER-0001"

// BackupMarker is the planted marker inside the fake /backup.sql dump.
const BackupMarker = "SYNTH-BACKUP-MARKER-0001"

// Options configure the service.
type Options struct {
	// UnsafeSSRF lets /api/fetch reach non-loopback hosts. Default false:
	// the handler refuses anything that does not resolve to loopback so a
	// CI runner can never be made to call out.
	UnsafeSSRF bool
	// PublicDir is the directory /files serves from (default ./public).
	PublicDir string
}

// Server holds the shared state behind the handlers.
type Server struct {
	db   *sql.DB
	opts Options
}

// New returns the vulnerable service as an http.Handler.
func New(db *sql.DB, opts Options) http.Handler {
	if opts.PublicDir == "" {
		opts.PublicDir = "./public"
	}
	s := &Server{db: db, opts: opts}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("GET /api/users/{id}", s.getUser)
	mux.HandleFunc("GET /api/search", s.apiSearch)
	mux.HandleFunc("POST /api/profile", s.updateProfile)
	mux.HandleFunc("GET /profile/{id}", s.showProfile)
	mux.HandleFunc("GET /search", s.htmlSearch)
	mux.HandleFunc("GET /redirect", s.redirect)
	mux.HandleFunc("GET /files", s.files)
	mux.HandleFunc("GET /api/admin/stats", s.adminStats)
	mux.HandleFunc("GET /api/fetch", s.fetch)
	mux.HandleFunc("GET /debug/config", s.debugConfig)
	mux.HandleFunc("GET /backup.sql", s.backup)
	mux.HandleFunc("GET /api/boom", s.boom)
	return s.middleware(mux)
}

// middleware adds permissive CORS and recovers panics with a verbose page.
// VULN: security-headers no Content-Security-Policy, X-Content-Type-Options, X-Frame-Options or Referrer-Policy are ever set
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // VULN: cors-misconfig wildcard origin combined with credentials
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Server", "synthetic-ops/0.1 (go net/http)")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		defer func() {
			if rec := recover(); rec != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "<h1>Internal Server Error</h1><pre>panic: %v\n\n%s</pre>", rec, debug.Stack()) // VULN: verbose-error stack trace returned to the client
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html><html><head><title>synthetic-ops</title></head><body>
<h1>synthetic-ops</h1>
<p>Deliberately vulnerable lab service for authorized CI testing only.</p>
<ul>
<li><a href="/search?term=widget">Search</a></li>
<li><a href="/profile/1">Profile 1</a></li>
<li><a href="/files?name=hello.txt">Files</a></li>
</ul>
</body></html>`)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func newToken() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// currentUser resolves the bearer token to a user id, or 0.
func (s *Server) currentUser(r *http.Request) int {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return 0
	}
	var id int
	if err := s.db.QueryRow(`SELECT user_id FROM sessions WHERE token = ?`, strings.TrimPrefix(auth, "Bearer ")).Scan(&id); err != nil {
		return 0
	}
	return id
}
