package handlers

import (
	"encoding/json"
	"net/http"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// login issues a bearer token. The query is parameterized; the planted
// weaknesses here are the seeded default credential (see db.go) and the
// absence of any attempt limiting.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	var id int
	var role string
	err := s.db.QueryRow(`SELECT id, role FROM users WHERE username = ? AND password = ?`, req.Username, req.Password).Scan(&id, &role) // VULN: rate-limit-missing unlimited password attempts
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid credentials"})
		return
	}
	token := newToken()
	if _, err := s.db.Exec(`INSERT INTO sessions (token, user_id) VALUES (?, ?)`, token, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user_id": id, "role": role})
}
