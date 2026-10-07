package remediate

import (
	"context"
	"encoding/json"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/model"
)

const labDir = "../../labs/synthetic-ops"

func finding(id string, class model.Class) model.Finding {
	return model.Finding{ID: id, Lab: "synthetic-ops", Class: class, Severity: model.SevHigh, State: model.StateValidated,
		Location: model.Location{Method: "GET", PathTemplate: "/api/search", Param: "q"}}
}

func TestTemplatesForEveryClass(t *testing.T) {
	g := &Generator{FixesDir: filepath.Join(labDir, "fixes")}
	for _, c := range model.AllClasses {
		f := finding("F-"+string(c), c)
		rem := g.Generate(f, nil)
		if rem.ConfigChange == "" || strings.Contains(rem.ConfigChange, "{") {
			t.Errorf("%s: config change %q", c, rem.ConfigChange)
		}
		lines := strings.Split(rem.VirtualPatch, "\n")
		if len(lines) != 2 || !strings.HasPrefix(lines[0], "SecRule ") || !strings.Contains(lines[0], "id:91000") {
			t.Errorf("%s: virtual patch %q", c, rem.VirtualPatch)
			continue
		}
		var js map[string]any
		if err := json.Unmarshal([]byte(lines[1]), &js); err != nil || js["engine"] != "modsecurity" || js["rule"] != lines[0] {
			t.Errorf("%s: json form %q (%v)", c, lines[1], err)
		}
		if rem.CodeDiff != "" || rem.Verified || rem.Draft {
			t.Errorf("%s: unexpected code diff / flags without ground truth", c)
		}
	}
	if len(Classes()) != len(model.AllClasses) {
		t.Fatalf("Classes: %d", len(Classes()))
	}
	// Rule ids are distinct per class.
	seen := map[int]bool{}
	for _, c := range model.AllClasses {
		if seen[RuleID(c)] {
			t.Fatalf("duplicate rule id for %s", c)
		}
		seen[RuleID(c)] = true
	}
}

func TestVirtualPatchShapes(t *testing.T) {
	f := finding("F 1\"x", model.ClassIDOR)
	f.Location = model.Location{Method: "GET", PathTemplate: "/api/users/{id}"}
	vp := VirtualPatch(f)
	if !strings.Contains(vp, `@rx ^/api/users/[^/]+`) || strings.Contains(vp, `F 1"x`) {
		t.Fatalf("%s", vp)
	}
	f = finding("F", model.ClassSQLi)
	f.Location.Param = ""
	if vp := VirtualPatch(f); !strings.Contains(vp, "SecRule ARGS:* ") {
		t.Fatalf("%s", vp)
	}
	f = finding("F", model.Class("made-up"))
	if vp := VirtualPatch(f); !strings.Contains(vp, "assay made-up virtual patch") {
		t.Fatalf("unknown class must fall back to the generic template: %s", vp)
	}
}

func TestCodeDiffOnlyForSyntheticLab(t *testing.T) {
	g := &Generator{FixesDir: filepath.Join(labDir, "fixes")}
	f := finding("F-SQLI", model.ClassSQLi)
	gt := &labs.GroundTruthEntry{ID: "SYN-003", Lab: "synthetic-ops", Class: model.ClassSQLi, SourceRef: "handlers/search.go:17"}
	rem := g.Generate(f, gt)
	if !strings.Contains(rem.CodeDiff, "--- a/handlers/search.go") || !strings.Contains(rem.CodeDiff, "+++ b/handlers/search.go") {
		t.Fatalf("code diff missing: %q", rem.CodeDiff)
	}
	other := *gt
	other.Lab = "juice-shop"
	other.SourceRef = "challenge:x"
	if rem := g.Generate(f, &other); rem.CodeDiff != "" {
		t.Fatal("non-synthetic lab must not get a code diff")
	}
	none := *gt
	none.ID = "SYN-999"
	if rem := g.Generate(f, &none); rem.CodeDiff != "" {
		t.Fatal("missing fix file must leave CodeDiff empty")
	}
	evil := *gt
	evil.ID = "../../go"
	if rem := g.Generate(f, &evil); rem.CodeDiff != "" {
		t.Fatal("ids must not traverse")
	}
	if rem := Generate(f, nil); rem.ConfigChange == "" {
		t.Fatal("package-level Generate")
	}
}

// TestCommittedFixesApply applies every labs/synthetic-ops/fixes/*.diff to a
// copy of the lab and parses the result as Go, so the fixes cannot rot
// silently. Building and re-checking the real lab needs its dependencies and
// runs in CI (ASSAY_LAB_E2E=1) via TestVerifyRealLabFix.
func TestCommittedFixesApply(t *testing.T) {
	diffs, err := filepath.Glob(filepath.Join(labDir, "fixes", "*.diff"))
	if err != nil || len(diffs) < 5 {
		t.Fatalf("want at least 5 committed fixes, got %d (%v)", len(diffs), err)
	}
	entries, _, err := labs.LoadGroundTruth(filepath.Join(labDir, "ground_truth.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diffs {
		id := strings.TrimSuffix(filepath.Base(d), ".diff")
		if labs.FindByID(entries, id) == nil {
			t.Errorf("%s: no ground-truth entry", id)
		}
		data, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		tmp := t.TempDir()
		if err := copyDir(labDir, tmp); err != nil {
			t.Fatal(err)
		}
		if err := ApplyUnified(tmp, string(data)); err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		// Every touched Go file must still parse.
		for _, p := range parsedPaths(string(data)) {
			if !strings.HasSuffix(p, ".go") {
				continue
			}
			src, err := os.ReadFile(filepath.Join(tmp, p))
			if err != nil {
				t.Errorf("%s: %v", id, err)
				continue
			}
			if _, err := parser.ParseFile(token.NewFileSet(), p, src, parser.AllErrors); err != nil {
				t.Errorf("%s: patched %s does not parse: %v", id, p, err)
			}
		}
	}
}

func parsedPaths(diff string) []string {
	var out []string
	for _, l := range strings.Split(diff, "\n") {
		if strings.HasPrefix(l, "+++ b/") {
			out = append(out, strings.TrimPrefix(l, "+++ b/"))
		}
	}
	return out
}

func touchedVulnLine(diff string) bool {
	for _, l := range strings.Split(diff, "\n") {
		if strings.HasPrefix(l, "-") && strings.Contains(l, "// VULN:") {
			return true
		}
	}
	return false
}

func TestFixesRemoveTheirVulnMarker(t *testing.T) {
	diffs, _ := filepath.Glob(filepath.Join(labDir, "fixes", "*.diff"))
	for _, d := range diffs {
		data, _ := os.ReadFile(d)
		if !touchedVulnLine(string(data)) {
			t.Errorf("%s: fix does not remove a // VULN: line", filepath.Base(d))
		}
		for _, l := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(l, "+") && strings.Contains(l, "// VULN:") {
				t.Errorf("%s: fix adds a // VULN: line", filepath.Base(d))
			}
		}
	}
}

func TestApplyUnifiedEdgeCases(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\nthree\nfour\n"), 0o644)
	// Context offset: the hunk claims line 10 but matches at line 2.
	diff := "--- a/a.txt\n+++ b/a.txt\n@@ -10,3 +10,3 @@\n two\n-three\n+THREE\n four\n"
	if err := ApplyUnified(root, diff); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(got) != "one\ntwo\nTHREE\nfour\n" {
		t.Fatalf("%q", got)
	}
	// New file and deletion.
	diff = "--- /dev/null\n+++ b/sub/new.txt\n@@ -0,0 +1,2 @@\n+hello\n+world\n--- a/a.txt\n+++ /dev/null\n@@ -1,4 +0,0 @@\n-one\n-two\n-THREE\n-four\n"
	if err := ApplyUnified(root, diff); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "sub", "new.txt")); string(got) != "hello\nworld\n" {
		t.Fatalf("%q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "a.txt")); !os.IsNotExist(err) {
		t.Fatal("a.txt should be deleted")
	}
	// Mismatched context fails.
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("x\n"), 0o644)
	if err := ApplyUnified(root, "--- a/b.txt\n+++ b/b.txt\n@@ -1 +1 @@\n-nope\n+yes\n"); err == nil || !strings.Contains(err.Error(), "does not apply") {
		t.Fatalf("got %v", err)
	}
	// Traversal and absolute paths refused.
	for _, bad := range []string{"--- a/../x\n+++ b/../x\n@@ -0,0 +1 @@\n+z\n", "--- /etc/x\n+++ /etc/x\n@@ -0,0 +1 @@\n+z\n"} {
		if err := ApplyUnified(root, bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if err := ApplyUnified(root, "nothing here"); err == nil {
		t.Fatal("empty patch must fail")
	}
	if err := ApplyUnified(root, "Binary files a and b differ\n"); err == nil {
		t.Fatal("binary must fail")
	}
}

func TestOrder(t *testing.T) {
	mk := func(id string, sev model.Severity, st model.FindingState, c model.Class) model.Finding {
		return model.Finding{ID: id, Severity: sev, State: st, Class: c}
	}
	in := []model.Finding{
		mk("d", model.SevLow, model.StateValidated, model.ClassCORS),
		mk("b", model.SevCritical, model.StateTheorized, model.ClassSQLi),
		mk("a", model.SevCritical, model.StateValidated, model.ClassSQLi),
		mk("c", model.SevHigh, model.StateRefuted, model.ClassIDOR),
		mk("e", model.SevHigh, model.StateValidated, model.ClassAuthMissing),
		mk("f", model.SevHigh, model.StateValidated, model.ClassIDOR),
	}
	out := Order(in)
	var ids []string
	for _, f := range out {
		ids = append(ids, f.ID)
	}
	if got := strings.Join(ids, ""); got != "abefcd" {
		t.Fatalf("order %s", got)
	}
	if in[0].ID != "d" {
		t.Fatal("input mutated")
	}
}

type hostGate struct{ host string }

func (g hostGate) Allow(_ context.Context, target string) model.Decision {
	u, err := url.Parse(target)
	if err != nil || u.Host != g.host {
		return model.Decision{Effect: model.EffectDeny, Reason: "OUT_OF_SCOPE"}
	}
	return model.Decision{Effect: model.EffectAllow, Reason: "IN_SCOPE"}
}

const minilabDiff = `--- a/cmd/minilab/main.go
+++ b/cmd/minilab/main.go
@@ -8,7 +8,7 @@ import (
 	"net/http"
 )

-const banner = "VULNERABLE"
+const banner = "FIXED"

 func main() {
 	addr := flag.String("addr", "127.0.0.1:9900", "listen address")
`

func TestVerifyWithMinilab(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	var seen string
	recheck := func(baseURL string) (bool, error) {
		u, _ := url.Parse(baseURL)
		c := &httpx.Client{Gate: hostGate{u.Host}, Auditor: &httpx.MemAuditor{}, Timeout: 5 * time.Second}
		_, _, body, err := c.Do(ctx, "GET", baseURL+"/", nil, nil)
		if err != nil {
			return false, err
		}
		seen = string(body)
		return seen == "FIXED", nil
	}
	fixed, err := Verify(ctx, "testdata/minilab", minilabDiff, recheck)
	if err != nil {
		t.Fatal(err)
	}
	if !fixed {
		t.Fatalf("recheck saw %q", seen)
	}
	// The original tree is untouched.
	src, _ := os.ReadFile("testdata/minilab/cmd/minilab/main.go")
	if !strings.Contains(string(src), `"VULNERABLE"`) {
		t.Fatal("Verify modified the source tree")
	}
	// A diff that does not apply is an error, not a false "fixed".
	if _, err := Verify(ctx, "testdata/minilab", "--- a/cmd/minilab/main.go\n+++ b/cmd/minilab/main.go\n@@ -1 +1 @@\n-zzz\n+yyy\n", recheck); err == nil {
		t.Fatal("expected apply error")
	}
	if _, err := Verify(ctx, "testdata/minilab", "", recheck); err == nil {
		t.Fatal("empty diff must error")
	}
	if _, err := Verify(ctx, "testdata/minilab", minilabDiff, nil); err == nil {
		t.Fatal("nil recheck must error")
	}
}

// TestVerifyRealLabFix rebuilds labs/synthetic-ops with the SQLi fix and
// confirms the boolean differential disappears. It needs the lab module's
// dependencies, so it runs only when ASSAY_LAB_E2E=1 (CI).
func TestVerifyRealLabFix(t *testing.T) {
	if os.Getenv("ASSAY_LAB_E2E") != "1" {
		t.Skip("set ASSAY_LAB_E2E=1 to rebuild the real lab")
	}
	diff, err := os.ReadFile(filepath.Join(labDir, "fixes", "SYN-003.diff"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	recheck := func(baseURL string) (bool, error) {
		u, _ := url.Parse(baseURL)
		c := &httpx.Client{Gate: hostGate{u.Host}, Auditor: &httpx.MemAuditor{}, Timeout: 10 * time.Second}
		count := func(q string) (int, error) {
			_, _, body, err := c.Do(ctx, "GET", baseURL+"/api/search?q="+url.QueryEscape(q), nil, nil)
			if err != nil {
				return 0, err
			}
			var arr []any
			if err := json.Unmarshal(body, &arr); err != nil {
				return 0, err
			}
			return len(arr), nil
		}
		tr, err := count("zzz%' OR '1%'='1")
		if err != nil {
			return false, err
		}
		fa, err := count("zzz%' AND '1%'='2")
		if err != nil {
			return false, err
		}
		return tr == fa, nil
	}
	fixed, err := Verify(ctx, labDir, string(diff), recheck)
	if err != nil {
		t.Fatal(err)
	}
	if !fixed {
		t.Fatal("SQLi fix did not remove the differential")
	}
}
