package promptmutate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestSplitSentences(t *testing.T) {
	s := splitSentences("Hello world. How are you? Fine!")
	if len(s) != 3 {
		t.Fatalf("expected 3 sentences, got %d: %v", len(s), s)
	}
}

func TestSplitSentencesNoTerminator(t *testing.T) {
	s := splitSentences("no punctuation here")
	if len(s) != 1 {
		t.Fatalf("expected 1 sentence, got %d", len(s))
	}
}

func TestHashPrompt(t *testing.T) {
	h1 := hashPrompt("hello")
	h2 := hashPrompt("hello")
	h3 := hashPrompt("world")
	if h1 != h2 {
		t.Fatal("same input should produce same hash")
	}
	if h1 == h3 {
		t.Fatal("different inputs should produce different hashes")
	}
}

func TestSeverityScore(t *testing.T) {
	score := severityScore(nil)
	if score != 0 {
		t.Fatalf("expected 0 for nil, got %.2f", score)
	}

	ff := []model.Finding{
		{Severity: model.SevHigh},
		{Severity: model.SevLow},
	}
	s := severityScore(ff)
	if s != 2.0 {
		t.Fatalf("expected 2.0, got %.2f", s)
	}
}

func TestFindingOverlapIdentical(t *testing.T) {
	ff := []model.Finding{
		{ID: "f1", DedupKey: "dk1"},
		{ID: "f2", DedupKey: "dk2"},
	}
	o := findingOverlap(ff, ff)
	if o != 1.0 {
		t.Fatalf("identical findings should have overlap 1.0, got %.2f", o)
	}
}

func TestFindingOverlapDisjoint(t *testing.T) {
	a := []model.Finding{{ID: "f1", DedupKey: "dk1"}}
	b := []model.Finding{{ID: "f2", DedupKey: "dk2"}}
	o := findingOverlap(a, b)
	if o != 0 {
		t.Fatalf("disjoint findings should have overlap 0, got %.2f", o)
	}
}

func TestFindingOverlapEmpty(t *testing.T) {
	o := findingOverlap(nil, nil)
	if o != 1.0 {
		t.Fatalf("both empty should be 1.0, got %.2f", o)
	}
}

func TestFindingOverlapOneEmpty(t *testing.T) {
	ff := []model.Finding{{ID: "f1"}}
	o := findingOverlap(ff, nil)
	if o != 0 {
		t.Fatalf("one empty should be 0, got %.2f", o)
	}
}

func TestDefaultMutations(t *testing.T) {
	prompt := "Check for security vulnerabilities. Look for exploit paths."
	for _, m := range DefaultMutations {
		result := m.Apply(prompt)
		if result == "" && prompt != "" {
			t.Fatalf("mutation %q produced empty output", m.Name)
		}
	}
}

func TestAnalyzeNoFindingsFunc(t *testing.T) {
	r := Analyze("test prompt", nil, DefaultMutations, nil, 0.5)
	if r.Summary.TotalMutations != len(DefaultMutations) {
		t.Fatalf("expected %d mutations, got %d", len(DefaultMutations), r.Summary.TotalMutations)
	}
	for _, m := range r.Mutations {
		if m.MutatedCount != 0 {
			t.Fatalf("expected 0 mutated findings with nil fn, got %d", m.MutatedCount)
		}
	}
}

func TestAnalyzeWithStableResults(t *testing.T) {
	origFindings := []model.Finding{
		{ID: "f1", DedupKey: "dk1", Severity: model.SevHigh},
		{ID: "f2", DedupKey: "dk2", Severity: model.SevMedium},
	}
	fn := func(_ string) []model.Finding {
		return origFindings
	}
	r := Analyze("test prompt", origFindings, DefaultMutations, fn, 0.5)
	if r.Summary.StableCount != len(DefaultMutations) {
		t.Fatalf("all should be stable, got %d/%d", r.Summary.StableCount, r.Summary.TotalMutations)
	}
	if r.Summary.RobustnessScore != 1.0 {
		t.Fatalf("expected robustness 1.0, got %.2f", r.Summary.RobustnessScore)
	}
}

func TestAnalyzeWithUnstableResults(t *testing.T) {
	origFindings := []model.Finding{
		{ID: "f1", DedupKey: "dk1", Severity: model.SevHigh},
	}
	fn := func(_ string) []model.Finding {
		return []model.Finding{
			{ID: "f99", DedupKey: "dk99", Severity: model.SevLow},
		}
	}
	r := Analyze("test prompt", origFindings, DefaultMutations, fn, 0.5)
	if r.Summary.UnstableCount != len(DefaultMutations) {
		t.Fatalf("all should be unstable, got %d unstable", r.Summary.UnstableCount)
	}
}

func TestAnalyzeDefaultThreshold(t *testing.T) {
	r := Analyze("test", nil, []Mutation{{Name: "noop", Apply: func(s string) string { return s }}}, nil, 0)
	if r.Summary.TotalMutations != 1 {
		t.Fatalf("expected 1, got %d", r.Summary.TotalMutations)
	}
}

func TestAnalyzeStatic(t *testing.T) {
	orig := []model.Finding{{ID: "f1", DedupKey: "dk1", Severity: model.SevHigh}}
	muts := []Mutation{
		{Name: "lower", Apply: strings.ToLower},
	}
	mutatedPrompt := strings.ToLower("Test Prompt")
	mutHash := hashPrompt(mutatedPrompt)

	results := map[string][]model.Finding{
		mutHash: {{ID: "f1", DedupKey: "dk1", Severity: model.SevHigh}},
	}
	r := AnalyzeStatic("Test Prompt", orig, results, muts, 0.5)
	if r.Summary.StableCount != 1 {
		t.Fatalf("expected 1 stable, got %d", r.Summary.StableCount)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze("Check for vulnerabilities in the code.", nil, DefaultMutations[:2], nil, 0.5)
	text := FormatText(r)
	if !strings.Contains(text, "Prompt Mutation Testing") {
		t.Fatal("missing header")
	}
	if !strings.Contains(text, "Robustness") {
		t.Fatal("missing robustness line")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze("test", nil, DefaultMutations[:1], nil, 0.5)
	out := FormatJSON(r)
	var data map[string]any
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["mutations"]; !ok {
		t.Fatal("missing mutations key")
	}
	if _, ok := data["summary"]; !ok {
		t.Fatal("missing summary key")
	}
}
