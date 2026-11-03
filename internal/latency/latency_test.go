package latency

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeBasic(t *testing.T) {
	rr := model.RunReport{
		RunID: "lat-1",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4", Agent: "recon"},
				Usage: model.Usage{LatencyMS: 1200},
			},
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "probe"},
				Usage: model.Usage{LatencyMS: 3500},
			},
		},
	}
	r := Analyze(rr)
	if r.RunID != "lat-1" {
		t.Errorf("run_id = %q, want lat-1", r.RunID)
	}
	if r.Summary.TotalModels != 2 {
		t.Errorf("total_models = %d, want 2", r.Summary.TotalModels)
	}
	if r.Summary.MaxMS != 3500 {
		t.Errorf("max_ms = %d, want 3500", r.Summary.MaxMS)
	}
	if r.Summary.MinMS != 1200 {
		t.Errorf("min_ms = %d, want 1200", r.Summary.MinMS)
	}
}

func TestAnalyzeOrdering(t *testing.T) {
	rr := model.RunReport{
		RunID: "ord-1",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "fast", Agent: "a"},
				Usage: model.Usage{LatencyMS: 200},
			},
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "slow", Agent: "b"},
				Usage: model.Usage{LatencyMS: 8000},
			},
		},
	}
	r := Analyze(rr)
	if len(r.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(r.Models))
	}
	if r.Models[0].LatencyMS < r.Models[1].LatencyMS {
		t.Error("models should be sorted by latency descending")
	}
}

func TestBottleneck(t *testing.T) {
	rr := model.RunReport{
		RunID: "bn-1",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "fast-model", Agent: "a"},
				Usage: model.Usage{LatencyMS: 500},
			},
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "slow-model", Agent: "b"},
				Usage: model.Usage{LatencyMS: 15000},
			},
		},
	}
	r := Analyze(rr)
	if r.Bottleneck != "slow-model" {
		t.Errorf("bottleneck = %q, want slow-model", r.Bottleneck)
	}
}

func TestGradeLatency(t *testing.T) {
	tests := []struct {
		ms    int64
		grade string
	}{
		{300, "A"},
		{500, "A"},
		{1500, "B"},
		{3000, "C"},
		{7000, "D"},
		{15000, "F"},
	}
	for _, tt := range tests {
		if got := gradeLatency(tt.ms); got != tt.grade {
			t.Errorf("gradeLatency(%d) = %q, want %q", tt.ms, got, tt.grade)
		}
	}
}

func TestEmptyRunReport(t *testing.T) {
	r := Analyze(model.RunReport{RunID: "empty"})
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
	if r.Summary.TotalModels != 0 {
		t.Errorf("total_models = %d, want 0", r.Summary.TotalModels)
	}
}

func TestPercentile(t *testing.T) {
	vals := []float64{100, 200, 300, 400, 500}
	p50 := percentile(vals, 50)
	if p50 != 300 {
		t.Errorf("p50 = %f, want 300", p50)
	}
	p99 := percentile(vals, 99)
	if p99 < 490 {
		t.Errorf("p99 = %f, want >= 490", p99)
	}
}

func TestPercentileSingle(t *testing.T) {
	vals := []float64{42}
	if got := percentile(vals, 50); got != 42 {
		t.Errorf("p50 single = %f, want 42", got)
	}
}

func TestMean(t *testing.T) {
	if got := mean([]float64{100, 200, 300}); got != 200 {
		t.Errorf("mean = %f, want 200", got)
	}
}

func TestStddev(t *testing.T) {
	vals := []float64{100, 100, 100}
	if got := stddev(vals, 100); got != 0 {
		t.Errorf("stddev uniform = %f, want 0", got)
	}
}

func TestOutlierDetection(t *testing.T) {
	rr := model.RunReport{
		RunID: "outlier-1",
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "a"}, Usage: model.Usage{LatencyMS: 100}},
			{Ref: model.ModelRef{Model: "b"}, Usage: model.Usage{LatencyMS: 110}},
			{Ref: model.ModelRef{Model: "c"}, Usage: model.Usage{LatencyMS: 120}},
			{Ref: model.ModelRef{Model: "d"}, Usage: model.Usage{LatencyMS: 105}},
			{Ref: model.ModelRef{Model: "outlier"}, Usage: model.Usage{LatencyMS: 50000}},
		},
	}
	r := Analyze(rr)
	outlierFound := false
	for _, m := range r.Models {
		if m.Model == "outlier" && m.IsOutlier {
			outlierFound = true
		}
	}
	if !outlierFound {
		t.Error("expected outlier model to be flagged")
	}
}

func TestFormatText(t *testing.T) {
	rr := model.RunReport{
		RunID: "fmt-1",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4", Agent: "recon"},
				Usage: model.Usage{LatencyMS: 1200},
			},
		},
	}
	r := Analyze(rr)
	text := FormatText(r)
	if !strings.Contains(text, "Latency Analysis") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "claude-sonnet-4") {
		t.Error("missing model name")
	}
	if !strings.Contains(text, "recon") {
		t.Error("missing agent name")
	}
}
