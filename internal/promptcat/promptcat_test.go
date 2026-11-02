package promptcat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(hash, mdl string, state model.FindingState, sev model.Severity, class model.Class) model.Finding {
	return model.Finding{
		PromptHash: hash,
		Model:      model.ModelRef{Model: mdl},
		State:      state,
		Severity:   sev,
		Class:      class,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	cr := Analyze(nil)
	if len(cr.Entries) != 0 {
		t.Fatalf("entries = %d", len(cr.Entries))
	}
	if cr.Total != 0 {
		t.Errorf("total = %d", cr.Total)
	}
}

func TestSinglePrompt(t *testing.T) {
	ff := []model.Finding{
		finding("abc", "gpt-4", model.StateValidated, model.SevHigh, "injection"),
	}
	cr := Analyze(ff)
	if len(cr.Entries) != 1 {
		t.Fatalf("entries = %d", len(cr.Entries))
	}
	e := cr.Entries[0]
	if e.Hash != "abc" {
		t.Errorf("hash = %q", e.Hash)
	}
	if e.UsageCount != 1 {
		t.Errorf("usage = %d", e.UsageCount)
	}
	if e.ValidRate != 100 {
		t.Errorf("valid rate = %.2f", e.ValidRate)
	}
}

func TestMultiplePromptsOrdered(t *testing.T) {
	ff := []model.Finding{
		finding("a", "m1", model.StateValidated, model.SevMedium, "c1"),
		finding("b", "m1", model.StateValidated, model.SevHigh, "c2"),
		finding("b", "m1", model.StateRefuted, model.SevLow, "c3"),
	}
	cr := Analyze(ff)
	if len(cr.Entries) != 2 {
		t.Fatalf("entries = %d", len(cr.Entries))
	}
	if cr.Entries[0].Hash != "b" {
		t.Errorf("first entry = %q, want b (highest usage)", cr.Entries[0].Hash)
	}
}

func TestNoHashFallback(t *testing.T) {
	ff := []model.Finding{
		finding("", "m1", model.StateValidated, model.SevMedium, "c1"),
	}
	cr := Analyze(ff)
	if cr.Entries[0].Hash != "(no-hash)" {
		t.Errorf("hash = %q", cr.Entries[0].Hash)
	}
}

func TestMultipleModels(t *testing.T) {
	ff := []model.Finding{
		finding("h1", "m1", model.StateValidated, model.SevHigh, "c1"),
		finding("h1", "m2", model.StateValidated, model.SevMedium, "c1"),
	}
	cr := Analyze(ff)
	e := cr.Entries[0]
	if len(e.Models) != 2 {
		t.Errorf("models = %d", len(e.Models))
	}
}

func TestMultipleClasses(t *testing.T) {
	ff := []model.Finding{
		finding("h1", "m1", model.StateValidated, model.SevHigh, "injection"),
		finding("h1", "m1", model.StateValidated, model.SevMedium, "ssrf"),
	}
	cr := Analyze(ff)
	e := cr.Entries[0]
	if len(e.Classes) != 2 {
		t.Errorf("classes = %d", len(e.Classes))
	}
}

func TestValidationRate(t *testing.T) {
	ff := []model.Finding{
		finding("h1", "m1", model.StateValidated, model.SevHigh, "c1"),
		finding("h1", "m1", model.StateRefuted, model.SevMedium, "c2"),
		finding("h1", "m1", model.StateValidated, model.SevLow, "c3"),
		finding("h1", "m1", model.StateRefuted, model.SevLow, "c4"),
	}
	cr := Analyze(ff)
	if cr.Entries[0].ValidRate != 50 {
		t.Errorf("valid rate = %.2f, want 50", cr.Entries[0].ValidRate)
	}
}

func TestAvgSeverity(t *testing.T) {
	ff := []model.Finding{
		finding("h1", "m1", model.StateValidated, model.SevCritical, "c1"),
		finding("h1", "m1", model.StateValidated, model.SevLow, "c2"),
	}
	cr := Analyze(ff)
	want := 2.5
	if cr.Entries[0].AvgSev != want {
		t.Errorf("avg sev = %.2f, want %.2f", cr.Entries[0].AvgSev, want)
	}
}

func TestDuplicateDetection(t *testing.T) {
	ff := []model.Finding{
		finding("h1", "m1", model.StateValidated, model.SevHigh, "c1"),
		finding("h2", "m1", model.StateRefuted, model.SevHigh, "c1"),
	}
	cr := Analyze(ff)
	if len(cr.Duplicates) != 1 {
		t.Fatalf("duplicates = %d, want 1", len(cr.Duplicates))
	}
	if len(cr.Duplicates[0].Hashes) != 2 {
		t.Errorf("dup hashes = %d", len(cr.Duplicates[0].Hashes))
	}
}

func TestNoDuplicatesWhenDifferent(t *testing.T) {
	ff := []model.Finding{
		finding("h1", "m1", model.StateValidated, model.SevHigh, "c1"),
		finding("h2", "m2", model.StateValidated, model.SevHigh, "c2"),
	}
	cr := Analyze(ff)
	if len(cr.Duplicates) != 0 {
		t.Errorf("duplicates = %d, want 0", len(cr.Duplicates))
	}
}

func TestFormatTextNotEmpty(t *testing.T) {
	ff := []model.Finding{
		finding("h1", "m1", model.StateValidated, model.SevHigh, "c1"),
	}
	cr := Analyze(ff)
	text := FormatText(cr)
	if !strings.Contains(text, "Prompt Catalog") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "h1") {
		t.Error("missing hash")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	cr := Analyze(nil)
	text := FormatText(cr)
	if !strings.Contains(text, "No prompts found") {
		t.Error("missing empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	ff := []model.Finding{
		finding("h1", "m1", model.StateValidated, model.SevHigh, "c1"),
	}
	cr := Analyze(ff)
	out := FormatJSON(cr)
	var parsed CatalogReport
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestTotalCounts(t *testing.T) {
	ff := []model.Finding{
		finding("h1", "m1", model.StateValidated, model.SevHigh, "c1"),
		finding("h2", "m1", model.StateValidated, model.SevHigh, "c2"),
		finding("h2", "m1", model.StateRefuted, model.SevLow, "c3"),
	}
	cr := Analyze(ff)
	if cr.Total != 3 {
		t.Errorf("total = %d, want 3", cr.Total)
	}
	if cr.UniqueHash != 2 {
		t.Errorf("unique = %d, want 2", cr.UniqueHash)
	}
}

func TestEllipsis(t *testing.T) {
	if ellipsis("short", 10) != "short" {
		t.Error("short string changed")
	}
	long := strings.Repeat("a", 50)
	if e := ellipsis(long, 10); len(e) != 10 || !strings.HasSuffix(e, "...") {
		t.Errorf("ellipsis = %q", e)
	}
}

func TestSeverityNum(t *testing.T) {
	tests := []struct {
		sev  model.Severity
		want float64
	}{
		{model.SevCritical, 4.0},
		{model.SevHigh, 3.0},
		{model.SevMedium, 2.0},
		{model.SevLow, 1.0},
		{"unknown", 0.0},
	}
	for _, tt := range tests {
		if got := severityNum(tt.sev); got != tt.want {
			t.Errorf("severityNum(%s) = %.1f, want %.1f", tt.sev, got, tt.want)
		}
	}
}
