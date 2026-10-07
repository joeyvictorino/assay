// Package labs defines the target-lab abstraction and its ground-truth
// format. Adapters live in subpackages and make every request through
// internal/httpx, so they are gated and audited like any other caller.
package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// Account is a lab login. Role "admin" marks the lab's documented default
// administrator credential; the default-creds checker uses it.
type Account struct {
	Role     string `json:"role" yaml:"role"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

// GroundTruthEntry is one planted or well-documented weakness in a lab.
//
// LEAD: S2's overlap package defines a compatible struct; unify here. The
// AuthRequired, Marker and Description fields are additions S3 needs for the
// auth-missing and marker checkers; they are optional in the JSON.
type GroundTruthEntry struct {
	ID           string         `json:"id"`
	Lab          string         `json:"lab"`
	Class        model.Class    `json:"class"`
	Method       string         `json:"method"`
	PathTemplate string         `json:"path_template"`
	Param        string         `json:"param,omitempty"`
	Severity     model.Severity `json:"severity"`
	CWE          string         `json:"cwe,omitempty"`
	SourceRef    string         `json:"source_ref,omitempty"`
	AuthRequired bool           `json:"auth_required,omitempty"`
	Marker       string         `json:"marker,omitempty"`
	Description  string         `json:"description,omitempty"`
}

// Lab is a target the harness is authorized to test.
type Lab interface {
	Name() string
	BaseURL() string
	Health(ctx context.Context) error
	Setup(ctx context.Context) error
	Accounts() []Account
	// GroundTruth returns the known weaknesses and whether the list is
	// exhaustive (true for synthetic labs with complete ground truth; false
	// for hand-made partial reference lists or when nothing is known).
	GroundTruth() ([]GroundTruthEntry, bool)
}

// ErrLoginRejected is wrapped by Authenticator.Login when the lab rejected
// the credential itself (as opposed to a gate denial or transport error).
// Checkers refute only on this error; anything else is inconclusive.
var ErrLoginRejected = errors.New("login rejected by lab")

// Authenticator is implemented by labs that can hand out a session for an
// Account. The returned headers are attached to subsequent requests; userID
// is the lab's identifier for the account (used to fill {id} templates).
type Authenticator interface {
	Login(ctx context.Context, acct Account) (headers map[string]string, userID string, err error)
}

// groundTruthFile is the on-disk envelope. A bare array is also accepted.
type groundTruthFile struct {
	Lab                    string             `json:"lab"`
	ReferenceNotExhaustive bool               `json:"reference_not_exhaustive"`
	Entries                []GroundTruthEntry `json:"entries"`
}

// ParseGroundTruth accepts either a JSON array of entries (exhaustive ground
// truth) or an object {lab, reference_not_exhaustive, entries}.
func ParseGroundTruth(data []byte) (entries []GroundTruthEntry, exhaustive bool, err error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, false, fmt.Errorf("ground truth: empty document")
	}
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, false, fmt.Errorf("ground truth: %w", err)
		}
		exhaustive = true
	} else {
		var f groundTruthFile
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, false, fmt.Errorf("ground truth: %w", err)
		}
		entries = f.Entries
		exhaustive = !f.ReferenceNotExhaustive
	}
	if err := validateEntries(entries); err != nil {
		return nil, false, err
	}
	return entries, exhaustive, nil
}

// LoadGroundTruth reads and parses a ground-truth or reference file.
func LoadGroundTruth(path string) ([]GroundTruthEntry, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("ground truth: %w", err)
	}
	return ParseGroundTruth(data)
}

func validateEntries(entries []GroundTruthEntry) error {
	valid := map[model.Class]bool{}
	for _, c := range model.AllClasses {
		valid[c] = true
	}
	seen := map[string]bool{}
	for i, e := range entries {
		if e.ID == "" {
			return fmt.Errorf("ground truth: entry %d has no id", i)
		}
		if seen[e.ID] {
			return fmt.Errorf("ground truth: duplicate id %s", e.ID)
		}
		seen[e.ID] = true
		if !valid[e.Class] {
			return fmt.Errorf("ground truth: %s: unknown class %q", e.ID, e.Class)
		}
		if e.Severity.Rank() == 0 && e.Severity != model.SevInfo {
			return fmt.Errorf("ground truth: %s: unknown severity %q", e.ID, e.Severity)
		}
		if !strings.HasPrefix(e.PathTemplate, "/") {
			return fmt.Errorf("ground truth: %s: path_template %q must start with /", e.ID, e.PathTemplate)
		}
		if e.Method == "" {
			return fmt.Errorf("ground truth: %s: method required", e.ID)
		}
	}
	return nil
}

// Find returns the first entry matching method and path template (case-
// insensitive method), or nil.
func Find(entries []GroundTruthEntry, method, pathTemplate string) *GroundTruthEntry {
	for i := range entries {
		e := &entries[i]
		if strings.EqualFold(e.Method, method) && e.PathTemplate == pathTemplate {
			return e
		}
	}
	return nil
}

// FindByClass returns the first entry of the given class matching the path
// template, or nil.
func FindByClass(entries []GroundTruthEntry, class model.Class, pathTemplate string) *GroundTruthEntry {
	for i := range entries {
		e := &entries[i]
		if e.Class == class && e.PathTemplate == pathTemplate {
			return e
		}
	}
	return nil
}

// FindByID returns the entry with the given id, or nil.
func FindByID(entries []GroundTruthEntry, id string) *GroundTruthEntry {
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i]
		}
	}
	return nil
}
