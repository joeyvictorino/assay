// Package scope loads a signed scope document and turns it into the
// authorization gate every outbound request must pass.
//
// The gate fails closed: a missing, unparseable, unsigned, expired or
// target-less document never produces a Gate. Signature verification is
// injected so this package does not depend on the trust store.
package scope

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"gopkg.in/yaml.v3"
)

// Error codes. Every failure to build or consult a Gate carries one.
const (
	CodeReadFailed       = "SCOPE_READ_FAILED"
	CodeParseFailed      = "SCOPE_PARSE_FAILED"
	CodeVerifierMissing  = "SCOPE_UNSIGNED_VERIFIER_MISSING"
	CodeSignatureInvalid = "SCOPE_SIGNATURE_INVALID"
	CodeMissingField     = "SCOPE_MISSING_FIELD"
	CodeInvalidField     = "SCOPE_INVALID_FIELD"
	CodeNoTargets        = "SCOPE_NO_TARGETS"
	CodeExpired          = "SCOPE_EXPIRED"
	CodeNotYetValid      = "SCOPE_NOT_YET_VALID"
	CodeRateLimited      = "RATE_LIMITED"

	// Decision reason codes.
	ReasonInScope       = "IN_SCOPE"
	ReasonOutOfScope    = "OUT_OF_SCOPE"
	ReasonMethodAllowed = "METHOD_ALLOWED"
	ReasonMethodDenied  = "METHOD_NOT_ALLOWED"
)

// Error is a coded scope error.
type Error struct {
	Code string
	Msg  string
}

func (e *Error) Error() string {
	if e.Msg == "" {
		return e.Code
	}
	return e.Code + ": " + e.Msg
}

// Is lets errors.Is match on code (so ErrRateLimited matches any RATE_LIMITED).
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// ErrRateLimited is returned by Take when the token bucket is empty.
var ErrRateLimited = &Error{Code: CodeRateLimited, Msg: "scope rate limit exceeded"}

func newErr(code, format string, a ...any) error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, a...)}
}

// Target is one authorized host.
type Target struct {
	Host         string   `yaml:"host" json:"host"`
	Ports        []int    `yaml:"ports" json:"ports"`
	Schemes      []string `yaml:"schemes" json:"schemes"`
	PathPrefixes []string `yaml:"path_prefixes,omitempty" json:"path_prefixes,omitempty"`
}

// Document is the parsed scope document.
type Document struct {
	Version                 string   `yaml:"version" json:"version"`
	AuthorizedBy            string   `yaml:"authorized_by" json:"authorized_by"`
	IssuedAt                string   `yaml:"issued_at" json:"issued_at"`
	ExpiresAt               string   `yaml:"expires_at" json:"expires_at"`
	RulesOfEngagementSHA256 string   `yaml:"rules_of_engagement_sha256" json:"rules_of_engagement_sha256"`
	Targets                 []Target `yaml:"targets" json:"targets"`
	Methods                 []string `yaml:"methods" json:"methods"`
	RateLimitPerMinute      int      `yaml:"rate_limit_per_minute" json:"rate_limit_per_minute"`
	MaxBodyBytes            int64    `yaml:"max_body_bytes" json:"max_body_bytes"`
	Signature               any      `yaml:"signature,omitempty" json:"signature,omitempty"`

	issued  time.Time
	expires time.Time
}

// Verifier checks the document's signature against a trust store and returns
// the signing key id. It receives the raw document map, signature included.
// Supplied by the trust store (S1); nil means the document cannot be trusted.
type Verifier func(doc map[string]any) (keyID, reason string, err error)

// Gate is the compiled scope document. It implements model.Gate.
type Gate struct {
	doc         Document
	fingerprint string
	keyID       string

	mu     sync.Mutex
	tokens float64
	last   time.Time
	rate   float64 // tokens per second
	burst  float64
	now    func() time.Time
}

// Load reads and verifies a scope document. It fails closed.
func Load(path string, now time.Time, verify Verifier) (*Gate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, newErr(CodeReadFailed, "%v", err)
	}
	return Parse(data, now, verify)
}

// Parse builds a Gate from document bytes (YAML or JSON). It fails closed.
func Parse(data []byte, now time.Time, verify Verifier) (*Gate, error) {
	if verify == nil {
		return nil, newErr(CodeVerifierMissing, "no signature verifier supplied; unsigned scope is never accepted")
	}
	raw := map[string]any{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, newErr(CodeParseFailed, "%v", err)
	}
	if len(raw) == 0 {
		return nil, newErr(CodeParseFailed, "empty document")
	}
	if _, ok := raw["signature"]; !ok || raw["signature"] == nil {
		return nil, newErr(CodeSignatureInvalid, "document has no signature")
	}
	keyID, reason, err := verify(raw)
	if err != nil {
		return nil, newErr(CodeSignatureInvalid, "%s: %v", reason, err)
	}
	if keyID == "" {
		return nil, newErr(CodeSignatureInvalid, "verifier returned empty key id")
	}

	var doc Document
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, newErr(CodeParseFailed, "%v", err)
	}
	if err := doc.validate(now); err != nil {
		return nil, err
	}
	fp, err := fingerprint(raw)
	if err != nil {
		return nil, newErr(CodeParseFailed, "canonicalize: %v", err)
	}
	g := &Gate{
		doc:         doc,
		fingerprint: fp,
		keyID:       keyID,
		now:         time.Now,
	}
	g.rate = float64(doc.RateLimitPerMinute) / 60.0
	g.burst = float64(doc.RateLimitPerMinute)
	g.tokens = g.burst
	g.last = g.now()
	return g, nil
}

func (d *Document) validate(now time.Time) error {
	missing := func(f string) error { return newErr(CodeMissingField, "%s", f) }
	switch {
	case d.Version == "":
		return missing("version")
	case d.AuthorizedBy == "":
		return missing("authorized_by")
	case d.IssuedAt == "":
		return missing("issued_at")
	case d.ExpiresAt == "":
		return missing("expires_at")
	case d.RulesOfEngagementSHA256 == "":
		return missing("rules_of_engagement_sha256")
	case len(d.Methods) == 0:
		return missing("methods")
	case d.RateLimitPerMinute <= 0:
		return missing("rate_limit_per_minute")
	case d.MaxBodyBytes <= 0:
		return missing("max_body_bytes")
	}
	if len(d.RulesOfEngagementSHA256) != 64 {
		return newErr(CodeInvalidField, "rules_of_engagement_sha256 must be 64 hex chars")
	}
	if _, err := hex.DecodeString(d.RulesOfEngagementSHA256); err != nil {
		return newErr(CodeInvalidField, "rules_of_engagement_sha256 is not hex")
	}
	var err error
	if d.issued, err = time.Parse(time.RFC3339, d.IssuedAt); err != nil {
		return newErr(CodeInvalidField, "issued_at: %v", err)
	}
	if d.expires, err = time.Parse(time.RFC3339, d.ExpiresAt); err != nil {
		return newErr(CodeInvalidField, "expires_at: %v", err)
	}
	if !d.expires.After(d.issued) {
		return newErr(CodeInvalidField, "expires_at must be after issued_at")
	}
	if now.Before(d.issued) {
		return newErr(CodeNotYetValid, "issued_at %s is in the future", d.IssuedAt)
	}
	if !now.Before(d.expires) {
		return newErr(CodeExpired, "expired at %s", d.ExpiresAt)
	}
	if len(d.Targets) == 0 {
		return newErr(CodeNoTargets, "scope lists zero targets")
	}
	for i := range d.Targets {
		t := &d.Targets[i]
		t.Host = strings.ToLower(strings.TrimSpace(t.Host))
		if t.Host == "" {
			return newErr(CodeMissingField, "targets[%d].host", i)
		}
		if ip, ok := canonicalIP(t.Host); ok {
			t.Host = ip
		} else {
			if strings.ContainsAny(t.Host, "/:@? #[]") {
				return newErr(CodeInvalidField, "targets[%d].host %q must be a bare hostname or IP", i, t.Host)
			}
			if t.Host == "*" || strings.HasSuffix(t.Host, ".*") || strings.Contains(strings.TrimPrefix(t.Host, "*."), "*") {
				return newErr(CodeInvalidField, "targets[%d].host %q: only an explicit leading *. wildcard is allowed", i, t.Host)
			}
		}
		if len(t.Ports) == 0 {
			return newErr(CodeMissingField, "targets[%d].ports", i)
		}
		for _, p := range t.Ports {
			if p < 1 || p > 65535 {
				return newErr(CodeInvalidField, "targets[%d].ports: %d out of range", i, p)
			}
		}
		if len(t.Schemes) == 0 {
			return newErr(CodeMissingField, "targets[%d].schemes", i)
		}
		for j, s := range t.Schemes {
			s = strings.ToLower(s)
			if s != "http" && s != "https" {
				return newErr(CodeInvalidField, "targets[%d].schemes: %q (http or https only)", i, s)
			}
			t.Schemes[j] = s
		}
		for j, p := range t.PathPrefixes {
			if !strings.HasPrefix(p, "/") {
				return newErr(CodeInvalidField, "targets[%d].path_prefixes[%d] must start with /", i, j)
			}
		}
	}
	for i, m := range d.Methods {
		d.Methods[i] = strings.ToUpper(strings.TrimSpace(m))
		if d.Methods[i] == "" {
			return newErr(CodeInvalidField, "methods[%d] is empty", i)
		}
	}
	return nil
}

// canonicalIP returns the canonical textual form of an IP literal.
func canonicalIP(host string) (string, bool) {
	h := strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	addr, err := netip.ParseAddr(h)
	if err != nil {
		// Accept legacy forms net.ParseIP understands (none beyond netip today,
		// but keep the two-stage parse so odd inputs are normalized or rejected).
		ip := net.ParseIP(h)
		if ip == nil {
			return "", false
		}
		return ip.String(), true
	}
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	return addr.WithZone("").String(), true
}

// fingerprint is sha256 over the canonical JSON of the document minus its
// signature. Map keys are sorted by encoding/json.
func fingerprint(raw map[string]any) (string, error) {
	cp := make(map[string]any, len(raw))
	for k, v := range raw {
		if k == "signature" {
			continue
		}
		cp[k] = v
	}
	b, err := json.Marshal(cp)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// Fingerprint is the sha256 of the canonical (unsigned) document.
func (g *Gate) Fingerprint() string { return g.fingerprint }

// KeyID is the trust-store key that signed the document.
func (g *Gate) KeyID() string { return g.keyID }

// Document returns a copy of the parsed scope.
func (g *Gate) Document() Document { return g.doc }

// MaxBodyBytes is the response body cap the scope demands.
func (g *Gate) MaxBodyBytes() int64 { return g.doc.MaxBodyBytes }

func (g *Gate) deny(reason string, format string, a ...any) model.Decision {
	return model.Decision{
		Effect:     model.EffectDeny,
		Reason:     reason,
		Rationale:  fmt.Sprintf(format, a...),
		PolicyHash: g.fingerprint,
	}
}

// Allow decides whether targetURL is inside the authorized scope.
func (g *Gate) Allow(_ context.Context, targetURL string) model.Decision {
	if g == nil {
		return model.Decision{Effect: model.EffectDeny, Reason: ReasonOutOfScope, Rationale: "no gate"}
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return g.deny(ReasonOutOfScope, "unparseable url: %v", err)
	}
	if !u.IsAbs() {
		return g.deny(ReasonOutOfScope, "url must be absolute")
	}
	if u.User != nil {
		return g.deny(ReasonOutOfScope, "url carries userinfo, which is never permitted")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return g.deny(ReasonOutOfScope, "scheme %q not permitted (http or https only)", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return g.deny(ReasonOutOfScope, "url has no host")
	}
	if strings.HasSuffix(host, ".") {
		return g.deny(ReasonOutOfScope, "host %q has a trailing dot", host)
	}
	if ip, ok := canonicalIP(host); ok {
		host = ip
	}
	port := 0
	if ps := u.Port(); ps != "" {
		port, err = strconv.Atoi(ps)
		if err != nil || port < 1 || port > 65535 {
			return g.deny(ReasonOutOfScope, "invalid port %q", ps)
		}
	} else if scheme == "https" {
		port = 443
	} else {
		port = 80
	}

	var hostMatched []int
	for i, t := range g.doc.Targets {
		if hostMatches(t.Host, host) {
			hostMatched = append(hostMatched, i)
		}
	}
	if len(hostMatched) == 0 {
		return g.deny(ReasonOutOfScope, "host %q is not an authorized target", host)
	}

	// Path checks apply only to targets with prefixes; traversal is always
	// refused when any prefix is in force, before comparison.
	rawPath := u.EscapedPath()
	decPath := u.Path
	if decPath == "" {
		decPath = "/"
	}
	cleaned := path.Clean(decPath)
	if strings.HasSuffix(decPath, "/") && cleaned != "/" {
		cleaned += "/"
	}

	var failures []string
	for _, i := range hostMatched {
		t := g.doc.Targets[i]
		if !containsString(t.Schemes, scheme) {
			failures = append(failures, fmt.Sprintf("target %s: scheme %s not listed", t.Host, scheme))
			continue
		}
		if !containsInt(t.Ports, port) {
			failures = append(failures, fmt.Sprintf("target %s: port %d not listed", t.Host, port))
			continue
		}
		if len(t.PathPrefixes) > 0 {
			if hasTraversal(rawPath, decPath) {
				failures = append(failures, fmt.Sprintf("target %s: path %q contains traversal or encoded dot segments", t.Host, rawPath))
				continue
			}
			ok := false
			for _, p := range t.PathPrefixes {
				if strings.HasPrefix(cleaned, p) {
					ok = true
					break
				}
			}
			if !ok {
				failures = append(failures, fmt.Sprintf("target %s: path %q outside prefixes %v", t.Host, cleaned, t.PathPrefixes))
				continue
			}
		}
		return model.Decision{
			Effect:       model.EffectAllow,
			Reason:       ReasonInScope,
			Rationale:    fmt.Sprintf("matched target %s (%s, port %d)", t.Host, scheme, port),
			MatchedRules: []string{fmt.Sprintf("targets[%d]", i)},
			PolicyHash:   g.fingerprint,
		}
	}
	return g.deny(ReasonOutOfScope, "%s", strings.Join(failures, "; "))
}

// hasTraversal reports whether a path has a ".." segment (decoded) or any
// percent-encoded dot, which a target server might decode a second time.
func hasTraversal(raw, decoded string) bool {
	for _, seg := range strings.Split(decoded, "/") {
		if seg == ".." || seg == "." {
			return true
		}
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "%2e") || strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") || strings.Contains(lower, "%25") {
		return true
	}
	return strings.Contains(decoded, "\\")
}

func hostMatches(target, host string) bool {
	if strings.HasPrefix(target, "*.") {
		suffix := target[1:] // ".example.com"
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	return target == host
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func containsInt(xs []int, n int) bool {
	for _, x := range xs {
		if x == n {
			return true
		}
	}
	return false
}

// AllowMethod decides whether an HTTP method is permitted by the scope.
func (g *Gate) AllowMethod(m string) model.Decision {
	m = strings.ToUpper(strings.TrimSpace(m))
	if containsString(g.doc.Methods, m) {
		return model.Decision{Effect: model.EffectAllow, Reason: ReasonMethodAllowed, Rationale: "method " + m + " listed", PolicyHash: g.fingerprint}
	}
	return g.deny(ReasonMethodDenied, "method %q not in %v", m, g.doc.Methods)
}

// Take consumes one request token or returns RATE_LIMITED.
func (g *Gate) Take() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	elapsed := now.Sub(g.last).Seconds()
	if elapsed > 0 {
		g.tokens += elapsed * g.rate
		if g.tokens > g.burst {
			g.tokens = g.burst
		}
		g.last = now
	}
	if g.tokens < 1 {
		return ErrRateLimited
	}
	g.tokens--
	return nil
}

// SetClock replaces the limiter clock (tests).
func (g *Gate) SetClock(now func() time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.now = now
	g.last = now()
}

// Targets lists the authorized hosts, sorted, for display.
func (g *Gate) Targets() []string {
	out := make([]string, 0, len(g.doc.Targets))
	for _, t := range g.doc.Targets {
		for _, p := range t.Ports {
			out = append(out, net.JoinHostPort(t.Host, strconv.Itoa(p)))
		}
	}
	sort.Strings(out)
	return out
}

// IsCode reports whether err carries the given scope error code.
func IsCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}
