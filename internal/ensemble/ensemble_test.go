package ensemble

import (
	"encoding/json"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(class model.Class, method, path string, sev model.Severity) model.Finding {
	return model.Finding{
		DedupKey: string(class) + "|" + method + "|" + path,
		Class:    class,
		Location: model.Location{Method: method, PathTemplate: path},
		Severity: sev,
		Summary:  string(class) + " at " + method + " " + path,
	}
}

func TestMajorityStrategy(t *testing.T) {
	shared := makeFinding(model.ClassSQLi, "GET", "/api/users", model.SevHigh)
	unique := makeFinding(model.ClassXSSReflected, "POST", "/api/search", model.SevMedium)

	runs := []runData{
		{RunID: "run1", Model: "model-a", Findings: []model.Finding{shared, unique}},
		{RunID: "run2", Model: "model-b", Findings: []model.Finding{shared}},
		{RunID: "run3", Model: "model-c", Findings: []model.Finding{shared}},
	}

	r := AnalyzeRuns(runs, StratMajority, 0.5)
	if r.RunCount != 3 {
		t.Errorf("expected 3 runs, got %d", r.RunCount)
	}
	found := false
	for _, c := range r.Consensus {
		if c.Class == model.ClassSQLi {
			found = true
			if c.VoterCount != 3 {
				t.Errorf("expected 3 voters for SQLi, got %d", c.VoterCount)
			}
		}
	}
	if !found {
		t.Error("expected SQLi in consensus")
	}
}

func TestUnanimousStrategy(t *testing.T) {
	shared := makeFinding(model.ClassSQLi, "GET", "/api/users", model.SevHigh)
	partial := makeFinding(model.ClassXSSReflected, "POST", "/search", model.SevMedium)

	runs := []runData{
		{RunID: "run1", Model: "a", Findings: []model.Finding{shared, partial}},
		{RunID: "run2", Model: "b", Findings: []model.Finding{shared}},
	}

	r := AnalyzeRuns(runs, StratUnanimous, 0.5)
	for _, c := range r.Consensus {
		if c.Class == model.ClassXSSReflected {
			t.Error("XSS should not be in unanimous consensus (only 1/2 models)")
		}
	}
	sqliFound := false
	for _, c := range r.Consensus {
		if c.Class == model.ClassSQLi {
			sqliFound = true
		}
	}
	if !sqliFound {
		t.Error("SQLi should be in unanimous consensus")
	}
}

func TestWeightedStrategy(t *testing.T) {
	f := makeFinding(model.ClassIDOR, "GET", "/api/orders/{id}", model.SevHigh)

	runs := []runData{
		{RunID: "run1", Model: "a", Findings: []model.Finding{f}},
		{RunID: "run2", Model: "b", Findings: []model.Finding{f}},
		{RunID: "run3", Model: "c", Findings: []model.Finding{}},
		{RunID: "run4", Model: "d", Findings: []model.Finding{}},
	}

	r := AnalyzeRuns(runs, StratWeighted, 0.6)
	for _, c := range r.Consensus {
		if c.Class == model.ClassIDOR {
			t.Error("IDOR should be rejected at 0.6 threshold (only 50% agreement)")
		}
	}

	r2 := AnalyzeRuns(runs, StratWeighted, 0.4)
	found := false
	for _, c := range r2.Consensus {
		if c.Class == model.ClassIDOR {
			found = true
		}
	}
	if !found {
		t.Error("IDOR should pass at 0.4 threshold")
	}
}

func TestEmptyRuns(t *testing.T) {
	r := AnalyzeRuns(nil, StratMajority, 0.5)
	if len(r.Consensus) != 0 {
		t.Error("expected no consensus for empty runs")
	}
}

func TestSingleRun(t *testing.T) {
	f := makeFinding(model.ClassSQLi, "GET", "/users", model.SevHigh)
	runs := []runData{
		{RunID: "run1", Model: "a", Findings: []model.Finding{f}},
	}
	r := AnalyzeRuns(runs, StratMajority, 0.5)
	if len(r.Consensus) != 1 {
		t.Errorf("expected 1 consensus finding, got %d", len(r.Consensus))
	}
	if r.Consensus[0].Agreement != 1.0 {
		t.Errorf("expected 100%% agreement, got %f", r.Consensus[0].Agreement)
	}
}

func TestFormatText(t *testing.T) {
	f := makeFinding(model.ClassSQLi, "GET", "/users", model.SevHigh)
	runs := []runData{
		{RunID: "run1", Model: "a", Findings: []model.Finding{f}},
		{RunID: "run2", Model: "b", Findings: []model.Finding{f}},
	}
	r := AnalyzeRuns(runs, StratMajority, 0.5)
	text := FormatText(r)
	if text == "" {
		t.Error("expected non-empty text")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	f := makeFinding(model.ClassSQLi, "GET", "/users", model.SevHigh)
	runs := []runData{
		{RunID: "run1", Model: "a", Findings: []model.Finding{f}},
	}
	r := AnalyzeRuns(runs, StratMajority, 0.5)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.RunCount != r.RunCount {
		t.Error("run count mismatch after round-trip")
	}
}

func TestAgreementDistribution(t *testing.T) {
	f1 := makeFinding(model.ClassSQLi, "GET", "/a", model.SevHigh)
	f2 := makeFinding(model.ClassXSSReflected, "GET", "/b", model.SevMedium)
	runs := []runData{
		{RunID: "r1", Model: "a", Findings: []model.Finding{f1, f2}},
		{RunID: "r2", Model: "b", Findings: []model.Finding{f1}},
	}
	r := AnalyzeRuns(runs, StratMajority, 0.0)
	if len(r.AgreementDist) == 0 {
		t.Error("expected agreement distribution")
	}
}

func TestPicksBestSeverity(t *testing.T) {
	fLow := model.Finding{
		DedupKey: "sqli|GET|/x",
		Class:    model.ClassSQLi,
		Location: model.Location{Method: "GET", PathTemplate: "/x"},
		Severity: model.SevLow,
		Summary:  "low finding",
	}
	fHigh := model.Finding{
		DedupKey: "sqli|GET|/x",
		Class:    model.ClassSQLi,
		Location: model.Location{Method: "GET", PathTemplate: "/x"},
		Severity: model.SevHigh,
		Summary:  "high finding",
	}
	runs := []runData{
		{RunID: "r1", Model: "a", Findings: []model.Finding{fLow}},
		{RunID: "r2", Model: "b", Findings: []model.Finding{fHigh}},
	}
	r := AnalyzeRuns(runs, StratMajority, 0.5)
	if len(r.Consensus) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(r.Consensus))
	}
	if r.Consensus[0].Severity != model.SevHigh {
		t.Errorf("expected high severity, got %s", r.Consensus[0].Severity)
	}
}
