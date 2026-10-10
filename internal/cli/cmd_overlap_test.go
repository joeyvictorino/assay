package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/overlap"
)

func writeJSONOverlap(t *testing.T, dir, name string, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func mkTestFinding(modelName, class, path, param string) model.Finding {
	return model.Finding{
		Lab: "lab-a", Class: model.Class(class),
		Location: model.Location{Method: "GET", PathTemplate: path, Param: param},
		Model:    model.ModelRef{Provider: "p", Model: modelName, Agent: "probe"},
	}
}

func TestOverlapCommand(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONOverlap(t, dir, "a.json", []model.Finding{mkTestFinding("alpha", "idor", "/users/{id}", "id"), mkTestFinding("alpha", "sqli", "/search", "q")})
	b := writeJSONOverlap(t, dir, "b.json", []model.Finding{mkTestFinding("beta", "idor", "/users/{id}", "id"), mkTestFinding("beta", "csrf", "/transfer", "")})
	gt := writeJSONOverlap(t, dir, "gt.json", []overlap.GroundTruthEntry{
		{Lab: "lab-a", Class: "idor", Method: "GET", PathTemplate: "/users/{id}", Param: "id"},
		{Lab: "lab-a", Class: "csrf", Method: "POST", PathTemplate: "/transfer", Param: ""},
	})
	out := filepath.Join(dir, "overlap.json")

	var stdout, stderr bytes.Buffer
	code := Main([]string{"overlap", "--inputs", a + "," + b, "--out", out, "--ground-truth", gt}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var rep overlap.Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Overlap.Models) != 2 || rep.Overlap.Models[0] != "alpha" || rep.Overlap.Models[1] != "beta" {
		t.Fatalf("models = %v", rep.Overlap.Models)
	}
	if rep.Overlap.Union != 3 || rep.Overlap.Intersection != 1 {
		t.Fatalf("union=%d intersection=%d", rep.Overlap.Union, rep.Overlap.Intersection)
	}
	if len(rep.Overlap.Inputs) != 2 || rep.Overlap.Inputs[0].Name != "a.json" || len(rep.Overlap.Inputs[0].SHA256) != 64 {
		t.Fatalf("inputs = %+v", rep.Overlap.Inputs)
	}
	// alpha: idor is TP, sqli is FP, csrf is FN.
	if p := rep.Precision["alpha"]; p.TP != 1 || p.FP != 1 || p.FN != 1 {
		t.Fatalf("alpha prf = %+v", p)
	}
	// beta: idor TP; csrf GET vs ground truth POST -> FP + FN.
	if p := rep.Precision["beta"]; p.TP != 1 || p.FP != 1 || p.FN != 1 {
		t.Fatalf("beta prf = %+v", p)
	}
	if !strings.Contains(stdout.String(), "models=2 union=3 intersection=1") {
		t.Fatalf("stdout = %s", stdout.String())
	}

	// --check against the file just written passes and is byte-identical.
	out2 := filepath.Join(dir, "overlap2.json")
	stdout.Reset()
	stderr.Reset()
	code = Main([]string{"overlap", "--inputs", b + "," + a, "--out", out2, "--ground-truth", gt, "--check", out}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("check exit %d: %s", code, stderr.String())
	}
	raw2, _ := os.ReadFile(out2)
	if !bytes.Equal(raw, raw2) {
		t.Fatalf("input order changed the bytes:\n%s\n---\n%s", raw, raw2)
	}
	if !strings.Contains(stdout.String(), "check OK") {
		t.Fatalf("stdout = %s", stdout.String())
	}

	// --check against a stale file exits 1.
	stale := filepath.Join(dir, "stale.json")
	if err := os.WriteFile(stale, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	code = Main([]string{"overlap", "--inputs", a + "," + b, "--out", out2, "--ground-truth", gt, "--check", stale}, &stdout, &stderr)
	if code != ExitBlocked || !strings.Contains(stderr.String(), "differs") {
		t.Fatalf("stale check exit %d: %s", code, stderr.String())
	}
}

func TestOverlapCommandWithoutGroundTruthOmitsPrecision(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONOverlap(t, dir, "a.json", []model.Finding{mkTestFinding("alpha", "idor", "/users/{id}", "id")})
	out := filepath.Join(dir, "o.json")
	var stdout, stderr bytes.Buffer
	if code := Main([]string{"overlap", "--inputs", a, "--out", out}, &stdout, &stderr); code != ExitPass {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, _ := os.ReadFile(out)
	if bytes.Contains(raw, []byte(`"precision"`)) {
		t.Fatalf("precision present without ground truth: %s", raw)
	}
}

func TestOverlapCommandModelNameFallsBackToFile(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONOverlap(t, dir, "mystery.json", []model.Finding{{Lab: "l", Class: "idor", Location: model.Location{Method: "GET", PathTemplate: "/x"}}})
	out := filepath.Join(dir, "o.json")
	var stdout, stderr bytes.Buffer
	if code := Main([]string{"overlap", "--inputs", a, "--out", out}, &stdout, &stderr); code != ExitPass {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, _ := os.ReadFile(out)
	var rep overlap.Report
	_ = json.Unmarshal(raw, &rep)
	if len(rep.Overlap.Models) != 1 || rep.Overlap.Models[0] != "mystery" {
		t.Fatalf("models = %v", rep.Overlap.Models)
	}
}

func TestOverlapCommandErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(bad, []byte("{not json"), 0o644)
	good := writeJSONOverlap(t, dir, "good.json", []model.Finding{})
	out := filepath.Join(dir, "o.json")
	tests := []struct {
		name string
		args []string
	}{
		{"missing flags", []string{"overlap"}},
		{"missing out", []string{"overlap", "--inputs", good}},
		{"unreadable input", []string{"overlap", "--inputs", filepath.Join(dir, "nope.json"), "--out", out}},
		{"bad json", []string{"overlap", "--inputs", bad, "--out", out}},
		{"bad ground truth", []string{"overlap", "--inputs", good, "--out", out, "--ground-truth", bad}},
		{"missing check file", []string{"overlap", "--inputs", good, "--out", out, "--check", filepath.Join(dir, "nope.json")}},
		{"unknown flag", []string{"overlap", "--bogus"}},
		{"empty inputs", []string{"overlap", "--inputs", ",", "--out", out}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Main(tt.args, &stdout, &stderr); code != ExitError {
				t.Fatalf("exit %d, want %d: %s", code, ExitError, stderr.String())
			}
		})
	}
}

// A model that reported nothing stays in the matrix, named from its
// findings.<model>.json file, exactly as the run computes it.
func TestOverlapCommandKeepsModelWithNoFindings(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONOverlap(t, dir, "findings.alpha.json", []model.Finding{mkTestFinding("alpha", "idor", "/users/{id}", "id")})
	b := writeJSONOverlap(t, dir, "findings.beta-1.json", []model.Finding{})
	out := filepath.Join(dir, "o.json")
	var stdout, stderr bytes.Buffer
	if code := Main([]string{"overlap", "--inputs", a + "," + b, "--out", out}, &stdout, &stderr); code != ExitPass {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	raw, _ := os.ReadFile(out)
	var rep overlap.Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatal(err)
	}
	want := overlap.Compute(map[string][]model.Finding{"alpha": {mkTestFinding("alpha", "idor", "/users/{id}", "id")}, "beta-1": {}}, nil)
	if strings.Join(rep.Overlap.Models, ",") != "alpha,beta-1" || rep.Overlap.Intersection != want.Intersection || rep.Overlap.Union != want.Union {
		t.Fatalf("overlap = %+v, want models alpha,beta-1 union %d intersection %d", rep.Overlap, want.Union, want.Intersection)
	}
}
