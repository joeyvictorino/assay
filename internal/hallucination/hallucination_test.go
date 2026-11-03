package hallucination

import (
	"encoding/json"
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
				Kind:          k,
				StatusCode:    200,
				Marker:        "marker-" + k,
				RequestDigest: "digest",
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

func withSeverity(s model.Severity) func(*model.Finding) {
	return func(f *model.Finding) { f.Severity = s }
}

func withClass(c model.Class) func(*model.Finding) {
	return func(f *model.Finding) { f.Class = c }
}

func withSummary(s string) func(*model.Finding) {
	return func(f *model.Finding) { f.Summary = s }
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil, 0.5)
	if r.Summary.TotalFindings != 0 {
		t.Fatalf("expected 0 findings, got %d", r.Summary.TotalFindings)
	}
}

func TestEvidenceQualityEmpty(t *testing.T) {
	f := finding("f1")
	s := scoreEvidence(f)
	if s != 1.0 {
		t.Errorf("empty evidence should score 1.0, got %.2f", s)
	}
}

func TestEvidenceQualityConcrete(t *testing.T) {
	f := finding("f1", withEvidence("http-exchange", "marker-reflected", "header-missing"))
	s := scoreEvidence(f)
	if s != 0.0 {
		t.Errorf("3 concrete evidence items should score 0.0, got %.2f", s)
	}
}

func TestEvidenceQualityPartial(t *testing.T) {
	f := finding("f1", withEvidence("http-exchange"))
	s := scoreEvidence(f)
	if s != 0.5 {
		t.Errorf("1 concrete evidence item should score 0.5, got %.2f", s)
	}
}

func TestEvidenceQualityNoConcreteData(t *testing.T) {
	f := finding("f1")
	f.Evidence = []model.Evidence{{Kind: "tool-output"}}
	s := scoreEvidence(f)
	if s != 0.8 {
		t.Errorf("evidence without concrete data should score 0.8, got %.2f", s)
	}
}

func TestLocationFullySpecified(t *testing.T) {
	f := finding("f1", withLocation("GET", "/api/users/{id}", "id", "api-gateway"))
	s := scoreLocation(f)
	if s != 0.0 {
		t.Errorf("fully specified location should score 0.0, got %.2f", s)
	}
}

func TestLocationEmpty(t *testing.T) {
	f := finding("f1")
	s := scoreLocation(f)
	if s != 1.0 {
		t.Errorf("empty location should score 1.0, got %.2f", s)
	}
}

func TestLocationPartial(t *testing.T) {
	f := finding("f1", withLocation("GET", "/api/users", "", ""))
	s := scoreLocation(f)
	if s != 0.3 {
		t.Errorf("2-field location should score 0.3, got %.2f", s)
	}
}

func TestSeverityCalibrationMatched(t *testing.T) {
	f := finding("f1",
		withSeverity(model.SevCritical),
		withSummary("Remote code execution via SQL injection in login endpoint"),
	)
	s := scoreSeverity(f)
	if s != 0.0 {
		t.Errorf("matched severity should score 0.0, got %.2f", s)
	}
}

func TestSeverityCalibrationMismatch(t *testing.T) {
	f := finding("f1",
		withSeverity(model.SevCritical),
		withSummary("Minor cosmetic style issue in footer"),
	)
	s := scoreSeverity(f)
	if s < 0.5 {
		t.Errorf("severity mismatch should score >= 0.5, got %.2f", s)
	}
}

func TestClassConsistencyMatch(t *testing.T) {
	f := finding("f1",
		withClass(model.ClassSQLi),
		withSummary("SQL injection in the query parameter allows database access"),
	)
	s := scoreClass(f)
	if s > 0.2 {
		t.Errorf("matching class should score <= 0.2, got %.2f", s)
	}
}

func TestClassConsistencyMismatch(t *testing.T) {
	f := finding("f1",
		withClass(model.ClassSQLi),
		withSummary("The redirect endpoint does not validate the target URL"),
	)
	s := scoreClass(f)
	if s < 0.5 {
		t.Errorf("mismatched class should score >= 0.5, got %.2f", s)
	}
}

func TestClassEmpty(t *testing.T) {
	f := finding("f1")
	s := scoreClass(f)
	if s != 0.5 {
		t.Errorf("empty class should score 0.5, got %.2f", s)
	}
}

func TestDuplicatePatternNone(t *testing.T) {
	summaries := []string{"first finding about sql injection", "second finding about xss", "third about auth bypass"}
	s := scoreDuplicate(summaries[0], summaries, 0)
	if s != 0.0 {
		t.Errorf("no duplicates should score 0.0, got %.2f", s)
	}
}

func TestDuplicatePatternHigh(t *testing.T) {
	summaries := []string{
		"SQL injection in user login endpoint allows data access",
		"SQL injection in user login endpoint allows data extraction",
	}
	s := scoreDuplicate(summaries[0], summaries, 0)
	if s < 0.5 {
		t.Errorf("high similarity should score >= 0.5, got %.2f", s)
	}
}

func TestDuplicatePatternEmptySummary(t *testing.T) {
	s := scoreDuplicate("", []string{"", "test"}, 0)
	if s != 0.0 {
		t.Errorf("empty summary should score 0.0, got %.2f", s)
	}
}

func TestAnalyzeFlagging(t *testing.T) {
	ff := []model.Finding{
		finding("good",
			withEvidence("http-exchange", "marker-reflected", "header-missing"),
			withLocation("GET", "/api/users/{id}", "id", "api"),
			withSeverity(model.SevHigh),
			withClass(model.ClassSQLi),
			withSummary("SQL injection in the query parameter allows database access"),
		),
		finding("suspect",
			withSeverity(model.SevCritical),
			withSummary("Minor cosmetic style issue in footer"),
		),
	}

	r := Analyze(ff, 0.5)
	if r.Summary.TotalFindings != 2 {
		t.Fatalf("expected 2 findings, got %d", r.Summary.TotalFindings)
	}

	var good, suspect *Score
	for i := range r.Scores {
		switch r.Scores[i].FindingID {
		case "good":
			good = &r.Scores[i]
		case "suspect":
			suspect = &r.Scores[i]
		}
	}

	if good == nil || suspect == nil {
		t.Fatal("missing scores")
	}

	if good.Flagged {
		t.Error("good finding should not be flagged")
	}
	if !suspect.Flagged {
		t.Error("suspect finding should be flagged")
	}
	if suspect.OverallScore <= good.OverallScore {
		t.Errorf("suspect (%.2f) should score higher than good (%.2f)", suspect.OverallScore, good.OverallScore)
	}
}

func TestAnalyzeSummaryIndicators(t *testing.T) {
	ff := []model.Finding{
		finding("f1",
			withEvidence("http-exchange"),
			withLocation("POST", "/login", "user", ""),
		),
	}
	r := Analyze(ff, 0.5)
	for _, key := range []string{"evidence_quality", "location_specificity", "severity_calibration", "class_consistency", "duplicate_pattern"} {
		if _, ok := r.Summary.ByIndicator[key]; !ok {
			t.Errorf("missing indicator %s", key)
		}
	}
}

func TestJaccardSimilarity(t *testing.T) {
	s := jaccardSimilarity("the cat sat on the mat", "the cat sat on the mat")
	if s != 1.0 {
		t.Errorf("identical strings should have similarity 1.0, got %.2f", s)
	}
	s = jaccardSimilarity("apple banana cherry", "dog elephant fox")
	if s != 0.0 {
		t.Errorf("disjoint sets should have similarity 0.0, got %.2f", s)
	}
}

func TestFormatTextOutput(t *testing.T) {
	ff := []model.Finding{
		finding("f1", withEvidence("http-exchange"), withSeverity(model.SevMedium)),
	}
	r := Analyze(ff, 0.5)
	text := FormatText(r)
	if text == "" {
		t.Fatal("empty text output")
	}
	for _, want := range []string{"Hallucination Analysis", "f1", "Evidence:", "Total findings:"} {
		if !contains(text, want) {
			t.Errorf("text output missing %q", want)
		}
	}
}

func TestFormatJSONRoundTrip(t *testing.T) {
	ff := []model.Finding{
		finding("f1", withEvidence("http-exchange")),
	}
	r := Analyze(ff, 0.5)
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

func TestThresholdZeroFlagsAll(t *testing.T) {
	ff := []model.Finding{finding("f1"), finding("f2")}
	r := Analyze(ff, 0.0)
	if r.Summary.FlaggedCount != 2 {
		t.Errorf("threshold 0.0 should flag all, got %d flagged", r.Summary.FlaggedCount)
	}
}

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		sev  string
		want int
	}{
		{"critical", 5},
		{"high", 4},
		{"medium", 3},
		{"low", 2},
		{"info", 1},
		{"unknown", 3},
	}
	for _, tt := range tests {
		got := severityRank(tt.sev)
		if got != tt.want {
			t.Errorf("severityRank(%q) = %d, want %d", tt.sev, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && containsString(s, substr))
}

func containsString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
