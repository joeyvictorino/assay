package responselatency

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(mn, lab string, latencyMS int) model.Finding {
	first := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	last := first.Add(time.Duration(latencyMS) * time.Millisecond)
	return model.Finding{
		ID:        mn + "-" + lab,
		Lab:       lab,
		Summary:   "response about " + lab,
		Model:     model.ModelRef{Model: mn},
		FirstSeen: first,
		LastSeen:  last,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestSingleFinding(t *testing.T) {
	fs := []model.Finding{mkFinding("gpt-4", "lab-1", 500)}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].Detail.MeanMS != 500 {
		t.Fatalf("expected mean 500ms, got %.1f", r.Models[0].Detail.MeanMS)
	}
}

func TestMultipleLatencies(t *testing.T) {
	fs := []model.Finding{
		mkFinding("claude", "lab-1", 100),
		mkFinding("claude", "lab-2", 200),
		mkFinding("claude", "lab-3", 300),
	}
	r := Analyze(fs)
	m := r.Models[0]
	if m.Detail.MedianMS != 200 {
		t.Fatalf("expected median 200ms, got %.1f", m.Detail.MedianMS)
	}
	if m.Detail.MinMS != 100 {
		t.Fatalf("expected min 100ms, got %.1f", m.Detail.MinMS)
	}
	if m.Detail.MaxMS != 300 {
		t.Fatalf("expected max 300ms, got %.1f", m.Detail.MaxMS)
	}
}

func TestPercentiles(t *testing.T) {
	var fs []model.Finding
	for i := 1; i <= 100; i++ {
		fs = append(fs, mkFinding("m1", "lab", i*10))
	}
	r := Analyze(fs)
	m := r.Models[0]
	if m.Detail.P90MS < 900 || m.Detail.P90MS > 910 {
		t.Fatalf("expected p90 around 900ms, got %.1f", m.Detail.P90MS)
	}
	if m.Detail.P95MS < 950 || m.Detail.P95MS > 960 {
		t.Fatalf("expected p95 around 950ms, got %.1f", m.Detail.P95MS)
	}
}

func TestSlowDetection(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "lab-1", 100),
		mkFinding("m1", "lab-2", 6000),
		mkFinding("m1", "lab-3", 7000),
	}
	r := Analyze(fs)
	if r.Models[0].SlowCount != 2 {
		t.Fatalf("expected 2 slow, got %d", r.Models[0].SlowCount)
	}
	if len(r.Models[0].SlowPatterns) != 2 {
		t.Fatalf("expected 2 slow patterns, got %d", len(r.Models[0].SlowPatterns))
	}
	if r.Models[0].SlowPatterns[0].LatencyMS < r.Models[0].SlowPatterns[1].LatencyMS {
		t.Fatal("slow patterns should be sorted by latency descending")
	}
}

func TestTimeoutDetection(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "lab-1", 100),
		mkFinding("m1", "lab-2", 35000),
	}
	r := Analyze(fs)
	if r.Models[0].TimeoutCount != 1 {
		t.Fatalf("expected 1 timeout, got %d", r.Models[0].TimeoutCount)
	}
	if r.TotalTimeout != 1 {
		t.Fatalf("expected 1 total timeout, got %d", r.TotalTimeout)
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		mkFinding("fast-model", "lab-1", 50),
		mkFinding("slow-model", "lab-1", 8000),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Fastest == "" || r.Slowest == "" {
		t.Fatal("expected fastest/slowest to be set")
	}
	if r.Models[0].LatencyScore < r.Models[1].LatencyScore {
		t.Fatal("models should be sorted by score descending")
	}
}

func TestZeroLatency(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1", Summary: "answer",
			Model: model.ModelRef{Model: "m1"},
		},
	}
	r := Analyze(fs)
	if r.Models[0].Grade != "N/A" {
		t.Fatalf("expected N/A grade for no latency data, got %s", r.Models[0].Grade)
	}
}

func TestUnknownModel(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1", Summary: "answer",
			Model:     model.ModelRef{Model: ""},
			FirstSeen: time.Now(),
			LastSeen:  time.Now().Add(100 * time.Millisecond),
		},
	}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestStdDev(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "lab-1", 100),
		mkFinding("m1", "lab-2", 100),
		mkFinding("m1", "lab-3", 100),
	}
	r := Analyze(fs)
	if r.Models[0].Detail.StdDevMS != 0 {
		t.Fatalf("expected 0 stddev for identical values, got %.1f", r.Models[0].Detail.StdDevMS)
	}
}

func TestGradeAssignment(t *testing.T) {
	cases := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, tc := range cases {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tc.score, got, tc.grade)
		}
	}
}

func TestFormatText_Empty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Response Latency") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "No latency data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatText_WithData(t *testing.T) {
	fs := []model.Finding{mkFinding("claude", "lab-1", 250)}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "claude") {
		t.Fatal("expected model name")
	}
	if !strings.Contains(text, "mean") {
		t.Fatal("expected mean field")
	}
	if !strings.Contains(text, "median") {
		t.Fatal("expected median field")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "lab-1", 500)}
	r := Analyze(fs)
	out := FormatJSON(r)
	var data map[string]any
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Fatal("expected models key")
	}
	if _, ok := data["total_timeouts"]; !ok {
		t.Fatal("expected total_timeouts key")
	}
}

func TestTruncate(t *testing.T) {
	long := strings.Repeat("x", 100)
	got := truncate(long, 80)
	if len(got) != 80 {
		t.Fatalf("expected 80 chars, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatal("expected ... suffix")
	}
	short := "hello"
	if truncate(short, 80) != short {
		t.Fatal("short strings should not be truncated")
	}
}

func TestLabCount(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "lab-a", 100),
		mkFinding("m1", "lab-b", 200),
		mkFinding("m1", "lab-a", 300),
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
	if r.Models[0].SampleCount != 3 {
		t.Fatalf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestScoreRange(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "lab-1", 100),
		mkFinding("m1", "lab-2", 200),
	}
	r := Analyze(fs)
	score := r.Models[0].LatencyScore
	if score < 0 || score > 100 {
		t.Fatalf("score out of range: %.1f", score)
	}
}
