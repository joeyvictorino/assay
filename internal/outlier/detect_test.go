package outlier

import (
	"math"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestDetectOutliersEmpty(t *testing.T) {
	r := DetectOutliers(model.RunReport{}, nil, 2.0)
	if len(r.Outliers) != 0 {
		t.Errorf("expected 0 outliers, got %d", len(r.Outliers))
	}
	if r.TotalPairs != 0 {
		t.Errorf("expected 0 pairs, got %d", r.TotalPairs)
	}
}

func TestDetectOutliersNoOutlier(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "l1", Model: "m1", Reviewer: "checker-a", Final: 7},
			{Lab: "l2", Model: "m2", Reviewer: "checker-a", Final: 7},
			{Lab: "l3", Model: "m3", Reviewer: "checker-a", Final: 7},
		},
	}
	r := DetectOutliers(rr, nil, 2.0)
	if len(r.Outliers) != 0 {
		t.Errorf("expected 0 outliers for identical scores, got %d", len(r.Outliers))
	}
}

func TestDetectOutliersWithOutlier(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "l1", Model: "m1", Reviewer: "checker-a", Final: 5},
			{Lab: "l2", Model: "m2", Reviewer: "checker-a", Final: 5},
			{Lab: "l3", Model: "m3", Reviewer: "checker-a", Final: 5},
			{Lab: "l4", Model: "m4", Reviewer: "checker-a", Final: 5},
			{Lab: "l5", Model: "m5", Reviewer: "checker-a", Final: 10},
		},
	}
	r := DetectOutliers(rr, nil, 1.5)
	if len(r.Outliers) == 0 {
		t.Error("expected at least 1 outlier for score 10 among 5s")
	}
	for _, o := range r.Outliers {
		if o.Model == "m5" && o.Type != "high" {
			t.Errorf("expected m5 to be 'high' outlier, got %q", o.Type)
		}
	}
}

func TestDefaultThreshold(t *testing.T) {
	r := DetectOutliers(model.RunReport{}, nil, 0)
	if r.Threshold != 2.0 {
		t.Errorf("expected default threshold 2.0, got %.1f", r.Threshold)
	}
}

func TestMeanStdDev(t *testing.T) {
	pairs := []scorePair{
		{"a", 2.0},
		{"b", 4.0},
		{"c", 4.0},
		{"d", 4.0},
		{"e", 5.0},
		{"f", 5.0},
		{"g", 7.0},
		{"h", 9.0},
	}
	mean, stddev := meanStdDev(pairs)
	if math.Abs(mean-5.0) > 0.001 {
		t.Errorf("mean = %.3f, want 5.0", mean)
	}
	if stddev < 1.0 || stddev > 3.0 {
		t.Errorf("stddev = %.3f, expected between 1.0 and 3.0", stddev)
	}
}

func TestSeverityScore(t *testing.T) {
	tests := []struct {
		sev  model.Severity
		want float64
	}{
		{model.SevCritical, 1.0},
		{model.SevHigh, 0.75},
		{model.SevMedium, 0.5},
		{model.SevLow, 0.25},
	}
	for _, tt := range tests {
		got := severityScore(tt.sev)
		if got != tt.want {
			t.Errorf("severityScore(%q) = %.2f, want %.2f", tt.sev, got, tt.want)
		}
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := OutlierReport{Threshold: 2.0}
	out := FormatText(r)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatTextWithOutliers(t *testing.T) {
	r := OutlierReport{
		Threshold:  2.0,
		TotalPairs: 10,
		OutlierPct: 10.0,
		Outliers: []Outlier{
			{Model: "m1", Checker: "c1", Score: 0.9, Mean: 0.5, StdDev: 0.1, ZScore: 4.0, Type: "high"},
		},
	}
	out := FormatText(r)
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestOutlierPercentage(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "l1", Model: "m1", Reviewer: "c", Final: 5},
			{Lab: "l2", Model: "m2", Reviewer: "c", Final: 5},
		},
	}
	r := DetectOutliers(rr, nil, 2.0)
	if r.TotalPairs != 2 {
		t.Errorf("expected 2 pairs, got %d", r.TotalPairs)
	}
}

func TestSinglePairNoOutlier(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "l1", Model: "m1", Reviewer: "checker-only", Final: 7},
		},
	}
	r := DetectOutliers(rr, nil, 2.0)
	if len(r.Outliers) != 0 {
		t.Errorf("single pair should produce no outliers, got %d", len(r.Outliers))
	}
}
