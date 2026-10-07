// Package syntheticops adapts the deliberately vulnerable synthetic-ops lab
// (labs/synthetic-ops, a separate Go module) to the labs.Lab interface.
package syntheticops

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
)

// Name is the lab identifier used in findings and ground truth.
const Name = "synthetic-ops"

// ReadmeMarker is the string in labs/synthetic-ops/README.md that the
// path-traversal checker looks for after reading ../README.md.
const ReadmeMarker = "SYNTH-LAB-README-MARKER-0001"

// SecretMarker is returned by /debug/config (info-disclosure).
const SecretMarker = "SYNTH-SECRET-MARKER-0001"

// LEAD: keep in sync with handlers.SeedAccounts in the lab module; the two
// modules cannot import each other.
var seedAccounts = []labs.Account{
	{Role: "admin", Username: "admin", Password: "admin123"},
	{Role: "user", Username: "alice", Password: "Wonderland-1"},
	{Role: "user", Username: "bob", Password: "Builder-22"},
}

// Lab is the synthetic-ops adapter.
type Lab struct {
	baseURL string
	gtPath  string
	http    *httpx.Client

	once    sync.Once
	entries []labs.GroundTruthEntry
	gtOK    bool
}

// New returns an adapter for a synthetic-ops instance at baseURL. labDir is
// the checkout of labs/synthetic-ops (ground_truth.json is read from it).
func New(baseURL, labDir string, client *httpx.Client) *Lab {
	return &Lab{
		baseURL: strings.TrimRight(baseURL, "/"),
		gtPath:  filepath.Join(labDir, "ground_truth.json"),
		http:    client,
	}
}

func (l *Lab) Name() string    { return Name }
func (l *Lab) BaseURL() string { return l.baseURL }

// Health checks /healthz.
func (l *Lab) Health(ctx context.Context) error {
	status, _, body, err := l.http.Do(ctx, "GET", l.baseURL+"/healthz", nil, nil)
	if err != nil {
		return err
	}
	if status != 200 || !strings.Contains(string(body), `"ok":true`) {
		return fmt.Errorf("synthetic-ops: unhealthy: status %d", status)
	}
	return nil
}

// Setup is a no-op: the lab seeds itself on start.
func (l *Lab) Setup(context.Context) error { return nil }

// Accounts returns the seeded accounts; the admin entry is the planted default.
func (l *Lab) Accounts() []labs.Account {
	out := make([]labs.Account, len(seedAccounts))
	copy(out, seedAccounts)
	return out
}

// GroundTruth loads ground_truth.json once. The list is exhaustive.
func (l *Lab) GroundTruth() ([]labs.GroundTruthEntry, bool) {
	l.once.Do(func() {
		entries, exhaustive, err := labs.LoadGroundTruth(l.gtPath)
		if err != nil {
			return
		}
		l.entries, l.gtOK = entries, exhaustive
	})
	if !l.gtOK {
		return nil, false
	}
	return append([]labs.GroundTruthEntry(nil), l.entries...), true
}

// Login posts to /api/login and returns a bearer header and the user id.
func (l *Lab) Login(ctx context.Context, acct labs.Account) (map[string]string, string, error) {
	body, _ := json.Marshal(map[string]string{"username": acct.Username, "password": acct.Password})
	status, _, resp, err := l.http.Do(ctx, "POST", l.baseURL+"/api/login", map[string]string{"Content-Type": "application/json"}, body)
	if err != nil {
		return nil, "", err
	}
	if status == 401 || status == 403 {
		return nil, "", fmt.Errorf("synthetic-ops: login %s: status %d: %w", acct.Username, status, labs.ErrLoginRejected)
	}
	if status != 200 {
		return nil, "", fmt.Errorf("synthetic-ops: login %s: status %d", acct.Username, status)
	}
	var out struct {
		Token  string `json:"token"`
		UserID int    `json:"user_id"`
	}
	if err := json.Unmarshal(resp, &out); err != nil || out.Token == "" {
		return nil, "", fmt.Errorf("synthetic-ops: login %s: bad response", acct.Username)
	}
	return map[string]string{"Authorization": "Bearer " + out.Token}, strconv.Itoa(out.UserID), nil
}

var _ labs.Lab = (*Lab)(nil)
var _ labs.Authenticator = (*Lab)(nil)
