package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func write(t *testing.T, dir, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSummarize(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "run.json", model.RunReport{
		RunID: "r1", Config: "fake-only", Verdict: "PASS", Labs: []string{"synthetic-ops"},
		Models:       []model.ModelResult{{Ref: model.ModelRef{Provider: "fake", Model: "fake-model"}}},
		Precision:    map[string]model.PRF{"fake-model@synthetic-ops": {TP: 2, FP: 0, FN: 13}},
		AuditHead:    strings.Repeat("ab", 32),
		AuditCount:   26,
		BudgetCapUSD: 15,
	})
	write(t, dir, "findings.redacted.json", []model.Finding{{
		Class: "security-headers", State: model.StateValidated, Severity: "low",
		Location:   model.Location{Method: "GET", PathTemplate: "/"},
		Validation: &model.ValidationResult{Evidence: []model.Evidence{{}, {}}},
	}})
	write(t, dir, "reconcile.json", map[string]any{"checked": 4, "mismatches": []any{}})

	var out bytes.Buffer
	if err := summarize(dir, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"r1: PASS", "scripted stand-in", "1 reported: 1 validated, 0 theorized, 0 refuted",
		"2 checker observations", "4 claimed by the model", "0 mismatches",
		"2 of 15 planted weaknesses found, 0 false positives", "26 records",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary lacks %q:\n%s", want, out.String())
		}
	}
}

func TestSummarizeMissingRun(t *testing.T) {
	if err := summarize(t.TempDir(), &bytes.Buffer{}); err == nil {
		t.Fatal("expected an error for a directory without run.json")
	}
}
