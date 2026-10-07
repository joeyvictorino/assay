package handlers

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// fetch retrieves a URL on behalf of the caller and returns its body.
//
// Safety rail for CI: unless Options.UnsafeSSRF is set, the target host
// must resolve to a loopback address. The weakness is still real (the
// service can be pointed at itself or at anything else on the loopback
// interface, such as /debug/config), but a CI runner can never be used to
// reach the outside world.
func (s *Server) fetch(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("url")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "url must be absolute http(s)"})
		return
	}
	if !s.opts.UnsafeSSRF && !resolvesToLoopback(u.Hostname()) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "refused: non-loopback target; start with -unsafe-ssrf to allow"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil) // VULN: ssrf server fetches a caller-supplied URL and returns the body
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	writeJSON(w, http.StatusOK, map[string]any{"status": resp.StatusCode, "body": string(body)})
}

func resolvesToLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !ip.IsLoopback() {
			return false
		}
	}
	return true
}
