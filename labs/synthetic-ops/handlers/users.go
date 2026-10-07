package handlers

import (
	"net/http"
	"strconv"
)

type userRecord struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Bio      string `json:"bio"`
}

// getUser returns any user's record to any authenticated caller.
func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	if s.currentUser(r) == 0 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "login required"})
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var u userRecord
	err = s.db.QueryRow(`SELECT id, username, email, role, bio FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.Bio) // VULN: idor no check that id belongs to the caller
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "no such user"})
		return
	}
	writeJSON(w, http.StatusOK, u)
}
