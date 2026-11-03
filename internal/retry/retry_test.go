package retry

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzePassingRun(t *testing.T) {
	rr := model.RunReport{
		RunID:   "pass-1",
		Verdict: "PASS",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 100000, OutputTokens: 50000},
			},
		},
	}

	p := Analyze(rr, 3)
	if p.ShouldRetry {
		t.Error("passing run should not need retry")
	}
	if !strings.Contains(p.Reason, "passed") {
		t.Errorf("reason = %q, want mention of passed", p.Reason)
	}
}

func TestAnalyzeWithRefusals(t *testing.T) {
	rr := model.RunReport{
		RunID:   "fail-1",
		Verdict: "BLOCKED",
		Models: []model.ModelResult{
			{
				Ref:      model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4", Agent: "probe"},
				Refusals: 3,
			},
		},
	}

	p := Analyze(rr, 3)
	if !p.ShouldRetry {
		t.Error("run with refusals should retry")
	}
	if len(p.FailedModels) != 1 {
		t.Fatalf("failed_models count = %d, want 1", len(p.FailedModels))
	}
	if p.FailedModels[0].FailureKind != FailBlocked {
		t.Errorf("failure_kind = %s, want %s", p.FailedModels[0].FailureKind, FailBlocked)
	}
}

func TestAnalyzeWithFailovers(t *testing.T) {
	rr := model.RunReport{
		RunID:   "fail-2",
		Verdict: "ERROR",
		Models: []model.ModelResult{
			{
				Ref:       model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Failovers: 2,
			},
		},
	}

	p := Analyze(rr, 5)
	if !p.ShouldRetry {
		t.Error("run with failovers should retry")
	}
	if p.FailedModels[0].FailureKind != FailError {
		t.Errorf("failure_kind = %s, want %s", p.FailedModels[0].FailureKind, FailError)
	}
}

func TestAnalyzeTruncated(t *testing.T) {
	rr := model.RunReport{
		RunID:     "trunc-1",
		Verdict:   "BLOCKED",
		Truncated: true,
	}

	p := Analyze(rr, 2)
	if !p.ShouldRetry {
		t.Error("truncated run should retry")
	}
	found := false
	for _, fm := range p.FailedModels {
		if fm.FailureKind == FailTruncate {
			found = true
		}
	}
	if !found {
		t.Error("expected TRUNCATED failure kind")
	}
}

func TestAnalyzeErrorExitNoFailures(t *testing.T) {
	rr := model.RunReport{
		RunID:    "err-exit",
		Verdict:  "ERROR",
		ExitCode: 1,
	}

	p := Analyze(rr, 3)
	if !p.ShouldRetry {
		t.Error("error exit with no specific failures should still retry")
	}
	if !strings.Contains(p.Reason, "transient") {
		t.Errorf("reason = %q, want mention of transient", p.Reason)
	}
}

func TestBackoffSchedule(t *testing.T) {
	ds := backoffSchedule(5)
	if len(ds) != 5 {
		t.Fatalf("schedule length = %d, want 5", len(ds))
	}
	for i := 1; i < len(ds); i++ {
		if ds[i].Delay <= ds[i-1].Delay {
			t.Errorf("delay[%d]=%s not > delay[%d]=%s", i, ds[i].DelayS, i-1, ds[i-1].DelayS)
		}
	}
	if ds[0].Attempt != 1 {
		t.Errorf("first attempt = %d, want 1", ds[0].Attempt)
	}
}

func TestBackoffCap(t *testing.T) {
	ds := backoffSchedule(10)
	for _, d := range ds {
		if d.Delay.Seconds() > 900 {
			t.Errorf("delay %s exceeds 15m cap", d.DelayS)
		}
	}
}

func TestFormatTextPassing(t *testing.T) {
	p := RetryPlan{
		RunID:       "fmt-pass",
		Verdict:     "PASS",
		ShouldRetry: false,
		Reason:      "run passed; no retry needed",
	}
	text := FormatText(p)
	if !strings.Contains(text, "Retry Analysis") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "false") {
		t.Error("expected retry=false")
	}
}

func TestFormatTextWithFailures(t *testing.T) {
	p := RetryPlan{
		RunID:       "fmt-fail",
		Verdict:     "ERROR",
		ExitCode:    1,
		ShouldRetry: true,
		Reason:      "1 model(s) with failures",
		FailedModels: []FailedModel{
			{Checker: "recon", Model: "gpt-4o", FailureKind: FailError, Failovers: 2},
		},
		MaxRetries: 3,
		Delays:     backoffSchedule(3),
	}
	text := FormatText(p)
	if !strings.Contains(text, "recon") {
		t.Error("expected checker name in output")
	}
	if !strings.Contains(text, "Backoff Schedule") {
		t.Error("expected backoff schedule")
	}
}
