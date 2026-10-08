package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/audit"
	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider/fake"
	"github.com/joeyvictorino/assay/internal/tools"
	"github.com/joeyvictorino/assay/internal/toolsig"
)

// pipelineFixture is everything runPipeline needs, built in a temp dir:
// two loopback lab servers (named like the real adapters expect), a trust
// store holding a throwaway key, a scope document signed with it, and the
// repository's policy, labs directory and PR run config.
type pipelineFixture struct {
	root    string
	opts    runOpts
	canary  string
	restore func()
}

func labHandler(canary string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Lab-Marker", canary)
		switch r.URL.Path {
		case "/healthz":
			io.WriteString(w, `{"ok":true}`)
		case "/rest/admin/application-version":
			io.WriteString(w, `{"version":"test"}`)
		default:
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<html><body>"+canary+"</body></html>")
		}
	})
}

func newPipelineFixture(t *testing.T, canary string) *pipelineFixture {
	t.Helper()
	// Locate the repository from this source file before changing directory.
	repo := repoRoot(t)
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("TMPDIR", root)
	t.Chdir(root)

	var ports []string
	urls := map[string]string{}
	for _, name := range []string{"synthetic-ops", "juice-shop"} {
		srv := httptest.NewServer(labHandler(canary))
		t.Cleanup(srv.Close)
		u, _ := url.Parse(srv.URL)
		ports = append(ports, u.Port())
		urls[name] = srv.URL
	}

	pub, priv := toolsig.GenerateKey()
	trust := filepath.Join(root, "trust")
	must(t, os.MkdirAll(trust, 0o755))
	must(t, os.WriteFile(filepath.Join(trust, "test.pub"), []byte(toolsig.EncodePublicKey(pub)+"\n"), 0o644))
	keyFile := filepath.Join(root, "test.key")
	must(t, os.WriteFile(keyFile, []byte(toolsig.EncodePrivateKey(priv)), 0o600))

	unsigned := filepath.Join(root, "scope.unsigned.yaml")
	must(t, os.WriteFile(unsigned, []byte(fmt.Sprintf(`version: "1"
authorized_by: "unit test (loopback servers only)"
issued_at: "2026-10-01T00:00:00Z"
expires_at: "2099-01-01T00:00:00Z"
rules_of_engagement_sha256: "0000000000000000000000000000000000000000000000000000000000000000"
targets:
  - host: 127.0.0.1
    ports: [%s, %s]
    schemes: [http]
methods: [GET, POST, HEAD, OPTIONS]
rate_limit_per_minute: 60000
max_body_bytes: 1048576
`, ports[0], ports[1])), 0o644))
	signed := filepath.Join(root, "scope.yaml")
	var out, errb bytes.Buffer
	if code := runScope([]string{"sign", "--in", unsigned, "--out", signed, "--key", keyFile}, &out, &errb); code != ExitPass {
		t.Fatalf("scope sign: %d %s", code, errb.String())
	}

	prevManifests, prevFake := manifestsFn, fakeProviderFn
	manifestsFn = func() ([]model.ToolManifest, error) {
		ms, err := tools.Manifests()
		if err != nil {
			return nil, err
		}
		for i := range ms {
			if ms[i], err = toolsig.Sign(ms[i], priv, time.Now()); err != nil {
				return nil, err
			}
		}
		return ms, nil
	}
	f := &pipelineFixture{root: root, canary: canary, restore: func() { manifestsFn, fakeProviderFn = prevManifests, prevFake }}
	t.Cleanup(f.restore)
	f.opts = runOpts{
		config: filepath.Join(repo, "runs", "pr.yaml"), mode: "full", out: filepath.Join(root, "results"),
		scopePath: signed, trust: trust, policyPath: filepath.Join(repo, "policy", "default.yaml"),
		ephemeralKey: true, labNames: []string{"synthetic-ops", "juice-shop"},
		labURLs: []string{"synthetic-ops=" + urls["synthetic-ops"], "juice-shop=" + urls["juice-shop"]},
		labDir:  filepath.Join(repo, "labs"), runID: "unit-run",
	}
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// repoRoot finds the repository root from the test's source file path.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("ASSAY_REPO_ROOT")
	if dir != "" {
		return dir
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate source file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func (f *pipelineFixture) run(t *testing.T) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	code, err := runPipeline(ctx, f.opts, &out, &errb)
	if err != nil {
		t.Fatalf("pipeline: %v\n%s", err, errb.String())
	}
	return code, errb.String()
}

// Two labs, one scripted provider reusing the same tool call ids in both:
// the join must be per task, so nothing is a false mismatch and no finding
// is downgraded by one.
func TestPipelineAcrossLabsHasNoFalseReconcileMismatches(t *testing.T) {
	f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
	fakeProviderFn = fakeScript
	code, stderr := f.run(t)
	if code != ExitPass {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	dir := filepath.Join(f.opts.out, f.opts.runID)

	var rec struct {
		Checked    int               `json:"checked"`
		Mismatches []json.RawMessage `json:"mismatches"`
	}
	readJSON(t, filepath.Join(dir, "reconcile.json"), &rec)
	if len(rec.Mismatches) != 0 || rec.Checked < 6 {
		t.Fatalf("checked=%d mismatches=%s", rec.Checked, rec.Mismatches)
	}

	var findings []model.Finding
	readJSON(t, filepath.Join(dir, "findings.redacted.json"), &findings)
	labsSeen := map[string]int{}
	for _, fd := range findings {
		labsSeen[fd.Lab]++
		if !fd.Reconcile.Checked || !fd.Reconcile.Match {
			t.Errorf("finding %s/%s downgraded by reconciliation: %+v", fd.Lab, fd.Class, fd.Reconcile)
		}
	}
	if len(labsSeen) != 2 {
		t.Fatalf("want findings from both labs, got %v", labsSeen)
	}

	// The audit chain verifies without the key.
	if _, err := audit.Verify(filepath.Join(f.root, ".audit", f.opts.runID+".log"), nil); err != nil {
		t.Fatalf("audit chain: %v", err)
	}
	for _, name := range []string{"run.json", "overlap.json", "costs.json", "report.md", "chains.mmd", "audit.export.jsonl", "ci.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing result file %s", name)
		}
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	must(t, err)
	must(t, json.Unmarshal(b, v))
}

var _ = fake.Options{}
