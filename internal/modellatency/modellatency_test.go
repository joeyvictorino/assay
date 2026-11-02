package modellatency

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, provider string, statusCodes ...int) model.Finding {
	var ev []model.Evidence
	for _, sc := range statusCodes {
		ev = append(ev, model.Evidence{Kind: "http-exchange", StatusCode: sc})
	}
	return model.Finding{
		Model:    model.ModelRef{Model: m, Provider: provider},
		Evidence: ev,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if r.Summary.TotalModels != 0 {
		t.Fatalf("expected 0 models, got %d", r.Summary.TotalModels)
	}
}

func TestSingleModel(t *testing.T) {
	ff := []model.Finding{
		finding("gpt-4", "openai", 200, 300, 400),
	}
	r := Analyze(ff)
	if r.Summary.TotalModels != 1 {
		t.Fatalf("expected 1 model, got %d", r.Summary.TotalModels)
	}
	if r.Summary.TotalSamples != 3 {
		t.Fatalf("expected 3 samples, got %d", r.Summary.TotalSamples)
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
}

func TestMultipleModels(t *testing.T) {
	ff := []model.Finding{
		finding("claude-3-opus", "anthropic", 100, 150, 200),
		finding("gpt-4", "openai", 500, 600, 700),
	}
	r := Analyze(ff)
	if r.Summary.TotalModels != 2 {
		t.Fatalf("expected 2 models, got %d", r.Summary.TotalModels)
	}
	if r.Summary.FastestModel != "claude-3-opus" {
		t.Fatalf("expected claude-3-opus fastest, got %s", r.Summary.FastestModel)
	}
	if r.Summary.SlowestModel != "gpt-4" {
		t.Fatalf("expected gpt-4 slowest, got %s", r.Summary.SlowestModel)
	}
}

func TestBucketDistribution(t *testing.T) {
	ff := []model.Finding{
		finding("fast", "p", 50, 80),
		finding("fast", "p", 150, 250),
		finding("fast", "p", 600),
	}
	r := Analyze(ff)
	m := r.Models[0]
	found100 := false
	found500 := false
	for _, b := range m.Buckets {
		if b.Label == "<100ms" && b.Count == 2 {
			found100 = true
		}
		if b.Label == "100-500ms" && b.Count == 2 {
			found500 = true
		}
	}
	if !found100 {
		t.Fatal("expected 2 samples in <100ms bucket")
	}
	if !found500 {
		t.Fatal("expected 2 samples in 100-500ms bucket")
	}
}

func TestShapeClassification(t *testing.T) {
	tests := []struct {
		sk   float64
		want string
	}{
		{1.5, "right-skewed"},
		{-1.5, "left-skewed"},
		{0.7, "moderately-right-skewed"},
		{-0.7, "moderately-left-skewed"},
		{0.2, "symmetric"},
	}
	for _, tt := range tests {
		got := classifyShape(tt.sk)
		if got != tt.want {
			t.Errorf("classifyShape(%.1f) = %s, want %s", tt.sk, got, tt.want)
		}
	}
}

func TestSpreadRatio(t *testing.T) {
	ff := []model.Finding{
		finding("fast", "p", 100, 100),
		finding("slow", "p", 1000, 1000),
	}
	r := Analyze(ff)
	if r.Summary.SpreadRatio != 10.0 {
		t.Fatalf("expected spread 10.0, got %.2f", r.Summary.SpreadRatio)
	}
}

func TestPercentiles(t *testing.T) {
	ff := []model.Finding{
		finding("m", "p", 100, 200, 300, 400, 500, 600, 700, 800, 900, 1000),
	}
	r := Analyze(ff)
	m := r.Models[0]
	if m.MedianMS < 500 || m.MedianMS > 600 {
		t.Fatalf("median should be ~550, got %.0f", m.MedianMS)
	}
	if m.P95MS < 900 {
		t.Fatalf("P95 should be >=900, got %.0f", m.P95MS)
	}
}

func TestSkipsZeroStatusCode(t *testing.T) {
	ff := []model.Finding{
		{
			Model: model.ModelRef{Model: "m", Provider: "p"},
			Evidence: []model.Evidence{
				{Kind: "marker-reflected", StatusCode: 0},
			},
		},
	}
	r := Analyze(ff)
	if r.Summary.TotalModels != 0 {
		t.Fatalf("expected 0 models for zero status code, got %d", r.Summary.TotalModels)
	}
}

func TestSkipsEmptyModel(t *testing.T) {
	ff := []model.Finding{
		{
			Model:    model.ModelRef{Model: "", Provider: "p"},
			Evidence: []model.Evidence{{Kind: "http-exchange", StatusCode: 200}},
		},
	}
	r := Analyze(ff)
	if r.Summary.TotalModels != 0 {
		t.Fatalf("expected 0 models for empty model name, got %d", r.Summary.TotalModels)
	}
}

func TestFormatText(t *testing.T) {
	ff := []model.Finding{
		finding("gpt-4", "openai", 200, 500, 800),
	}
	r := Analyze(ff)
	text := FormatText(r)
	if !strings.Contains(text, "Response Time Distribution") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name in output")
	}
}

func TestFormatJSON(t *testing.T) {
	ff := []model.Finding{
		finding("claude", "anthropic", 100),
	}
	r := Analyze(ff)
	j := FormatJSON(r)
	if !strings.Contains(j, "\"model\"") {
		t.Fatal("expected JSON field")
	}
}

func TestStdDevSingleSample(t *testing.T) {
	ff := []model.Finding{
		finding("m", "p", 500),
	}
	r := Analyze(ff)
	if r.Models[0].StdDev != 0 {
		t.Fatalf("expected stddev 0 for single sample, got %.2f", r.Models[0].StdDev)
	}
}
