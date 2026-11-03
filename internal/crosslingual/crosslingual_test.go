package crosslingual

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func f(id, dedup string, sev model.Severity) model.Finding {
	return model.Finding{ID: id, DedupKey: dedup, Severity: sev, State: model.StateValidated}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalLanguages != 0 {
		t.Fatalf("expected 0 languages, got %d", r.TotalLanguages)
	}
}

func TestAnalyzeSingleLanguage(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(byLang)
	if r.TotalLanguages != 1 {
		t.Fatalf("expected 1 language, got %d", r.TotalLanguages)
	}
	if r.UniversalCount != 1 {
		t.Errorf("single lang: expected 1 universal, got %d", r.UniversalCount)
	}
	if r.ConsistencyPct != 100 {
		t.Errorf("expected 100%% consistency, got %.1f%%", r.ConsistencyPct)
	}
}

func TestAnalyzeIdenticalFindings(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevLow)},
		"fr": {f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevLow)},
	}
	r := Analyze(byLang)
	if r.UniversalCount != 2 {
		t.Errorf("expected 2 universal, got %d", r.UniversalCount)
	}
	if r.UniqueCount != 0 {
		t.Errorf("expected 0 unique, got %d", r.UniqueCount)
	}
	if r.ConsistencyPct != 100 {
		t.Errorf("expected 100%% consistency, got %.1f%%", r.ConsistencyPct)
	}
	if r.SeverityAgree != 100 {
		t.Errorf("expected 100%% severity agreement, got %.1f%%", r.SeverityAgree)
	}
}

func TestAnalyzeDisjointFindings(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh)},
		"de": {f("f2", "d2", model.SevLow)},
	}
	r := Analyze(byLang)
	if r.UniversalCount != 0 {
		t.Errorf("expected 0 universal, got %d", r.UniversalCount)
	}
	if r.UniqueCount != 2 {
		t.Errorf("expected 2 unique, got %d", r.UniqueCount)
	}
	if r.ConsistencyPct != 0 {
		t.Errorf("expected 0%% consistency, got %.1f%%", r.ConsistencyPct)
	}
}

func TestAnalyzePartialOverlap(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevMedium)},
		"fr": {f("f1", "d1", model.SevHigh), f("f3", "d3", model.SevLow)},
		"ja": {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(byLang)
	if r.UniversalCount != 1 {
		t.Errorf("expected 1 universal (d1), got %d", r.UniversalCount)
	}
	if r.UniqueCount != 2 {
		t.Errorf("expected 2 unique (d2, d3), got %d", r.UniqueCount)
	}
}

func TestAnalyzeSeverityDisagreement(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh)},
		"fr": {f("f1", "d1", model.SevLow)},
	}
	r := Analyze(byLang)
	if r.SeverityAgree != 0 {
		t.Errorf("expected 0%% severity agreement, got %.1f%%", r.SeverityAgree)
	}
	if r.Matches[0].Consistent {
		t.Error("expected inconsistent finding")
	}
}

func TestAnalyzeSeverityMixedAgreement(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevLow)},
		"fr": {f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevMedium)},
	}
	r := Analyze(byLang)
	if r.SeverityAgree != 50 {
		t.Errorf("expected 50%% severity agreement, got %.1f%%", r.SeverityAgree)
	}
}

func TestAnalyzeCoverageOrdering(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevLow)},
		"fr": {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(byLang)
	for i := 1; i < len(r.Matches); i++ {
		if r.Matches[i].Coverage > r.Matches[i-1].Coverage {
			t.Errorf("matches not sorted by coverage: %v > %v",
				r.Matches[i].Coverage, r.Matches[i-1].Coverage)
		}
	}
}

func TestAnalyzeDedupKeyFallback(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {{ID: "f1", Severity: model.SevHigh}},
		"fr": {{ID: "f1", Severity: model.SevHigh}},
	}
	r := Analyze(byLang)
	if r.UniversalCount != 1 {
		t.Errorf("expected 1 universal with ID fallback, got %d", r.UniversalCount)
	}
}

func TestAnalyzeLanguageStats(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevLow)},
		"fr": {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(byLang)
	if len(r.Languages) != 2 {
		t.Fatalf("expected 2 lang stats, got %d", len(r.Languages))
	}
	found := false
	for _, ls := range r.Languages {
		if ls.Language == "en" && ls.Unique == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected en to have 1 unique finding")
	}
}

func TestAnalyzeTotalFindings(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh)},
		"fr": {f("f2", "d2", model.SevLow), f("f3", "d3", model.SevMedium)},
	}
	r := Analyze(byLang)
	if r.TotalFindings != 3 {
		t.Errorf("expected 3 total findings, got %d", r.TotalFindings)
	}
}

func TestFormatTextOutput(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh)},
		"fr": {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(byLang)
	text := FormatText(r)
	if !strings.Contains(text, "Cross-Lingual") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "en:") {
		t.Error("missing language name")
	}
	if !strings.Contains(text, "Universal") {
		t.Error("missing universal line")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No cross-lingual data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(byLang)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.TotalLanguages != 1 {
		t.Errorf("expected 1 language, got %d", parsed.TotalLanguages)
	}
}

func TestFiveLanguages(t *testing.T) {
	byLang := map[string][]model.Finding{
		"en": {f("f1", "d1", model.SevHigh)},
		"fr": {f("f1", "d1", model.SevHigh)},
		"de": {f("f1", "d1", model.SevHigh)},
		"ja": {f("f1", "d1", model.SevHigh)},
		"zh": {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(byLang)
	if r.TotalLanguages != 5 {
		t.Errorf("expected 5 languages, got %d", r.TotalLanguages)
	}
	if r.Matches[0].Coverage != 100 {
		t.Errorf("expected 100%% coverage, got %.1f%%", r.Matches[0].Coverage)
	}
}
