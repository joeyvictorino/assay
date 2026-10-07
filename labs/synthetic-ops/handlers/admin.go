package handlers

import "net/http"

// adminStats reports table counts. It is meant for administrators.
func (s *Server) adminStats(w http.ResponseWriter, r *http.Request) {
	// VULN: auth-missing no session or role check before returning admin data
	var users, sessions, products int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessions)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&products)
	writeJSON(w, http.StatusOK, map[string]any{
		"users":    users,
		"sessions": sessions,
		"products": products,
		"admin":    true,
	})
}
