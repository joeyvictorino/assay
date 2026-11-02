package temperature

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func findings(entries ...model.Finding) []model.Finding {
	return entries
}

func finding(mdl string, temp float64, sev model.Severity) model.Finding {
	return model.Finding{
		Model:       model.ModelRef{Model: mdl},
		Temperature: temp,
		Severity:    sev,
	}
}

func TestEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalFindings != 0 {
		t.Fatalf("want 0, got %d", r.TotalFindings)
	}
}

func TestSingleFinding(t *testing.T) {
	r := Analyze(findings(finding("gpt-4", 0.7, model.SevMedium)))
	if r.TotalFindings != 1 {
		t.Fatalf("want 1, got %d", r.TotalFindings)
	}
	if len(r.Overall) != 1 {
		t.Fatalf("want 1 bucket, got %d", len(r.Overall))
	}
	if r.Overall[0].Temperature != 0.7 {
		t.Fatalf("want temp 0.7, got %.1f", r.Overall[0].Temperature)
	}
}

func TestMultipleTemperatures(t *testing.T) {
	r := Analyze(findings(
		finding("gpt-4", 0.0, model.SevLow),
		finding("gpt-4", 0.5, model.SevMedium),
		finding("gpt-4", 1.0, model.SevHigh),
	))
	if len(r.Overall) != 3 {
		t.Fatalf("want 3 buckets, got %d", len(r.Overall))
	}
	if r.SafestTemp != 0.0 {
		t.Fatalf("want safest at 0.0, got %.1f", r.SafestTemp)
	}
	if r.DegradationAt != 1.0 {
		t.Fatalf("want degradation at 1.0, got %.1f", r.DegradationAt)
	}
}

func TestByModel(t *testing.T) {
	r := Analyze(findings(
		finding("gpt-4", 0.3, model.SevLow),
		finding("claude-3", 0.3, model.SevMedium),
	))
	if len(r.ByModel) != 2 {
		t.Fatalf("want 2 models, got %d", len(r.ByModel))
	}
}

func TestSafetyScore(t *testing.T) {
	r := Analyze(findings(finding("gpt-4", 0.0, model.SevCritical)))
	if r.Overall[0].SafetyScore != 0.0 {
		t.Fatalf("want safety 0.0 for critical, got %.2f", r.Overall[0].SafetyScore)
	}

	r2 := Analyze(findings(finding("gpt-4", 0.0, model.SevLow)))
	if r2.Overall[0].SafetyScore < 0.7 {
		t.Fatalf("want safety > 0.7 for low, got %.2f", r2.Overall[0].SafetyScore)
	}
}

func TestRoundTemp(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{0.72, 0.7},
		{0.75, 0.8},
		{1.0, 1.0},
		{0.0, 0.0},
	}
	for _, tt := range tests {
		got := roundTemp(tt.in)
		if got != tt.want {
			t.Errorf("roundTemp(%.2f) = %.1f, want %.1f", tt.in, got, tt.want)
		}
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze(findings(finding("gpt-4", 0.5, model.SevMedium)))
	text := FormatText(r)
	if !strings.Contains(text, "Temperature") {
		t.Fatal("should contain header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze(findings(finding("gpt-4", 0.5, model.SevMedium)))
	j := FormatJSON(r)
	if !strings.Contains(j, "safest_temperature") {
		t.Fatal("should contain safest_temperature field")
	}
}

func TestSeverityToNum(t *testing.T) {
	tests := []struct {
		sev  model.Severity
		want float64
	}{
		{model.SevCritical, 4},
		{model.SevHigh, 3},
		{model.SevMedium, 2},
		{model.SevLow, 1},
		{model.Severity("unknown"), 0},
	}
	for _, tt := range tests {
		got := severityToNum(tt.sev)
		if got != tt.want {
			t.Errorf("severityToNum(%q) = %.0f, want %.0f", tt.sev, got, tt.want)
		}
	}
}
