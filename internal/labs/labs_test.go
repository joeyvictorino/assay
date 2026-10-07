package labs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestParseArrayIsExhaustive(t *testing.T) {
	entries, exhaustive, err := ParseGroundTruth([]byte(`[{"id":"A","lab":"x","class":"sqli","method":"GET","path_template":"/s","severity":"high"}]`))
	if err != nil || !exhaustive || len(entries) != 1 || entries[0].Class != model.ClassSQLi {
		t.Fatalf("entries=%v exhaustive=%v err=%v", entries, exhaustive, err)
	}
}

func TestParseEnvelopeNotExhaustive(t *testing.T) {
	entries, exhaustive, err := ParseGroundTruth([]byte(`{"lab":"j","reference_not_exhaustive":true,"entries":[{"id":"A","lab":"j","class":"idor","method":"GET","path_template":"/u/{id}","severity":"medium"}]}`))
	if err != nil || exhaustive || len(entries) != 1 {
		t.Fatalf("entries=%v exhaustive=%v err=%v", entries, exhaustive, err)
	}
}

func TestParseRejectsBadEntries(t *testing.T) {
	cases := map[string]string{
		"empty":       ``,
		"bad class":   `[{"id":"A","class":"nope","method":"GET","path_template":"/","severity":"low"}]`,
		"bad sev":     `[{"id":"A","class":"sqli","method":"GET","path_template":"/","severity":"huge"}]`,
		"no id":       `[{"class":"sqli","method":"GET","path_template":"/","severity":"low"}]`,
		"dup id":      `[{"id":"A","class":"sqli","method":"GET","path_template":"/","severity":"low"},{"id":"A","class":"sqli","method":"GET","path_template":"/","severity":"low"}]`,
		"bad path":    `[{"id":"A","class":"sqli","method":"GET","path_template":"x","severity":"low"}]`,
		"no method":   `[{"id":"A","class":"sqli","path_template":"/","severity":"low"}]`,
		"not json":    `{{{`,
		"info sev ok": ``,
	}
	for name, doc := range cases {
		if name == "info sev ok" {
			continue
		}
		if _, _, err := ParseGroundTruth([]byte(doc)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, _, err := ParseGroundTruth([]byte(`[{"id":"A","class":"other","method":"GET","path_template":"/","severity":"info"}]`)); err != nil {
		t.Fatalf("info severity must be accepted: %v", err)
	}
}

func TestLoadAndFind(t *testing.T) {
	p := filepath.Join(t.TempDir(), "gt.json")
	if err := os.WriteFile(p, []byte(`[{"id":"A","class":"auth-missing","method":"GET","path_template":"/admin","severity":"high","auth_required":true},
{"id":"B","class":"sqli","method":"get","path_template":"/s","severity":"critical"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, _, err := LoadGroundTruth(p)
	if err != nil {
		t.Fatal(err)
	}
	if e := Find(entries, "GET", "/s"); e == nil || e.ID != "B" {
		t.Fatalf("Find: %+v", e)
	}
	if e := Find(entries, "POST", "/s"); e != nil {
		t.Fatal("method must matter")
	}
	if e := FindByClass(entries, model.ClassAuthMissing, "/admin"); e == nil || !e.AuthRequired {
		t.Fatalf("FindByClass: %+v", e)
	}
	if e := FindByID(entries, "A"); e == nil || e.Class != model.ClassAuthMissing {
		t.Fatalf("FindByID: %+v", e)
	}
	if FindByID(entries, "Z") != nil {
		t.Fatal("missing id must be nil")
	}
	if _, _, err := LoadGroundTruth(filepath.Join(t.TempDir(), "nope.json")); err == nil || !strings.Contains(err.Error(), "ground truth") {
		t.Fatalf("missing file: %v", err)
	}
}

// The committed data files must parse against model.AllClasses.
func TestCommittedFilesParse(t *testing.T) {
	for _, tc := range []struct {
		path       string
		exhaustive bool
		min        int
	}{
		{"../../labs/synthetic-ops/ground_truth.json", true, 12},
		{"../../labs/juice-shop/reference_findings.json", false, 8},
		{"../../labs/dvwa/reference_findings.json", false, 8},
	} {
		entries, exhaustive, err := LoadGroundTruth(tc.path)
		if err != nil {
			t.Errorf("%s: %v", tc.path, err)
			continue
		}
		if exhaustive != tc.exhaustive || len(entries) < tc.min {
			t.Errorf("%s: exhaustive=%v entries=%d", tc.path, exhaustive, len(entries))
		}
	}
}
