package injdefense

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id string, class model.Class, sev model.Severity) model.Finding {
	return model.Finding{ID: id, Class: class, Severity: sev, State: model.StateValidated}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil, nil)
	if r.Summary.TotalDefenses != 0 {
		t.Fatalf("expected 0 defenses, got %d", r.Summary.TotalDefenses)
	}
	if r.Summary.TotalAttempts != 0 {
		t.Fatalf("expected 0 attempts, got %d", r.Summary.TotalAttempts)
	}
}

func TestAnalyzeFullBlock(t *testing.T) {
	baseline := []model.Finding{
		finding("f1", "injection", model.SevHigh),
		finding("f2", "injection", model.SevCritical),
	}
	defended := []model.Finding{}
	r := Analyze(defended, baseline)
	if r.Summary.OverallBlockRate != 100 {
		t.Errorf("expected 100%% block rate, got %.1f%%", r.Summary.OverallBlockRate)
	}
}

func TestAnalyzeNoneBlocked(t *testing.T) {
	baseline := []model.Finding{
		finding("f1", "injection", model.SevHigh),
		finding("f2", "injection", model.SevCritical),
	}
	defended := []model.Finding{
		finding("f1", "injection", model.SevHigh),
		finding("f2", "injection", model.SevCritical),
	}
	r := Analyze(defended, baseline)
	if r.Summary.OverallBlockRate != 0 {
		t.Errorf("expected 0%% block rate, got %.1f%%", r.Summary.OverallBlockRate)
	}
}

func TestAnalyzePartialBlock(t *testing.T) {
	baseline := []model.Finding{
		finding("f1", "injection", model.SevHigh),
		finding("f2", "injection", model.SevMedium),
		finding("f3", "exfil", model.SevHigh),
		finding("f4", "exfil", model.SevLow),
	}
	defended := []model.Finding{
		finding("f1", "injection", model.SevHigh),
		finding("f3", "exfil", model.SevHigh),
	}
	r := Analyze(defended, baseline)
	if r.Summary.OverallBlockRate != 50 {
		t.Errorf("expected 50%% block rate, got %.1f%%", r.Summary.OverallBlockRate)
	}
}

func TestAnalyzeCriticalBypasses(t *testing.T) {
	baseline := []model.Finding{
		finding("f1", "injection", model.SevCritical),
		finding("f2", "ssrf", model.SevHigh),
	}
	defended := []model.Finding{
		finding("f1", "injection", model.SevCritical),
	}
	r := Analyze(defended, baseline)
	if r.Summary.CriticalBypasses != 1 {
		t.Errorf("expected 1 critical bypass, got %d", r.Summary.CriticalBypasses)
	}
}

func TestAnalyzeDefenseGrading(t *testing.T) {
	tests := []struct {
		rate float64
		want string
	}{
		{100, "A"},
		{95, "A"},
		{85, "B"},
		{70, "C"},
		{50, "D"},
		{30, "F"},
		{0, "F"},
	}
	for _, tt := range tests {
		got := rateToGrade(tt.rate)
		if got != tt.want {
			t.Errorf("rateToGrade(%.0f) = %s, want %s", tt.rate, got, tt.want)
		}
	}
}

func TestClassifyDefense(t *testing.T) {
	tests := []struct {
		class string
		want  DefenseType
	}{
		{"injection", DefInputFilter},
		{"prompt-inject", DefInputFilter},
		{"exfiltration", DefOutputFilter},
		{"data-leak", DefOutputFilter},
		{"system-prompt-bypass", DefSystemPrompt},
		{"rate-limit-bypass", DefRateLimiting},
		{"unknown-class", DefContentPolicy},
	}
	for _, tt := range tests {
		got := classifyDefense(tt.class)
		if got != tt.want {
			t.Errorf("classifyDefense(%q) = %s, want %s", tt.class, got, tt.want)
		}
	}
}

func TestAnalyzeStrongestWeakest(t *testing.T) {
	baseline := []model.Finding{
		finding("f1", "injection", model.SevHigh),
		finding("f2", "injection", model.SevMedium),
		finding("f3", "exfil", model.SevHigh),
		finding("f4", "exfil", model.SevLow),
	}
	defended := []model.Finding{
		finding("f3", "exfil", model.SevHigh),
		finding("f4", "exfil", model.SevLow),
	}
	r := Analyze(defended, baseline)
	if r.Summary.WeakestDefense == "" {
		t.Error("expected weakest defense to be set")
	}
	if r.Summary.StrongestDefense == "" {
		t.Error("expected strongest defense to be set")
	}
	if r.Summary.WeakestDefense == r.Summary.StrongestDefense && r.Summary.TotalDefenses > 1 {
		t.Error("expected different weakest and strongest when multiple defenses differ")
	}
}

func TestFormatTextOutput(t *testing.T) {
	baseline := []model.Finding{
		finding("f1", "injection", model.SevHigh),
	}
	r := Analyze(nil, baseline)
	text := FormatText(r)
	if !strings.Contains(text, "Prompt Injection Defense") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "block rate") {
		t.Error("missing block rate")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil, nil)
	text := FormatText(r)
	if !strings.Contains(text, "No defense data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	baseline := []model.Finding{
		finding("f1", "injection", model.SevHigh),
	}
	r := Analyze(nil, baseline)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestIsCriticalClass(t *testing.T) {
	if !isCriticalClass("injection") {
		t.Error("injection should be critical")
	}
	if !isCriticalClass("data-exfil") {
		t.Error("exfil should be critical")
	}
	if !isCriticalClass("ssrf") {
		t.Error("ssrf should be critical")
	}
	if isCriticalClass("info-disclosure") {
		t.Error("info-disclosure should not be critical")
	}
}

func TestGroupByClass(t *testing.T) {
	findings := []model.Finding{
		finding("f1", "injection", model.SevHigh),
		finding("f2", "injection", model.SevLow),
		finding("f3", "exfil", model.SevMedium),
	}
	groups := groupByClass(findings)
	if len(groups["injection"]) != 2 {
		t.Errorf("expected 2 injection findings, got %d", len(groups["injection"]))
	}
	if len(groups["exfil"]) != 1 {
		t.Errorf("expected 1 exfil finding, got %d", len(groups["exfil"]))
	}
}
