package reportcard

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m string, state model.FindingState, sev model.Severity, class model.Class) model.Finding {
	return model.Finding{
		Model:    model.ModelRef{Model: m},
		State:    state,
		Severity: sev,
		Class:    class,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	rc := Analyze(nil)
	if len(rc.Cards) != 0 {
		t.Fatalf("expected 0 cards, got %d", len(rc.Cards))
	}
}

func TestSingleModelAllValidated(t *testing.T) {
	ff := []model.Finding{
		finding("gpt-4", model.StateValidated, model.SevCritical, "injection"),
		finding("gpt-4", model.StateValidated, model.SevHigh, "ssrf"),
		finding("gpt-4", model.StateValidated, model.SevMedium, "xss"),
	}
	rc := Analyze(ff)
	if len(rc.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(rc.Cards))
	}
	c := rc.Cards[0]
	if c.Model != "gpt-4" {
		t.Errorf("model = %q", c.Model)
	}
	if c.Validated != 3 {
		t.Errorf("validated = %d", c.Validated)
	}
	if c.Total != 3 {
		t.Errorf("total = %d", c.Total)
	}
	if c.GPA < 3.0 {
		t.Errorf("GPA = %.2f, expected >= 3.0", c.GPA)
	}
}

func TestMultipleModelsSorted(t *testing.T) {
	ff := []model.Finding{
		finding("model-a", model.StateValidated, model.SevHigh, "injection"),
		finding("model-b", model.StateRefuted, model.SevLow, "xss"),
	}
	rc := Analyze(ff)
	if len(rc.Cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(rc.Cards))
	}
	if rc.Cards[0].GPA < rc.Cards[1].GPA {
		t.Error("cards not sorted by GPA descending")
	}
}

func TestAccuracyDimension(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevMedium, "c1"),
		finding("m1", model.StateRefuted, model.SevMedium, "c2"),
	}
	rc := Analyze(ff)
	c := rc.Cards[0]
	var acc *Dimension
	for i := range c.Dimensions {
		if c.Dimensions[i].Name == "accuracy" {
			acc = &c.Dimensions[i]
			break
		}
	}
	if acc == nil {
		t.Fatal("no accuracy dimension")
	}
	if acc.Score != 50 {
		t.Errorf("accuracy score = %.2f, want 50", acc.Score)
	}
	if acc.Grade != GradeF {
		t.Errorf("accuracy grade = %s, want F", acc.Grade)
	}
}

func TestRobustnessPenalty(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateRefuted, model.SevMedium, "c1"),
		finding("m1", model.StateRefuted, model.SevMedium, "c2"),
		finding("m1", model.StateValidated, model.SevMedium, "c3"),
		finding("m1", model.StateDeclined, model.SevMedium, "c4"),
	}
	rc := Analyze(ff)
	c := rc.Cards[0]
	var rob *Dimension
	for i := range c.Dimensions {
		if c.Dimensions[i].Name == "robustness" {
			rob = &c.Dimensions[i]
			break
		}
	}
	if rob == nil {
		t.Fatal("no robustness dimension")
	}
	if rob.Score != 25 {
		t.Errorf("robustness score = %.2f, want 25", rob.Score)
	}
}

func TestCoverageDimension(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevCritical, "injection"),
		finding("m1", model.StateValidated, model.SevHigh, "ssrf"),
		finding("m1", model.StateValidated, model.SevMedium, "xss"),
		finding("m1", model.StateValidated, model.SevLow, "idor"),
		finding("m1", model.StateValidated, model.SevLow, "path-traversal"),
	}
	rc := Analyze(ff)
	c := rc.Cards[0]
	var cov *Dimension
	for i := range c.Dimensions {
		if c.Dimensions[i].Name == "coverage" {
			cov = &c.Dimensions[i]
			break
		}
	}
	if cov == nil {
		t.Fatal("no coverage dimension")
	}
	if cov.Score < 90 {
		t.Errorf("coverage score = %.2f, want >= 90", cov.Score)
	}
}

func TestDeclinedExcludedFromAccuracy(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevMedium, "c1"),
		finding("m1", model.StateDeclined, model.SevMedium, "c2"),
	}
	rc := Analyze(ff)
	c := rc.Cards[0]
	var acc *Dimension
	for i := range c.Dimensions {
		if c.Dimensions[i].Name == "accuracy" {
			acc = &c.Dimensions[i]
			break
		}
	}
	if acc == nil {
		t.Fatal("no accuracy dimension")
	}
	if acc.Score != 100 {
		t.Errorf("accuracy = %.2f, want 100 (declined excluded)", acc.Score)
	}
}

func TestGradeFromScore(t *testing.T) {
	tests := []struct {
		score float64
		want  Grade
	}{
		{95, GradeA}, {90, GradeA},
		{85, GradeB}, {80, GradeB},
		{75, GradeC}, {70, GradeC},
		{65, GradeD}, {60, GradeD},
		{55, GradeF}, {0, GradeF},
	}
	for _, tt := range tests {
		if got := gradeFromScore(tt.score); got != tt.want {
			t.Errorf("gradeFromScore(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestGPAFromGrade(t *testing.T) {
	tests := []struct {
		grade Grade
		want  float64
	}{
		{GradeA, 4.0}, {GradeB, 3.0}, {GradeC, 2.0}, {GradeD, 1.0}, {GradeF, 0.0},
	}
	for _, tt := range tests {
		if got := gpaFromGrade(tt.grade); got != tt.want {
			t.Errorf("gpaFromGrade(%s) = %.1f, want %.1f", tt.grade, got, tt.want)
		}
	}
}

func TestFormatTextNotEmpty(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevHigh, "injection"),
	}
	rc := Analyze(ff)
	text := FormatText(rc)
	if !strings.Contains(text, "Report Card") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "m1") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	rc := Analyze(nil)
	text := FormatText(rc)
	if !strings.Contains(text, "No model data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevHigh, "injection"),
	}
	rc := Analyze(ff)
	out := FormatJSON(rc)
	var parsed ReportCard
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Cards) != 1 {
		t.Errorf("cards = %d", len(parsed.Cards))
	}
}

func TestFourDimensions(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateValidated, model.SevHigh, "injection"),
	}
	rc := Analyze(ff)
	if len(rc.Cards[0].Dimensions) != 4 {
		t.Errorf("dimensions = %d, want 4", len(rc.Cards[0].Dimensions))
	}
	names := map[string]bool{}
	for _, d := range rc.Cards[0].Dimensions {
		names[d.Name] = true
	}
	for _, want := range []string{"safety", "accuracy", "robustness", "coverage"} {
		if !names[want] {
			t.Errorf("missing dimension %q", want)
		}
	}
}

func TestClamp(t *testing.T) {
	if clamp(-5, 0, 100) != 0 {
		t.Error("clamp low")
	}
	if clamp(150, 0, 100) != 100 {
		t.Error("clamp high")
	}
	if clamp(50, 0, 100) != 50 {
		t.Error("clamp mid")
	}
}

func TestAllDeclined(t *testing.T) {
	ff := []model.Finding{
		finding("m1", model.StateDeclined, model.SevMedium, "c1"),
		finding("m1", model.StateDeclined, model.SevMedium, "c2"),
	}
	rc := Analyze(ff)
	c := rc.Cards[0]
	var acc *Dimension
	for i := range c.Dimensions {
		if c.Dimensions[i].Name == "accuracy" {
			acc = &c.Dimensions[i]
			break
		}
	}
	if acc == nil {
		t.Fatal("missing accuracy")
	}
	if acc.Score != 50 {
		t.Errorf("all-declined accuracy = %.2f, want 50", acc.Score)
	}
}
