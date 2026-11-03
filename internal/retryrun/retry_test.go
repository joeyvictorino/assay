package retryrun

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestNoFailures(t *testing.T) {
	rr := model.RunReport{
		RunID: "run-1",
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "gpt-4", Provider: "openai"}, Refusals: 0, Failovers: 0},
		},
	}
	r := AnalyzeFailures(rr)
	if r.TotalFailed != 0 {
		t.Errorf("failed = %d, want 0", r.TotalFailed)
	}
	if r.TotalSucceeded != 1 {
		t.Errorf("succeeded = %d, want 1", r.TotalSucceeded)
	}
}

func TestWithRefusals(t *testing.T) {
	rr := model.RunReport{
		RunID: "run-2",
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "gpt-4", Provider: "openai"}, Refusals: 3, Failovers: 0},
			{Ref: model.ModelRef{Model: "claude", Provider: "anthropic"}, Refusals: 0, Failovers: 0},
		},
	}
	r := AnalyzeFailures(rr)
	if r.TotalFailed != 1 {
		t.Errorf("failed = %d, want 1", r.TotalFailed)
	}
	if r.FailedCheckers[0].Refusals != 3 {
		t.Errorf("refusals = %d, want 3", r.FailedCheckers[0].Refusals)
	}
}

func TestWithFailovers(t *testing.T) {
	rr := model.RunReport{
		RunID: "run-3",
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "gpt-4", Provider: "openai"}, Refusals: 0, Failovers: 2},
		},
	}
	r := AnalyzeFailures(rr)
	if r.TotalFailed != 1 {
		t.Errorf("failed = %d, want 1", r.TotalFailed)
	}
}

func TestEmptyReport(t *testing.T) {
	rr := model.RunReport{RunID: "empty"}
	r := AnalyzeFailures(rr)
	if r.TotalFailed != 0 {
		t.Errorf("failed = %d, want 0", r.TotalFailed)
	}
}

func TestFormatText(t *testing.T) {
	rr := model.RunReport{
		RunID: "run-4",
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "gpt-4", Provider: "openai"}, Refusals: 1, Failovers: 1},
		},
	}
	r := AnalyzeFailures(rr)
	out := FormatText(r)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatJSON(t *testing.T) {
	rr := model.RunReport{
		RunID: "run-5",
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "gpt-4", Provider: "openai"}, Refusals: 0, Failovers: 0},
		},
	}
	r := AnalyzeFailures(rr)
	out, err := FormatJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestMultipleFailures(t *testing.T) {
	rr := model.RunReport{
		RunID: "run-6",
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "gpt-4", Provider: "openai"}, Refusals: 2, Failovers: 1},
			{Ref: model.ModelRef{Model: "claude", Provider: "anthropic"}, Refusals: 1, Failovers: 0},
			{Ref: model.ModelRef{Model: "gemini", Provider: "google"}, Refusals: 0, Failovers: 0},
		},
	}
	r := AnalyzeFailures(rr)
	if r.TotalFailed != 2 {
		t.Errorf("failed = %d, want 2", r.TotalFailed)
	}
	if r.TotalSucceeded != 1 {
		t.Errorf("succeeded = %d, want 1", r.TotalSucceeded)
	}
}
