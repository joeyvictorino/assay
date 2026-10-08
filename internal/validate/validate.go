// Package validate turns theorized findings into validated or refuted ones
// with deterministic, non-destructive checkers.
//
// Rules every checker obeys:
//   - every request goes through internal/httpx (gated, audited);
//   - payloads only read: no writes beyond a marker in a field the checker
//     owns, no deletion, no lockout, no timing attacks, no stacked queries;
//   - a finding becomes validated only after two consistent observations;
//   - anything inconclusive stays theorized; only a clear negative refutes.
package validate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/model"
)

// Env is what a checker may use.
type Env struct {
	HTTP *httpx.Client
	Lab  labs.Lab
	// Marker is the planted string expected in disclosed content when the
	// ground truth does not name one (info-disclosure, sensitive-file,
	// path-traversal).
	Marker string
	// Markers holds optional per-purpose overrides:
	//   "traversal-file"   relative file the traversal checker reads (default README.md)
	//   "traversal-marker" string expected inside that file (default Marker)
	// LEAD: fold into a typed struct if more purposes appear.
	Markers map[string]string
	// Now is the clock (tests); nil means time.Now.
	Now func() time.Time
	// Pace is the gap between paced requests in observational checks (rate
	// limiting). Zero means 100ms; negative disables pacing (tests).
	Pace time.Duration
}

func (e Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now().UTC()
}

func (e Env) marker(key string) string {
	if v, ok := e.Markers[key]; ok && v != "" {
		return v
	}
	return e.Marker
}

// Checker validates one class of finding.
type Checker interface {
	Class() model.Class
	Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error)
}

// MinObservations is how many consistent observations "validated" needs.
const MinObservations = 2

// Checkers returns the default checker set in model.AllClasses order.
func Checkers() []Checker {
	all := []Checker{
		securityHeaders{},
		verboseError{},
		corsMisconfig{},
		markerChecker{class: model.ClassInfoDisclosure},
		markerChecker{class: model.ClassSensitiveFile},
		authMissing{},
		openRedirect{},
		xssReflected{},
		sqli{},
		pathTraversal{},
		idor{},
		defaultCreds{},
		authBypass{},
		jwtWeak{},
		massAssignment{},
		xssStored{},
		rateLimitMissing{},
		csrf{},
		ssrf{},
	}
	byClass := map[model.Class]Checker{}
	for _, c := range all {
		byClass[c.Class()] = c
	}
	out := make([]Checker, 0, len(all))
	for _, cl := range model.AllClasses {
		if c, ok := byClass[cl]; ok {
			out = append(out, c)
		}
	}
	return out
}

// Lookup returns the checker for a class, or nil.
func Lookup(class model.Class) Checker {
	for _, c := range Checkers() {
		if c.Class() == class {
			return c
		}
	}
	return nil
}

// Validate runs the matching checker for every finding and returns copies
// with State and Validation set. Findings keep their input order. A finding
// with no checker, a checker error, or fewer than MinObservations pieces of
// evidence is theorized, never validated.
func Validate(ctx context.Context, findings []model.Finding, env Env) []model.Finding {
	out := make([]model.Finding, len(findings))
	copy(out, findings)
	for i := range out {
		f := &out[i]
		if f.State == model.StateDeclined {
			continue
		}
		res := runOne(ctx, *f, env)
		f.State = res.State
		f.Validation = &res
		f.LastSeen = res.CheckedAt
	}
	return out
}

func runOne(ctx context.Context, f model.Finding, env Env) model.ValidationResult {
	now := env.now()
	c := Lookup(f.Class)
	if c == nil {
		return model.ValidationResult{State: model.StateTheorized, Checker: "none", Note: "no checker for class " + string(f.Class), CheckedAt: now}
	}
	if env.HTTP == nil || env.Lab == nil {
		return model.ValidationResult{State: model.StateTheorized, Checker: string(c.Class()), Note: "validation environment incomplete", CheckedAt: now}
	}
	res, err := c.Check(ctx, f, env)
	res.Checker = string(c.Class())
	if err != nil {
		res.State = model.StateTheorized
		res.Note = joinNote("checker error: "+shortErr(err), res.Note)
		res.CheckedAt = now
		return res
	}
	return enforce(res, env)
}

// enforce applies the evidentiary rules to a raw checker result: validated
// needs MinObservations pieces of evidence; unknown states become theorized.
func enforce(res model.ValidationResult, env Env) model.ValidationResult {
	res.CheckedAt = env.now()
	switch res.State {
	case model.StateValidated:
		if len(res.Evidence) < MinObservations {
			res.State = model.StateTheorized
			res.Note = joinNote(fmt.Sprintf("only %d observation(s); %d required", len(res.Evidence), MinObservations), res.Note)
		}
	case model.StateRefuted, model.StateTheorized:
	default:
		res.State = model.StateTheorized
		res.Note = joinNote("checker returned no state", res.Note)
	}
	return res
}

func joinNote(a, b string) string {
	if b == "" {
		return a
	}
	return a + "; " + b
}

// shortErr bounds error text and drops anything that looks like a URL
// query so no payload lands in a note.
func shortErr(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '?'); i > 0 {
		s = s[:i]
	}
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

var tplVar = regexp.MustCompile(`\{[a-zA-Z_]+\}`)

// targetURL builds the request URL for a finding, filling {id}-style
// template variables from vars (default "1").
func targetURL(env Env, f model.Finding, vars map[string]string) string {
	p := f.Location.PathTemplate
	if p == "" {
		p = "/"
	}
	p = tplVar.ReplaceAllStringFunc(p, func(m string) string {
		k := strings.Trim(m, "{}")
		if v, ok := vars[k]; ok {
			return url.PathEscape(v)
		}
		return "1"
	})
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(env.Lab.BaseURL(), "/") + p
}

// withQuery appends one query parameter to a URL.
func withQuery(target, key, value string) string {
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

// newMarker returns a unique, obviously-synthetic marker "assay-<hex>".
func newMarker() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "assay-" + hex.EncodeToString(b[:])
}

// evidence builds a digest-only Evidence from an exchange.
func evidence(kind string, r *httpx.Response, marker string) model.Evidence {
	return model.Evidence{
		Kind:           kind,
		ToolCallID:     r.ToolCallID,
		AuditSeq:       r.AuditSeq,
		RequestDigest:  r.RequestDigest,
		ResponseDigest: r.ResponseDigest,
		StatusCode:     r.Status,
		Marker:         marker,
	}
}

func result(state model.FindingState, note string, ev ...model.Evidence) model.ValidationResult {
	return model.ValidationResult{State: state, Note: note, Evidence: ev}
}

func paramOf(f model.Finding, fallback string) string {
	if f.Location.Param != "" {
		return f.Location.Param
	}
	return fallback
}

func methodOf(f model.Finding) string {
	if f.Location.Method == "" {
		return "GET"
	}
	return strings.ToUpper(f.Location.Method)
}

func isHTML(headers map[string]string) bool {
	ct := strings.ToLower(headers["Content-Type"])
	return ct == "" || strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

// groundTruthFor finds the ground-truth entry for a finding by class and
// path template, falling back to method and path.
func groundTruthFor(env Env, f model.Finding) *labs.GroundTruthEntry {
	entries, _ := env.Lab.GroundTruth()
	if entries == nil {
		return nil
	}
	if e := labs.FindByClass(entries, f.Class, f.Location.PathTemplate); e != nil {
		return e
	}
	return labs.Find(entries, methodOf(f), f.Location.PathTemplate)
}
