package refusal

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, prompt, summary string, state model.FindingState, class model.Class) model.Finding {
	return model.Finding{
		Model:      model.ModelRef{Model: m},
		PromptHash: prompt,
		Summary:    summary,
		State:      state,
		Class:      class,
	}
}

func TestIsRefusal_Declined(t *testing.T) {
	f := finding("gpt-4", "p1", "some text", model.StateDeclined, model.ClassSQLi)
	if !IsRefusal(f) {
		t.Fatal("expected declined state to be a refusal")
	}
}

func TestIsRefusal_EmptySummary(t *testing.T) {
	f := finding("gpt-4", "p1", "", model.StateTheorized, model.ClassSQLi)
	if !IsRefusal(f) {
		t.Fatal("expected empty summary to be a refusal")
	}
}

func TestIsRefusal_PatternMatch(t *testing.T) {
	cases := []string{
		"I cannot assist with that request",
		"I'm sorry, I can't help with this",
		"As an AI, I don't have access",
		"I'm not able to perform this action",
		"I don't have the ability to do that",
	}
	for _, s := range cases {
		f := finding("m1", "p1", s, model.StateTheorized, model.ClassSQLi)
		if !IsRefusal(f) {
			t.Errorf("expected %q to be a refusal", s)
		}
	}
}

func TestIsRefusal_NotRefusal(t *testing.T) {
	f := finding("m1", "p1", "Found SQL injection in /api/users", model.StateValidated, model.ClassSQLi)
	if IsRefusal(f) {
		t.Fatal("expected normal finding not to be a refusal")
	}
}

func TestAnalyze_Empty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalFindings != 0 || r.TotalRefusals != 0 {
		t.Fatal("expected zeros for empty input")
	}
	if r.OverallRate != 0 {
		t.Fatal("expected 0 rate")
	}
}

func TestAnalyze_NoRefusals(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "p1", "Found XSS", model.StateValidated, model.ClassXSSReflected),
		finding("m2", "p1", "Found SQLI", model.StateValidated, model.ClassSQLi),
	}
	r := Analyze(ff)
	if r.TotalRefusals != 0 {
		t.Fatalf("expected 0 refusals, got %d", r.TotalRefusals)
	}
	if r.OverallRate != 0 {
		t.Fatal("expected 0 rate")
	}
}

func TestAnalyze_AllRefusals(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "p1", "I cannot do this", model.StateTheorized, model.ClassSQLi),
		finding("m1", "p2", "", model.StateTheorized, model.ClassXSSReflected),
	}
	r := Analyze(ff)
	if r.TotalRefusals != 2 {
		t.Fatalf("expected 2 refusals, got %d", r.TotalRefusals)
	}
	if r.OverallRate != 1.0 {
		t.Fatalf("expected 1.0 rate, got %f", r.OverallRate)
	}
}

func TestAnalyze_ByModel(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "p1", "I cannot do this", model.StateTheorized, model.ClassSQLi),
		finding("m1", "p2", "Found XSS", model.StateValidated, model.ClassXSSReflected),
		finding("m2", "p1", "Found SQLI", model.StateValidated, model.ClassSQLi),
		finding("m2", "p2", "Found IDOR", model.StateValidated, model.ClassIDOR),
	}
	r := Analyze(ff)
	if len(r.ByModel) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.ByModel))
	}
	// m1 has higher rate (0.5) so should come first
	if r.ByModel[0].Model != "m1" {
		t.Fatalf("expected m1 first (higher rate), got %s", r.ByModel[0].Model)
	}
	if r.ByModel[0].Refusals != 1 || r.ByModel[0].Total != 2 {
		t.Fatal("wrong m1 counts")
	}
	if r.ByModel[1].Model != "m2" {
		t.Fatalf("expected m2 second, got %s", r.ByModel[1].Model)
	}
	if r.ByModel[1].Refusals != 0 {
		t.Fatal("expected m2 to have 0 refusals")
	}
}

func TestAnalyze_ByPrompt(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "p1", "I cannot", model.StateTheorized, model.ClassSQLi),
		finding("m2", "p1", "I cannot", model.StateTheorized, model.ClassSQLi),
		finding("m1", "p2", "Found XSS", model.StateValidated, model.ClassXSSReflected),
	}
	r := Analyze(ff)
	// p1 should be always-refused
	if len(r.AlwaysRefused) != 1 || r.AlwaysRefused[0] != "p1" {
		t.Fatalf("expected p1 always refused, got %v", r.AlwaysRefused)
	}
	// p1 breakdown should list both models
	for _, pb := range r.ByPrompt {
		if pb.PromptHash == "p1" {
			if !pb.AlwaysRefused {
				t.Fatal("expected p1 always_refused=true")
			}
			if len(pb.RefusingModels) != 2 {
				t.Fatalf("expected 2 refusing models, got %d", len(pb.RefusingModels))
			}
		}
	}
}

func TestAnalyze_ByChecker(t *testing.T) {
	f1 := finding("m1", "p1", "I cannot", model.StateTheorized, model.ClassSQLi)
	f2 := finding("m1", "p2", "Found it", model.StateValidated, model.ClassSQLi)
	r := Analyze([]model.Finding{f1, f2})
	if len(r.ByChecker) != 1 {
		t.Fatalf("expected 1 checker, got %d", len(r.ByChecker))
	}
	if r.ByChecker[0].Refusals != 1 || r.ByChecker[0].Total != 2 {
		t.Fatal("wrong checker counts")
	}
}

func TestAnalyze_DeclinedState(t *testing.T) {
	f := finding("m1", "p1", "Some text that looks normal", model.StateDeclined, model.ClassSQLi)
	r := Analyze([]model.Finding{f})
	if r.TotalRefusals != 1 {
		t.Fatal("expected declined state to count as refusal")
	}
}

func TestAnalyze_TopPatterns(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "p1", "I cannot do A", model.StateTheorized, model.ClassSQLi),
		finding("m1", "p2", "I cannot do B", model.StateTheorized, model.ClassSQLi),
		finding("m1", "p3", "", model.StateTheorized, model.ClassSQLi),
	}
	r := Analyze(ff)
	if len(r.ByModel) != 1 {
		t.Fatal("expected 1 model")
	}
	if len(r.ByModel[0].TopPatterns) == 0 {
		t.Fatal("expected top patterns")
	}
}

func TestAnalyze_NoPromptHash(t *testing.T) {
	f := finding("m1", "", "I cannot", model.StateTheorized, model.ClassSQLi)
	r := Analyze([]model.Finding{f})
	if len(r.ByPrompt) != 1 {
		t.Fatal("expected 1 prompt entry")
	}
	if r.ByPrompt[0].PromptHash != "(no-hash)" {
		t.Fatalf("expected (no-hash), got %s", r.ByPrompt[0].PromptHash)
	}
}

func TestFormatText(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "p1", "I cannot do this", model.StateTheorized, model.ClassSQLi),
		finding("m2", "p1", "Found it", model.StateValidated, model.ClassSQLi),
	}
	r := Analyze(ff)
	text := FormatText(r)
	if !strings.Contains(text, "Refusal Rate") {
		t.Fatal("expected header in text output")
	}
	if !strings.Contains(text, "50.0%") {
		t.Fatal("expected 50% rate in output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if data["total_findings"].(float64) != 0 {
		t.Fatal("expected 0 total_findings")
	}
}

func TestAnalyze_MixedRefusalPatterns(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "p1", "I'm sorry, I can't help", model.StateTheorized, model.ClassSQLi),
		finding("m1", "p2", "As an AI, I cannot", model.StateTheorized, model.ClassSQLi),
		finding("m1", "p3", "Normal finding", model.StateValidated, model.ClassSQLi),
	}
	r := Analyze(ff)
	if r.TotalRefusals != 2 {
		t.Fatalf("expected 2 refusals, got %d", r.TotalRefusals)
	}
}

func TestAnalyze_ValidationChecker(t *testing.T) {
	f := model.Finding{
		Model:      model.ModelRef{Model: "m1"},
		PromptHash: "p1",
		Summary:    "I cannot",
		State:      model.StateTheorized,
		Class:      model.ClassSQLi,
		Validation: &model.ValidationResult{Checker: "my-checker"},
	}
	r := Analyze([]model.Finding{f})
	if len(r.ByChecker) != 1 || r.ByChecker[0].Checker != "my-checker" {
		t.Fatal("expected validation checker name")
	}
}
