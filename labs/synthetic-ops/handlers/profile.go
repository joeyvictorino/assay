package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type profileRequest struct {
	Bio string `json:"bio"`
}

// updateProfile stores the caller's bio verbatim.
func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	uid := s.currentUser(r)
	if uid == 0 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "login required"})
		return
	}
	var req profileRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if len(req.Bio) > 2000 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bio too long"})
		return
	}
	if _, err := s.db.Exec(`UPDATE users SET bio = ? WHERE id = ?`, req.Bio, uid); err != nil { // VULN: xss-stored bio persisted without sanitization and rendered raw by showProfile
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "profile": "/profile/" + strconv.Itoa(uid)})
}

// showProfile renders a user's profile page.
func (s *Server) showProfile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var username, bio string
	if err := s.db.QueryRow(`SELECT username, bio FROM users WHERE id = ?`, id).Scan(&username, &bio); err != nil {
		http.Error(w, "no such user", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<!doctype html><html><head><title>%s</title></head><body><h1>%s</h1><div class=\"bio\">%s</div></body></html>", username, username, bio) // VULN: xss-stored bio written into HTML unescaped
}
