package handlers

import "net/http"

// redirect sends the browser wherever ?to= says.
func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	to := r.URL.Query().Get("to")
	if to == "" {
		to = "/"
	}
	http.Redirect(w, r, to, http.StatusFound) // VULN: open-redirect destination taken from the request without an allowlist
}
