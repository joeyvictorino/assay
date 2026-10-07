// Package finding derives deterministic identities for findings and
// enforces their state machine. It depends only on the standard library and
// internal/model so every stream can use it.
package finding

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/joeyvictorino/assay/internal/model"
)

// MaxSummaryRunes bounds Finding.Summary.
const MaxSummaryRunes = 280

var (
	// ErrInvalidTransition is returned by Transition for a disallowed move.
	ErrInvalidTransition = errors.New("finding: invalid state transition")
	// ErrInvalidClass is returned when a class string is not in the taxonomy.
	ErrInvalidClass = errors.New("finding: invalid class")
)

var (
	uuidRe    = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	numericRe = regexp.MustCompile(`/\d+(/|$)`)
	slashesRe = regexp.MustCompile(`/{2,}`)
)

// NormalizePath turns a raw URL or path into a path template: lowercase,
// scheme/host/query/fragment stripped, numeric segments replaced by {id},
// UUID segments by {uuid}, repeated slashes collapsed, trailing slash
// removed, leading slash kept. The empty path becomes "/".
func NormalizePath(p string) string {
	p = strings.TrimSpace(p)
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if i := strings.Index(p, "://"); i >= 0 {
		rest := p[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			p = rest[j:]
		} else {
			p = "/"
		}
	} else if strings.HasPrefix(p, "//") {
		// scheme-relative URL: //host/path
		rest := p[2:]
		if j := strings.Index(rest, "/"); j >= 0 {
			p = rest[j:]
		} else {
			p = "/"
		}
	}
	p = strings.ToLower(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = slashesRe.ReplaceAllString(p, "/")
	p = uuidRe.ReplaceAllString(p, "{uuid}")
	// Loop: a match consumes the following slash, so adjacent numeric
	// segments need a second pass.
	for {
		next := numericRe.ReplaceAllString(p, "/{id}$1")
		if next == p {
			break
		}
		p = next
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	if p == "" {
		p = "/"
	}
	return p
}

// keyFields is marshalled with encoding/json; struct field order is the
// alphabetical order of the JSON names, which makes the output canonical
// without a separate canonicalizer.
type keyFields struct {
	Class        string `json:"class"`
	Lab          string `json:"lab"`
	Method       string `json:"method"`
	Param        string `json:"param"`
	PathTemplate string `json:"path_template"`
}

// DedupKey is the hex sha256 of the canonical JSON of
// {class, lab, method, param, path_template}. Method is upper-cased and the
// path is normalized so callers cannot produce two keys for one location.
func DedupKey(lab string, class model.Class, method, pathTemplate, param string) string {
	b, _ := json.Marshal(keyFields{
		Class:        string(class),
		Lab:          lab,
		Method:       strings.ToUpper(strings.TrimSpace(method)),
		Param:        strings.TrimSpace(param),
		PathTemplate: NormalizePath(pathTemplate),
	})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ValidClass reports whether s names a class in model.AllClasses.
func ValidClass(s string) (model.Class, bool) {
	c := model.Class(strings.ToLower(strings.TrimSpace(s)))
	for _, k := range model.AllClasses {
		if k == c {
			return c, true
		}
	}
	return "", false
}

// ValidSeverity reports whether s is one of the five severities.
func ValidSeverity(s string) (model.Severity, bool) {
	sev := model.Severity(strings.ToLower(strings.TrimSpace(s)))
	switch sev {
	case model.SevInfo, model.SevLow, model.SevMedium, model.SevHigh, model.SevCritical:
		return sev, true
	}
	return "", false
}

// TruncateSummary bounds s to MaxSummaryRunes runes, valid UTF-8 preserved.
func TruncateSummary(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= MaxSummaryRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:MaxSummaryRunes])
}

// Params carries the inputs to NewFinding.
type Params struct {
	RunID    string
	Lab      string
	Class    model.Class
	Method   string
	Path     string // raw; normalized here
	Param    string
	Severity model.Severity
	Summary  string
	Model    model.ModelRef
	// ToolCallIDs are the transcript-side ids the model cited as evidence.
	ToolCallIDs []string
	PromptHash  string
	Now         time.Time
}

// NewFinding builds a theorized finding with a derived ID and DedupKey.
// It returns ErrInvalidClass for a class outside the taxonomy.
func NewFinding(p Params) (model.Finding, error) {
	class, ok := ValidClass(string(p.Class))
	if !ok {
		return model.Finding{}, fmt.Errorf("%w: %q", ErrInvalidClass, p.Class)
	}
	if p.Now.IsZero() {
		p.Now = time.Now().UTC()
	}
	sev := p.Severity
	if _, ok := ValidSeverity(string(sev)); !ok {
		sev = model.SevInfo
	}
	method := strings.ToUpper(strings.TrimSpace(p.Method))
	tmpl := NormalizePath(p.Path)
	key := DedupKey(p.Lab, class, method, tmpl, p.Param)
	ids := append([]string(nil), p.ToolCallIDs...)
	sort.Strings(ids)
	return model.Finding{
		ID:       "f-" + key[:16],
		DedupKey: key,
		RunID:    p.RunID,
		Lab:      p.Lab,
		Class:    class,
		Location: model.Location{
			Method:       method,
			PathTemplate: tmpl,
			Param:        strings.TrimSpace(p.Param),
		},
		Severity:    sev,
		State:       model.StateTheorized,
		Summary:     TruncateSummary(p.Summary),
		Model:       p.Model,
		PromptHash:  p.PromptHash,
		ToolCallIDs: ids,
		FirstSeen:   p.Now,
		LastSeen:    p.Now,
	}, nil
}

// Merge folds incoming into existing: earliest FirstSeen, latest LastSeen,
// evidence and tool-call ids unioned, the higher severity kept, and the
// non-theorized state kept if exactly one side has one. Identity fields
// come from existing.
func Merge(existing, incoming model.Finding) model.Finding {
	out := existing
	if !incoming.FirstSeen.IsZero() && (out.FirstSeen.IsZero() || incoming.FirstSeen.Before(out.FirstSeen)) {
		out.FirstSeen = incoming.FirstSeen
	}
	if incoming.LastSeen.After(out.LastSeen) {
		out.LastSeen = incoming.LastSeen
	}
	if incoming.Severity.Rank() > out.Severity.Rank() {
		out.Severity = incoming.Severity
	}
	if out.Summary == "" {
		out.Summary = TruncateSummary(incoming.Summary)
	}
	if out.State == model.StateTheorized && incoming.State != model.StateTheorized && incoming.State != "" {
		out.State = incoming.State
		if incoming.Validation != nil {
			out.Validation = incoming.Validation
		}
	}
	out.Evidence = unionEvidence(out.Evidence, incoming.Evidence)
	out.ToolCallIDs = unionStrings(out.ToolCallIDs, incoming.ToolCallIDs)
	out.ControlPlane = unionAuditRefs(out.ControlPlane, incoming.ControlPlane)
	return out
}

func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range append(append([]string(nil), a...), b...) {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func unionEvidence(a, b []model.Evidence) []model.Evidence {
	seen := map[string]bool{}
	var out []model.Evidence
	for _, e := range append(append([]model.Evidence(nil), a...), b...) {
		k, _ := json.Marshal(e)
		if seen[string(k)] {
			continue
		}
		seen[string(k)] = true
		out = append(out, e)
	}
	return out
}

func unionAuditRefs(a, b []model.AuditRef) []model.AuditRef {
	seen := map[model.AuditRef]bool{}
	var out []model.AuditRef
	for _, r := range append(append([]model.AuditRef(nil), a...), b...) {
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RunID != out[j].RunID {
			return out[i].RunID < out[j].RunID
		}
		return out[i].Seq < out[j].Seq
	})
	return out
}

// Transition moves f to state to. Only theorized -> validated, refuted or
// declined is permitted; every other move is ErrInvalidTransition.
func Transition(f *model.Finding, to model.FindingState) error {
	if f == nil {
		return fmt.Errorf("%w: nil finding", ErrInvalidTransition)
	}
	if f.State != model.StateTheorized {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, f.State, to)
	}
	switch to {
	case model.StateValidated, model.StateRefuted, model.StateDeclined:
		f.State = to
		return nil
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, f.State, to)
}
