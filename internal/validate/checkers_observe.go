package validate

// Observational checkers: they read pages and headers, pace a bounded number
// of identical requests, or point the target at its own loopback address.
// None of them writes anything.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/model"
)

// ---- rate-limit-missing ---------------------------------------------------

type rateLimitMissing struct{}

func (rateLimitMissing) Class() model.Class { return model.ClassRateLimit }

const (
	// rateProbeTotal is the most identical requests the checker ever sends
	// for one finding: two observations of rateProbeBatch each. It never
	// exceeds 20 and the requests are paced, so this is not a flood.
	rateProbeTotal = 16
	rateProbeBatch = rateProbeTotal / MinObservations
	// defaultPace is the gap between paced requests when Env.Pace is zero.
	defaultPace = 100 * time.Millisecond
)

// pace waits Env.Pace (default defaultPace; negative disables) or until the
// context ends.
func pace(ctx context.Context, env Env) error {
	d := env.Pace
	if d == 0 {
		d = defaultPace
	}
	if d < 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isRateLimited(r *httpx.Response) bool {
	return r.Status == 429 || r.Headers["Retry-After"] != ""
}

// Check sends two paced batches of identical requests carrying a synthetic
// value for the finding's parameter (a bogus credential, never a real
// account) and reports whether any response was rate-limited. Validated
// means "observed not limited", which is all a black-box check can say.
func (rateLimitMissing) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	method := methodOf(f)
	target := targetURL(env, f, nil)
	marker := newMarker()
	var hdrs map[string]string
	var body []byte
	switch {
	case isWrite(method):
		fields := map[string]string{}
		if p := f.Location.Param; p != "" {
			fields[p] = marker
		}
		body, _ = json.Marshal(fields)
		hdrs = jsonHeaders
	case f.Location.Param != "":
		target = withQuery(target, f.Location.Param, marker)
	}
	var evs []model.Evidence
	limited := 0
	for b := 0; b < MinObservations; b++ {
		var last *httpx.Response
		hit := false
		for i := 0; i < rateProbeBatch; i++ {
			if err := pace(ctx, env); err != nil {
				return model.ValidationResult{}, err
			}
			r, err := env.HTTP.Exchange(ctx, method, target, hdrs, body)
			if err != nil {
				return model.ValidationResult{}, err
			}
			last = r
			if isRateLimited(r) {
				hit = true
				break
			}
		}
		if hit {
			limited++
			continue
		}
		evs = append(evs, evidence("http-exchange", last, fmt.Sprintf("%d identical requests, none rate-limited", rateProbeBatch)))
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, fmt.Sprintf("%d identical paced %s requests in two batches, none rate-limited (observational)", rateProbeTotal, method), evs...), nil
	case limited == MinObservations:
		return result(model.StateRefuted, "rate limiting observed (429 or Retry-After) in both batches"), nil
	}
	return result(model.StateTheorized, "rate limiting observed in only one batch", evs...), nil
}

// ---- csrf -----------------------------------------------------------------

type csrf struct{}

func (csrf) Class() model.Class { return model.ClassCSRF }

var (
	formRe       = regexp.MustCompile(`(?is)<form\b[^>]*>.*?</form>`)
	methodPostRe = regexp.MustCompile(`(?i)\bmethod\s*=\s*["']?\s*post\b`)
	actionRe     = regexp.MustCompile(`(?i)\baction\s*=\s*["']?([^"'\s>]+)`)
	inputRe      = regexp.MustCompile(`(?is)<input\b[^>]*>`)
	hiddenRe     = regexp.MustCompile(`(?i)\btype\s*=\s*["']?hidden`)
	tokenNameRe  = regexp.MustCompile(`(?i)\bname\s*=\s*["']?[^"'\s>]*(csrf|xsrf|token|nonce|authenticity|verification)`)
	sameSiteRe   = regexp.MustCompile(`(?i)samesite\s*=\s*(strict|lax)`)
)

type formInfo struct {
	action string
	token  bool
}

// postForms lists the POST forms on a page, preferring those whose action
// is the given path. A form without an action posts to the page itself.
func postForms(page, path string) []formInfo {
	var all, matching []formInfo
	for _, fm := range formRe.FindAllString(page, -1) {
		if !methodPostRe.MatchString(fm) {
			continue
		}
		action := path
		if m := actionRe.FindStringSubmatch(fm); m != nil {
			action = m[1]
		}
		token := false
		for _, in := range inputRe.FindAllString(fm, -1) {
			if hiddenRe.MatchString(in) && tokenNameRe.MatchString(in) {
				token = true
				break
			}
		}
		info := formInfo{action: action, token: token}
		all = append(all, info)
		if strings.EqualFold(strings.TrimRight(action, "/"), strings.TrimRight(path, "/")) {
			matching = append(matching, info)
		}
	}
	if len(matching) > 0 {
		return matching
	}
	return all
}

// Check GETs the finding's path and inspects the forms it serves. Nothing
// is ever submitted: a POST form without a hidden anti-CSRF field, on a
// response without a SameSite session cookie, is the observation.
func (csrf) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	path := f.Location.PathTemplate
	if path == "" {
		path = "/"
	}
	target := targetURL(env, f, nil)
	var evs []model.Evidence
	protected, noForm := 0, 0
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.Exchange(ctx, "GET", target, nil, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		if r.Status != 200 || !isHTML(r.Headers) {
			noForm++
			continue
		}
		forms := postForms(string(r.Body), path)
		if len(forms) == 0 {
			noForm++
			continue
		}
		unprotected := ""
		for _, fm := range forms {
			if !fm.token {
				unprotected = fm.action
				break
			}
		}
		if unprotected == "" || sameSiteRe.MatchString(r.Headers["Set-Cookie"]) {
			protected++
			continue
		}
		evs = append(evs, evidence("http-exchange", r, "post form action="+unprotected+" without anti-CSRF token"))
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, "state-changing form posts without an anti-CSRF token and without a SameSite cookie (observed only, nothing submitted)", evs...), nil
	case protected == MinObservations:
		return result(model.StateRefuted, "post forms carry a token or the session cookie is SameSite"), nil
	case noForm == MinObservations:
		return result(model.StateTheorized, "no state-changing form observed at the path"), nil
	}
	return result(model.StateTheorized, "inconsistent form observations", evs...), nil
}

// ---- ssrf -----------------------------------------------------------------

type ssrf struct{}

func (ssrf) Class() model.Class { return model.ClassSSRF }

// fingerprintRe finds runs of plain text long enough to recognize a page
// after it has been JSON-encoded inside another response.
var fingerprintRe = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9 -]{11,}`)

// fingerprint returns the longest plain-text run in body, or "".
func fingerprint(body []byte) string {
	best := ""
	for _, m := range fingerprintRe.FindAllString(string(body), -1) {
		m = strings.TrimSpace(m)
		if len(m) > len(best) {
			best = m
		}
	}
	if len(best) < 12 {
		return ""
	}
	return best
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Check asks the target to fetch its own root URL and looks for that page's
// text in the answer. The only URL ever supplied is the lab's own loopback
// address; a lab that is not on loopback is never probed.
func (ssrf) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	param := paramOf(f, "url")
	base, err := url.Parse(env.Lab.BaseURL())
	if err != nil || !isLoopbackHost(base.Hostname()) {
		return result(model.StateTheorized, "lab is not on loopback; SSRF probe withheld (the checker only points the target at itself)"), nil
	}
	self := strings.TrimRight(env.Lab.BaseURL(), "/") + "/"
	direct, err := env.HTTP.Exchange(ctx, "GET", self, nil, nil)
	if err != nil {
		return model.ValidationResult{}, err
	}
	fp := fingerprint(direct.Body)
	if fp == "" {
		return result(model.StateTheorized, "lab root page has no plain text to recognize in a fetched copy"), nil
	}
	target := withQuery(targetURL(env, f, nil), param, self)
	var evs []model.Evidence
	blocked := 0
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.Exchange(ctx, "GET", target, nil, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		if r.Status == 200 && strings.Contains(string(r.Body), fp) {
			evs = append(evs, evidence("http-exchange", r, "loopback "+self))
		} else {
			blocked++
		}
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, "server fetched its own loopback URL and returned the page", evs...), nil
	case blocked == MinObservations:
		return result(model.StateRefuted, "server did not return loopback content for a caller-supplied URL"), nil
	}
	return result(model.StateTheorized, "inconsistent fetch behavior", evs...), nil
}
