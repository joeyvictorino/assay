package ablation

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func f(id, dedup string, sev model.Severity) model.Finding {
	return model.Finding{ID: id, DedupKey: dedup, Severity: sev, State: model.StateValidated}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil, nil)
	if r.TotalFull != 0 {
		t.Fatalf("expected 0 full findings, got %d", r.TotalFull)
	}
	if len(r.Components) != 0 {
		t.Fatalf("expected 0 components, got %d", len(r.Components))
	}
}

func TestAnalyzeNoAblated(t *testing.T) {
	full := []model.Finding{f("f1", "d1", model.SevHigh)}
	r := Analyze(full, map[Component][]model.Finding{})
	if r.TotalFull != 1 {
		t.Fatalf("expected 1 full, got %d", r.TotalFull)
	}
	if len(r.Components) != 0 {
		t.Fatalf("expected 0 components, got %d", len(r.Components))
	}
}

func TestAnalyzeIdenticalResults(t *testing.T) {
	full := []model.Finding{f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevLow)}
	abl := map[Component][]model.Finding{
		CompExamples: {f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevLow)},
	}
	r := Analyze(full, abl)
	if len(r.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(r.Components))
	}
	c := r.Components[0]
	if c.FindingDelta != 0 {
		t.Errorf("expected delta 0, got %d", c.FindingDelta)
	}
	if len(c.LostFindings) != 0 {
		t.Errorf("expected no lost findings, got %v", c.LostFindings)
	}
	if len(c.NewFindings) != 0 {
		t.Errorf("expected no new findings, got %v", c.NewFindings)
	}
}

func TestAnalyzeLostFindings(t *testing.T) {
	full := []model.Finding{
		f("f1", "d1", model.SevHigh),
		f("f2", "d2", model.SevMedium),
		f("f3", "d3", model.SevLow),
	}
	abl := map[Component][]model.Finding{
		CompSystemPrompt: {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(full, abl)
	c := r.Components[0]
	if c.FindingDelta != -2 {
		t.Errorf("expected delta -2, got %d", c.FindingDelta)
	}
	if len(c.LostFindings) != 2 {
		t.Errorf("expected 2 lost findings, got %d", len(c.LostFindings))
	}
}

func TestAnalyzeNewFindings(t *testing.T) {
	full := []model.Finding{f("f1", "d1", model.SevHigh)}
	abl := map[Component][]model.Finding{
		CompConstraints: {
			f("f1", "d1", model.SevHigh),
			f("f2", "d2", model.SevCritical),
		},
	}
	r := Analyze(full, abl)
	c := r.Components[0]
	if len(c.NewFindings) != 1 {
		t.Errorf("expected 1 new finding, got %d", len(c.NewFindings))
	}
	if c.NewFindings[0] != "d2" {
		t.Errorf("expected new finding d2, got %s", c.NewFindings[0])
	}
}

func TestAnalyzeSeverityShift(t *testing.T) {
	full := []model.Finding{
		f("f1", "d1", model.SevHigh),
		f("f2", "d2", model.SevHigh),
	}
	abl := map[Component][]model.Finding{
		CompFormatting: {
			f("f1", "d1", model.SevLow),
			f("f2", "d2", model.SevHigh),
		},
	}
	r := Analyze(full, abl)
	c := r.Components[0]
	if len(c.SeverityShift) == 0 {
		t.Fatal("expected severity shifts")
	}
}

func TestAnalyzeMultipleComponents(t *testing.T) {
	full := []model.Finding{f("f1", "d1", model.SevHigh)}
	abl := map[Component][]model.Finding{
		CompSystemPrompt: {},
		CompExamples:     {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(full, abl)
	if len(r.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(r.Components))
	}
	if r.MostImpact != CompSystemPrompt {
		t.Errorf("expected most impact system-prompt, got %s", r.MostImpact)
	}
}

func TestAnalyzeImpactScoreOrder(t *testing.T) {
	full := []model.Finding{f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevLow)}
	abl := map[Component][]model.Finding{
		CompSystemPrompt: {},
		CompConstraints:  {f("f1", "d1", model.SevHigh)},
		CompExamples:     {f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevLow)},
	}
	r := Analyze(full, abl)
	for i := 1; i < len(r.Components); i++ {
		if r.Components[i].ImpactScore > r.Components[i-1].ImpactScore {
			t.Errorf("components not sorted by impact: %v > %v",
				r.Components[i].ImpactScore, r.Components[i-1].ImpactScore)
		}
	}
}

func TestAnalyzeDeltaPercent(t *testing.T) {
	full := []model.Finding{f("f1", "d1", model.SevHigh), f("f2", "d2", model.SevLow)}
	abl := map[Component][]model.Finding{
		CompContext: {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(full, abl)
	c := r.Components[0]
	if c.DeltaPercent != -50 {
		t.Errorf("expected -50%% delta, got %.1f%%", c.DeltaPercent)
	}
}

func TestAnalyzeDedupKeyFallback(t *testing.T) {
	full := []model.Finding{{ID: "f1", Severity: model.SevHigh}}
	abl := map[Component][]model.Finding{
		CompExamples: {{ID: "f1", Severity: model.SevHigh}},
	}
	r := Analyze(full, abl)
	c := r.Components[0]
	if len(c.LostFindings) != 0 {
		t.Errorf("expected no lost findings with ID fallback, got %v", c.LostFindings)
	}
}

func TestFormatTextOutput(t *testing.T) {
	full := []model.Finding{f("f1", "d1", model.SevHigh)}
	abl := map[Component][]model.Finding{
		CompSystemPrompt: {},
	}
	r := Analyze(full, abl)
	text := FormatText(r)
	if !strings.Contains(text, "Prompt Ablation") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "system-prompt") {
		t.Error("missing component name")
	}
	if !strings.Contains(text, "Most impactful") {
		t.Error("missing most impactful line")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil, nil)
	text := FormatText(r)
	if !strings.Contains(text, "No ablation data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	full := []model.Finding{f("f1", "d1", model.SevHigh)}
	abl := map[Component][]model.Finding{
		CompExamples: {f("f1", "d1", model.SevHigh)},
	}
	r := Analyze(full, abl)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.TotalFull != 1 {
		t.Errorf("expected total_full 1, got %d", parsed.TotalFull)
	}
}

func TestComputeImpactZero(t *testing.T) {
	score := computeImpact(0, 0, nil, nil)
	if score != 0 {
		t.Errorf("expected 0 impact, got %f", score)
	}
}

func TestAllComponentsOrdering(t *testing.T) {
	full := []model.Finding{f("f1", "d1", model.SevHigh)}
	abl := make(map[Component][]model.Finding)
	for _, c := range AllComponents {
		abl[c] = []model.Finding{f("f1", "d1", model.SevHigh)}
	}
	r := Analyze(full, abl)
	if len(r.Components) != len(AllComponents) {
		t.Errorf("expected %d components, got %d", len(AllComponents), len(r.Components))
	}
}

func TestMergeSevKeys(t *testing.T) {
	a := map[string]int{"high": 1, "low": 2}
	b := map[string]int{"medium": 1, "low": 3}
	keys := mergeSevKeys(a, b)
	if len(keys) != 3 {
		t.Errorf("expected 3 merged keys, got %d", len(keys))
	}
}
