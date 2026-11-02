package reasoning

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id string, opts ...func(*model.Finding)) model.Finding {
	f := model.Finding{ID: id, Summary: "test finding " + id}
	for _, o := range opts {
		o(&f)
	}
	return f
}

func withEvidence(kinds ...string) func(*model.Finding) {
	return func(f *model.Finding) {
		for _, k := range kinds {
			f.Evidence = append(f.Evidence, model.Evidence{
				Kind:       k,
				StatusCode: 200,
				Marker:     "m-" + k,
			})
		}
	}
}

func withLocation(method, path, param, comp string) func(*model.Finding) {
	return func(f *model.Finding) {
		f.Location = model.Location{
			Method:       method,
			PathTemplate: path,
			Param:        param,
			Component:    comp,
		}
	}
}

func withSummary(s string) func(*model.Finding) {
	return func(f *model.Finding) { f.Summary = s }
}

var emptyRR model.RunReport

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(emptyRR, nil, 0.6)
	if r.Summary.TotalFindings != 0 {
		t.Fatalf("expected 0 findings, got %d", r.Summary.TotalFindings)
	}
	if len(r.Summary.GradeDistribution) != 0 {
		t.Error("expected empty grade distribution")
	}
}

func TestChainCompletenessMultiKind(t *testing.T) {
	f := finding("f1", withEvidence("http-exchange", "marker-reflected", "header-missing"))
	s := scoreChainCompleteness(f)
	if s != 1.0 {
		t.Errorf("3 distinct kinds should score 1.0, got %.2f", s)
	}
}

func TestChainCompletenessNoEvidence(t *testing.T) {
	f := finding("f1")
	s := scoreChainCompleteness(f)
	if s != 0.1 {
		t.Errorf("no evidence should score 0.1, got %.2f", s)
	}
}

func TestChainCompletenessSingleKind(t *testing.T) {
	f := finding("f1", withEvidence("http-exchange"))
	s := scoreChainCompleteness(f)
	if s != 0.4 {
		t.Errorf("1 kind should score 0.4, got %.2f", s)
	}
}

func TestChainCompletenessTwoKinds(t *testing.T) {
	f := finding("f1", withEvidence("http-exchange", "marker-reflected"))
	s := scoreChainCompleteness(f)
	if s != 0.7 {
		t.Errorf("2 kinds should score 0.7, got %.2f", s)
	}
}

func TestSpecificityFull(t *testing.T) {
	f := finding("f1",
		withLocation("POST", "/api/login", "password", "auth-service"),
		withEvidence("http-exchange"),
	)
	s := scoreSpecificity(f)
	if s < 0.7 {
		t.Errorf("full location + evidence should score >= 0.7, got %.2f", s)
	}
}

func TestSpecificityNone(t *testing.T) {
	f := finding("f1")
	s := scoreSpecificity(f)
	if s != 0.1 {
		t.Errorf("no location or evidence should score 0.1, got %.2f", s)
	}
}

func TestCausalClarityMultiple(t *testing.T) {
	f := finding("f1", withSummary("Missing auth allows unauthorized access because the endpoint enables direct resource retrieval"))
	s := scoreCausalClarity(f)
	if s != 1.0 {
		t.Errorf("2+ causal terms should score 1.0, got %.2f", s)
	}
}

func TestCausalClaritySingle(t *testing.T) {
	f := finding("f1", withSummary("This vulnerability allows remote code execution"))
	s := scoreCausalClarity(f)
	if s != 0.5 {
		t.Errorf("1 causal term should score 0.5, got %.2f", s)
	}
}

func TestCausalClarityNone(t *testing.T) {
	f := finding("f1", withSummary("The parameter is vulnerable"))
	s := scoreCausalClarity(f)
	if s != 0.1 {
		t.Errorf("no causal terms should score 0.1, got %.2f", s)
	}
}

func TestCounterargumentPresent(t *testing.T) {
	f := finding("f1", withSummary("This may be exploitable, although mitigation exists in some cases"))
	s := scoreCounterargument(f)
	if s != 0.8 {
		t.Errorf("hedging present should score 0.8, got %.2f", s)
	}
}

func TestCounterargumentAbsent(t *testing.T) {
	f := finding("f1", withSummary("The endpoint is definitively vulnerable to attack"))
	s := scoreCounterargument(f)
	if s != 0.3 {
		t.Errorf("no hedging should score 0.3, got %.2f", s)
	}
}

func TestActionabilityMultiple(t *testing.T) {
	f := finding("f1", withSummary("Fix the query by using parameterized statements and validate all user input"))
	s := scoreActionability(f)
	if s != 1.0 {
		t.Errorf("2+ action terms should score 1.0, got %.2f", s)
	}
}

func TestActionabilitySingle(t *testing.T) {
	f := finding("f1", withSummary("Sanitize the user input before processing"))
	s := scoreActionability(f)
	if s != 0.6 {
		t.Errorf("1 action term should score 0.6, got %.2f", s)
	}
}

func TestActionabilityNone(t *testing.T) {
	f := finding("f1", withSummary("The system is vulnerable"))
	s := scoreActionability(f)
	if s != 0.2 {
		t.Errorf("no action terms should score 0.2, got %.2f", s)
	}
}

func TestGradeFromQuality(t *testing.T) {
	tests := []struct {
		quality float64
		grade   string
	}{
		{0.9, "A"},
		{0.8, "A"},
		{0.7, "B"},
		{0.65, "B"},
		{0.55, "C"},
		{0.5, "C"},
		{0.4, "D"},
		{0.35, "D"},
		{0.2, "F"},
		{0.0, "F"},
	}
	for _, tt := range tests {
		got := gradeFromQuality(tt.quality)
		if got != tt.grade {
			t.Errorf("gradeFromQuality(%.2f) = %q, want %q", tt.quality, got, tt.grade)
		}
	}
}

func TestAnalyzeHighQuality(t *testing.T) {
	ff := []model.Finding{
		finding("good",
			withEvidence("http-exchange", "marker-reflected", "header-missing"),
			withLocation("POST", "/api/login", "password", "auth"),
			withSummary("Missing authentication allows unauthorized access because the endpoint enables direct data retrieval; fix by adding auth middleware and validate tokens"),
		),
	}

	r := Analyze(emptyRR, ff, 0.6)
	if r.Summary.TotalFindings != 1 {
		t.Fatalf("expected 1 finding, got %d", r.Summary.TotalFindings)
	}
	sc := r.Scores[0]
	if sc.Grade == "F" || sc.Grade == "D" {
		t.Errorf("high-quality finding should not get grade %s (quality: %.2f)", sc.Grade, sc.OverallQuality)
	}
}

func TestAnalyzeLowQuality(t *testing.T) {
	ff := []model.Finding{
		finding("weak", withSummary("Vulnerable")),
	}

	r := Analyze(emptyRR, ff, 0.6)
	sc := r.Scores[0]
	if sc.Grade == "A" || sc.Grade == "B" {
		t.Errorf("low-quality finding should not get grade %s (quality: %.2f)", sc.Grade, sc.OverallQuality)
	}
}

func TestPassRate(t *testing.T) {
	ff := []model.Finding{
		finding("good",
			withEvidence("http-exchange", "marker-reflected", "header-missing"),
			withLocation("POST", "/api/login", "pw", "auth"),
			withSummary("Allows data access because the auth is missing; fix by adding middleware and validate input"),
		),
		finding("weak", withSummary("Vulnerable")),
	}

	r := Analyze(emptyRR, ff, 0.5)
	if r.Summary.PassRate < 0.0 || r.Summary.PassRate > 1.0 {
		t.Errorf("pass rate out of range: %.2f", r.Summary.PassRate)
	}
}

func TestSummaryIndicators(t *testing.T) {
	ff := []model.Finding{
		finding("f1",
			withEvidence("http-exchange"),
			withLocation("GET", "/api", "", ""),
			withSummary("Allows access because auth is missing"),
		),
	}
	r := Analyze(emptyRR, ff, 0.6)
	for _, key := range []string{"chain_completeness", "specificity", "causal_clarity", "counterargument_awareness", "actionability"} {
		if _, ok := r.Summary.ByIndicator[key]; !ok {
			t.Errorf("missing indicator %s", key)
		}
	}
}

func TestGradeDistribution(t *testing.T) {
	ff := []model.Finding{
		finding("f1", withSummary("Vulnerable")),
		finding("f2", withSummary("Also vulnerable")),
	}
	r := Analyze(emptyRR, ff, 0.6)
	total := 0
	for _, count := range r.Summary.GradeDistribution {
		total += count
	}
	if total != 2 {
		t.Errorf("grade distribution total = %d, want 2", total)
	}
}

func TestFormatTextOutput(t *testing.T) {
	ff := []model.Finding{
		finding("f1",
			withEvidence("http-exchange"),
			withSummary("Allows access because auth missing"),
		),
	}
	r := Analyze(emptyRR, ff, 0.6)
	text := FormatText(r)
	if text == "" {
		t.Fatal("empty text output")
	}
	for _, want := range []string{"Reasoning Quality", "f1", "Chain:", "Total findings:"} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q", want)
		}
	}
}

func TestFormatJSONRoundTrip(t *testing.T) {
	ff := []model.Finding{
		finding("f1", withEvidence("http-exchange")),
	}
	r := Analyze(emptyRR, ff, 0.6)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var r2 Report
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatal(err)
	}
	if r2.Summary.TotalFindings != 1 {
		t.Errorf("expected 1 finding after round-trip, got %d", r2.Summary.TotalFindings)
	}
}

func TestMinQuality(t *testing.T) {
	ff := []model.Finding{
		finding("good",
			withEvidence("http-exchange", "marker-reflected", "header-missing"),
			withLocation("POST", "/login", "pw", "auth"),
			withSummary("Allows access because auth missing; fix by adding middleware and validate"),
		),
		finding("weak", withSummary("Bad")),
	}
	r := Analyze(emptyRR, ff, 0.6)
	if r.Summary.MinQuality >= r.Summary.MeanQuality {
		t.Errorf("min (%.2f) should be < mean (%.2f)", r.Summary.MinQuality, r.Summary.MeanQuality)
	}
}
