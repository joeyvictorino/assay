package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/model"
)

// ---- open-redirect --------------------------------------------------------

type openRedirect struct{}

func (openRedirect) Class() model.Class { return model.ClassOpenRedirect }

// Check asks for a redirect to a unique host under the reserved .invalid TLD
// and inspects the Location header. The Location is never fetched: the
// request uses ExchangeNoFollow and .invalid cannot resolve.
func (openRedirect) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	param := paramOf(f, "to")
	host := newMarker() + ".invalid"
	payload := "http://" + host + "/"
	target := withQuery(targetURL(env, f, nil), param, payload)
	var evs []model.Evidence
	safe := 0
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.ExchangeNoFollow(ctx, "GET", target, nil, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		if r.Status >= 300 && r.Status < 400 {
			if loc, err := url.Parse(r.Headers["Location"]); err == nil && strings.EqualFold(loc.Hostname(), host) {
				evs = append(evs, evidence("http-exchange", r, host))
				continue
			}
		}
		safe++
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, "Location points at caller-supplied external host", evs...), nil
	case safe == MinObservations:
		return result(model.StateRefuted, "no redirect to external host"), nil
	}
	return result(model.StateTheorized, "inconsistent redirect behavior", evs...), nil
}

// ---- xss-reflected --------------------------------------------------------

type xssReflected struct{}

func (xssReflected) Class() model.Class { return model.ClassXSSReflected }

// Check reflects "<b>assay-<hex></b>": a benign formatting tag, never a
// script. Validated only when the exact unescaped markup comes back in an
// HTML response.
func (xssReflected) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	param := f.Location.Param
	if param == "" {
		return result(model.StateTheorized, "finding names no parameter"), nil
	}
	marker := newMarker()
	payload := "<b>" + marker + "</b>"
	target := withQuery(targetURL(env, f, nil), param, payload)
	var evs []model.Evidence
	escaped := 0
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.Exchange(ctx, "GET", target, nil, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		body := string(r.Body)
		switch {
		case strings.Contains(body, payload) && isHTML(r.Headers):
			evs = append(evs, evidence("marker-reflected", r, marker))
		case strings.Contains(body, marker):
			escaped++ // present but encoded or not HTML
		default:
			escaped++
		}
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, "benign tag reflected unescaped in HTML", evs...), nil
	case escaped == MinObservations:
		return result(model.StateRefuted, "marker absent, encoded, or not in an HTML response"), nil
	}
	return result(model.StateTheorized, "inconsistent reflection", evs...), nil
}

// ---- sqli (boolean differential) -----------------------------------------

type sqli struct{}

func (sqli) Class() model.Class { return model.ClassSQLi }

// sqliPair is a tautology and a contradiction that differ only in truth
// value. No stacked statements, no DDL, no time-based functions.
type sqliPair struct {
	name, truthy, falsy string
}

var sqliPairs = []sqliPair{
	{"quoted-equality", "assay' OR 'a'='a", "assay' AND 'a'='b"},
	{"quoted-like", "assay%' OR 'a%'='a", "assay%' AND 'a%'='b"},
	{"numeric", "1 OR 1=1", "1 AND 1=2"},
}

func (sqli) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	param := f.Location.Param
	if param == "" {
		return result(model.StateTheorized, "finding names no parameter"), nil
	}
	base := targetURL(env, f, nil)
	equal := 0
	for _, p := range sqliPairs {
		var evs []model.Evidence
		confirmed := 0
		for i := 0; i < MinObservations; i++ {
			rt, err := env.HTTP.Exchange(ctx, "GET", withQuery(base, param, p.truthy), nil, nil)
			if err != nil {
				return model.ValidationResult{}, err
			}
			rf, err := env.HTTP.Exchange(ctx, "GET", withQuery(base, param, p.falsy), nil, nil)
			if err != nil {
				return model.ValidationResult{}, err
			}
			if rt.Status != 200 || rf.Status != 200 {
				break // errors are inconclusive, not evidence
			}
			ct, cf := resultCount(rt), resultCount(rf)
			if ct < 0 || cf < 0 {
				break
			}
			if ct > cf {
				confirmed++
				evs = append(evs, evidence("http-exchange", rt, fmt.Sprintf("%s:true=%d", p.name, ct)))
				evs = append(evs, evidence("http-exchange", rf, fmt.Sprintf("%s:false=%d", p.name, cf)))
			} else {
				if ct == cf {
					equal++
				}
				break
			}
		}
		if confirmed == MinObservations {
			return result(model.StateValidated, "boolean differential via "+p.name+" (tautology returns more rows than contradiction)", evs...), nil
		}
	}
	if equal == len(sqliPairs) {
		return result(model.StateRefuted, "tautology and contradiction return identical result counts for every form"), nil
	}
	return result(model.StateTheorized, "no reproducible boolean differential; errors or unknown response shape"), nil
}

// resultCount estimates how many records a response holds: the length of a
// JSON array (top level or first array-valued field), else the number of
// repeated list rows in HTML, else -1 when unknown.
func resultCount(r *httpx.Response) int {
	trim := strings.TrimSpace(string(r.Body))
	if strings.HasPrefix(trim, "[") {
		var arr []json.RawMessage
		if json.Unmarshal(r.Body, &arr) == nil {
			return len(arr)
		}
		return -1
	}
	if strings.HasPrefix(trim, "{") {
		var obj map[string]json.RawMessage
		if json.Unmarshal(r.Body, &obj) == nil {
			keys := make([]string, 0, len(obj))
			for k := range obj {
				keys = append(keys, k)
			}
			sortStrings(keys)
			for _, k := range keys {
				var arr []json.RawMessage
				if json.Unmarshal(obj[k], &arr) == nil {
					return len(arr)
				}
			}
			return -1
		}
		return -1
	}
	if isHTML(r.Headers) {
		lower := strings.ToLower(trim)
		if n := strings.Count(lower, "<tr"); n > 0 {
			return n
		}
		if n := strings.Count(lower, "<li"); n > 0 {
			return n
		}
		return 0
	}
	return -1
}

// ---- path-traversal -------------------------------------------------------

type pathTraversal struct{}

func (pathTraversal) Class() model.Class { return model.ClassPathTraversal }

// Check reads a known benign file (the lab's own README by default) through
// "../" sequences and looks for its marker. Nothing is written.
func (pathTraversal) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	param := paramOf(f, "name")
	file := env.Markers["traversal-file"]
	if file == "" {
		file = "README.md"
	}
	marker := env.marker("traversal-marker")
	if marker == "" {
		return result(model.StateTheorized, "no traversal marker configured"), nil
	}
	base := targetURL(env, f, nil)
	notFound := 0
	for depth := 1; depth <= 3; depth++ {
		payload := strings.Repeat("../", depth) + file
		target := withQuery(base, param, payload)
		var evs []model.Evidence
		for i := 0; i < MinObservations; i++ {
			r, err := env.HTTP.Exchange(ctx, "GET", target, nil, nil)
			if err != nil {
				return model.ValidationResult{}, err
			}
			if r.Status == 200 && strings.Contains(string(r.Body), marker) {
				evs = append(evs, evidence("marker-reflected", r, marker))
			} else {
				notFound++
				break
			}
		}
		if len(evs) == MinObservations {
			return result(model.StateValidated, fmt.Sprintf("read %s via %d-level traversal", file, depth), evs...), nil
		}
	}
	if notFound == 3 {
		return result(model.StateRefuted, "traversal sequences did not expose the marker file"), nil
	}
	return result(model.StateTheorized, "intermittent traversal result"), nil
}
