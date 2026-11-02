package diversity

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(id string, class model.Class, sev model.Severity, mdl string, method, path string) model.Finding {
	return model.Finding{
		ID:       id,
		DedupKey: id,
		Class:    class,
		Severity: sev,
		Model:    model.ModelRef{Model: mdl},
		Location: model.Location{Method: method, PathTemplate: path},
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if r.TotalFindings != 0 {
		t.Errorf("expected 0, got %d", r.TotalFindings)
	}
	if r.Metrics.OverallScore != 0 {
		t.Errorf("expected 0 score, got %.2f", r.Metrics.OverallScore)
	}
}

func TestSingleFinding(t *testing.T) {
	f := mkFinding("f1", model.ClassSQLi, model.SevHigh, "gpt-4", "GET", "/api/users")
	r := Analyze([]model.Finding{f})
	if r.TotalFindings != 1 {
		t.Errorf("expected 1, got %d", r.TotalFindings)
	}
	if r.UniqueClasses != 1 {
		t.Errorf("expected 1 class, got %d", r.UniqueClasses)
	}
	if r.Metrics.ClassEntropy != 0 {
		t.Errorf("single class should have 0 entropy, got %.2f", r.Metrics.ClassEntropy)
	}
}

func TestMultipleClasses(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", model.ClassSQLi, model.SevHigh, "gpt-4", "GET", "/api/users"),
		mkFinding("f2", model.ClassXSSReflected, model.SevMedium, "gpt-4", "POST", "/api/search"),
		mkFinding("f3", model.ClassIDOR, model.SevHigh, "gpt-4", "GET", "/api/orders"),
	}
	r := Analyze(findings)
	if r.UniqueClasses != 3 {
		t.Errorf("expected 3 classes, got %d", r.UniqueClasses)
	}
	if r.Metrics.ClassEntropy <= 0 {
		t.Error("expected positive class entropy for diverse classes")
	}
}

func TestSeveritySpread(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", model.ClassSQLi, model.SevInfo, "m1", "GET", "/a"),
		mkFinding("f2", model.ClassSQLi, model.SevLow, "m1", "GET", "/b"),
		mkFinding("f3", model.ClassSQLi, model.SevMedium, "m1", "GET", "/c"),
		mkFinding("f4", model.ClassSQLi, model.SevHigh, "m1", "GET", "/d"),
		mkFinding("f5", model.ClassSQLi, model.SevCritical, "m1", "GET", "/e"),
	}
	r := Analyze(findings)
	if r.Metrics.SeveritySpread != 1.0 {
		t.Errorf("all 5 severities should give spread 1.0, got %.2f", r.Metrics.SeveritySpread)
	}
}

func TestModelAgreement(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", model.ClassSQLi, model.SevHigh, "gpt-4", "GET", "/a"),
		mkFinding("f1", model.ClassSQLi, model.SevHigh, "claude-3", "GET", "/a"),
	}
	r := Analyze(findings)
	if r.Metrics.ModelAgreement != 1.0 {
		t.Errorf("expected 1.0 agreement, got %.2f", r.Metrics.ModelAgreement)
	}
}

func TestNoModelAgreement(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", model.ClassSQLi, model.SevHigh, "gpt-4", "GET", "/a"),
		mkFinding("f2", model.ClassXSSReflected, model.SevMedium, "claude-3", "POST", "/b"),
	}
	r := Analyze(findings)
	if r.Metrics.ModelAgreement != 0 {
		t.Errorf("expected 0 agreement for unique findings, got %.2f", r.Metrics.ModelAgreement)
	}
}

func TestClassDistribution(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", model.ClassSQLi, model.SevHigh, "m1", "GET", "/a"),
		mkFinding("f2", model.ClassSQLi, model.SevHigh, "m1", "GET", "/b"),
		mkFinding("f3", model.ClassXSSReflected, model.SevMedium, "m1", "POST", "/c"),
	}
	r := Analyze(findings)
	if r.ClassDist["sqli"] != 2 {
		t.Errorf("expected 2 sqli, got %d", r.ClassDist["sqli"])
	}
	if r.ClassDist["xss-reflected"] != 1 {
		t.Errorf("expected 1 xss-reflected, got %d", r.ClassDist["xss-reflected"])
	}
}

func TestModelDistribution(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", model.ClassSQLi, model.SevHigh, "gpt-4", "GET", "/a"),
		mkFinding("f2", model.ClassSQLi, model.SevHigh, "claude-3", "GET", "/b"),
		mkFinding("f3", model.ClassSQLi, model.SevHigh, "gpt-4", "POST", "/c"),
	}
	r := Analyze(findings)
	if r.ModelDist["gpt-4"] != 2 {
		t.Errorf("expected 2 gpt-4, got %d", r.ModelDist["gpt-4"])
	}
	if r.ModelDist["claude-3"] != 1 {
		t.Errorf("expected 1 claude-3, got %d", r.ModelDist["claude-3"])
	}
}

func TestNormalizedEntropy(t *testing.T) {
	dist := map[string]int{"a": 5, "b": 5}
	e := normalizedEntropy(dist, 10)
	if e != 1.0 {
		t.Errorf("uniform distribution should have entropy 1.0, got %.2f", e)
	}

	dist2 := map[string]int{"a": 10}
	e2 := normalizedEntropy(dist2, 10)
	if e2 != 0 {
		t.Errorf("single category should have entropy 0, got %.2f", e2)
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", model.ClassSQLi, model.SevHigh, "gpt-4", "GET", "/api/users"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Diversity Report") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "Total findings") {
		t.Error("expected total findings line")
	}
}

func TestUniqueLocations(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", model.ClassSQLi, model.SevHigh, "m1", "GET", "/api/users"),
		mkFinding("f2", model.ClassSQLi, model.SevHigh, "m1", "GET", "/api/users"),
		mkFinding("f3", model.ClassSQLi, model.SevHigh, "m1", "POST", "/api/orders"),
	}
	r := Analyze(findings)
	if r.UniqueLocations != 2 {
		t.Errorf("expected 2 unique locations, got %d", r.UniqueLocations)
	}
}

func TestOverallScoreBounded(t *testing.T) {
	findings := []model.Finding{
		mkFinding("f1", model.ClassSQLi, model.SevHigh, "m1", "GET", "/a"),
	}
	r := Analyze(findings)
	if r.Metrics.OverallScore < 0 || r.Metrics.OverallScore > 1.0 {
		t.Errorf("overall score should be 0-1, got %.2f", r.Metrics.OverallScore)
	}
}
