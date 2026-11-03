package rundigest

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeReport() model.RunReport {
	return model.RunReport{
		RunID:         "run-001",
		Verdict:       "pass",
		WeightedScore: 87.5,
		Started:       time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC),
		Finished:      time.Date(2024, 6, 15, 10, 5, 30, 0, time.UTC),
		SpentUSD:      0.0234,
		Models: []model.ModelResult{
			{
				Ref: model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
				Findings: map[model.FindingState]int{
					model.StateValidated: 12,
					model.StateRefuted:   2,
					model.StateTheorized: 5,
				},
				Usage:    model.Usage{CostUSD: 0.0150},
				P50MS:    450,
				Refusals: 1,
			},
			{
				Ref: model.ModelRef{Provider: "openai", Model: "gpt-4o"},
				Findings: map[model.FindingState]int{
					model.StateValidated: 8,
					model.StateRefuted:   3,
					model.StateTheorized: 4,
				},
				Usage:    model.Usage{CostUSD: 0.0084},
				P50MS:    320,
				Refusals: 0,
			},
		},
	}
}

func TestSummarizeBasic(t *testing.T) {
	d := Summarize(makeReport())
	if d.RunID != "run-001" {
		t.Errorf("RunID = %q", d.RunID)
	}
	if d.Verdict != "pass" {
		t.Errorf("Verdict = %q", d.Verdict)
	}
	if d.Models != 2 {
		t.Errorf("Models = %d", d.Models)
	}
}

func TestSummarizeFindings(t *testing.T) {
	d := Summarize(makeReport())
	if d.Validated != 20 {
		t.Errorf("Validated = %d, want 20", d.Validated)
	}
	if d.Refuted != 5 {
		t.Errorf("Refuted = %d, want 5", d.Refuted)
	}
	if d.Theorized != 9 {
		t.Errorf("Theorized = %d, want 9", d.Theorized)
	}
}

func TestSummarizeCost(t *testing.T) {
	d := Summarize(makeReport())
	if d.TotalCost != 0.0234 {
		t.Errorf("TotalCost = %f, want 0.0234", d.TotalCost)
	}
}

func TestSummarizeRefusals(t *testing.T) {
	d := Summarize(makeReport())
	if d.Refusals != 1 {
		t.Errorf("Refusals = %d, want 1", d.Refusals)
	}
}

func TestSummarizeDuration(t *testing.T) {
	d := Summarize(makeReport())
	if d.Duration != "5.5m" {
		t.Errorf("Duration = %q, want 5.5m", d.Duration)
	}
}

func TestSummarizeTopModel(t *testing.T) {
	d := Summarize(makeReport())
	if d.TopModel != "claude-sonnet-4-20250514" {
		t.Errorf("TopModel = %q", d.TopModel)
	}
}

func TestSummarizeAvgLatency(t *testing.T) {
	d := Summarize(makeReport())
	if d.AvgLatency != 385 {
		t.Errorf("AvgLatency = %f, want 385", d.AvgLatency)
	}
}

func TestOneLineContainsVerdict(t *testing.T) {
	d := Summarize(makeReport())
	if !strings.Contains(d.OneLine, "[PASS]") {
		t.Errorf("OneLine missing verdict: %q", d.OneLine)
	}
}

func TestParagraphContainsRunID(t *testing.T) {
	d := Summarize(makeReport())
	if !strings.Contains(d.Paragraph, "run-001") {
		t.Errorf("Paragraph missing RunID: %q", d.Paragraph)
	}
}

func TestFormatTextOutput(t *testing.T) {
	d := Summarize(makeReport())
	text := FormatText(d)
	if !strings.Contains(text, "Run Digest") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "run-001") {
		t.Error("missing run ID")
	}
	if !strings.Contains(text, "PASS") {
		t.Error("missing verdict")
	}
}

func TestSummarizeEmpty(t *testing.T) {
	d := Summarize(model.RunReport{RunID: "empty"})
	if d.Models != 0 {
		t.Errorf("Models = %d", d.Models)
	}
	if d.OneLine == "" {
		t.Error("OneLine should not be empty")
	}
}

func TestSummarizeJSON(t *testing.T) {
	d := Summarize(makeReport())
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["run_id"] != "run-001" {
		t.Errorf("JSON run_id = %v", m["run_id"])
	}
}

func TestFormatDurationHours(t *testing.T) {
	d := formatDuration(2*time.Hour + 30*time.Minute)
	if d != "2.5h" {
		t.Errorf("got %q, want 2.5h", d)
	}
}

func TestFormatDurationSeconds(t *testing.T) {
	d := formatDuration(45 * time.Second)
	if d != "45s" {
		t.Errorf("got %q, want 45s", d)
	}
}

func TestFormatDurationNegative(t *testing.T) {
	d := formatDuration(-5 * time.Second)
	if d != "" {
		t.Errorf("got %q, want empty", d)
	}
}
