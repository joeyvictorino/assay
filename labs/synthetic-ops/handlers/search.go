package handlers

import (
	"net/http"
)

type product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int    `json:"price_cents"`
}

// apiSearch filters products by name.
func (s *Server) apiSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	query := "SELECT id, name, description, price_cents FROM products WHERE name LIKE '%" + q + "%'" // VULN: sqli user input concatenated into SQL
	rows, err := s.db.Query(query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "query": query})
		return
	}
	defer rows.Close()
	out := []product{}
	for rows.Next() {
		var p product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, out)
}
