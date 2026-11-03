package policy

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestBenchmarkStrictTemplate(t *testing.T) {
	yml, _ := GenerateTemplate("strict")
	eng, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	manifests := testManifests(eng)

	r := Benchmark(eng, manifests, 10)

	if r.TotalEvaluations == 0 {
		t.Error("expected evaluations > 0")
	}
	if r.AvgPerEvalUS <= 0 {
		t.Error("expected avg > 0")
	}
	if r.MinPerEvalUS > r.AvgPerEvalUS {
		t.Errorf("min %.2f should be <= avg %.2f", r.MinPerEvalUS, r.AvgPerEvalUS)
	}
	if r.AvgPerEvalUS > r.MaxPerEvalUS {
		t.Errorf("avg %.2f should be <= max %.2f", r.AvgPerEvalUS, r.MaxPerEvalUS)
	}
	if r.Agents != 3 {
		t.Errorf("agents = %d, want 3", r.Agents)
	}
	if r.Iterations != 10 {
		t.Errorf("iterations = %d, want 10", r.Iterations)
	}
}

func TestBenchmarkMinimalTemplate(t *testing.T) {
	yml, _ := GenerateTemplate("minimal")
	eng, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	manifests := []model.ToolManifest{
		{Name: "test-tool", Version: "1.0.0", RiskTier: model.TierLow, Capabilities: []string{"http.read"}},
	}

	r := Benchmark(eng, manifests, 5)

	if r.TotalEvaluations != r.Agents*r.Tools*r.Iterations {
		t.Errorf("evaluations = %d, want %d", r.TotalEvaluations, r.Agents*r.Tools*r.Iterations)
	}
	if r.EvalPerSecond <= 0 {
		t.Error("expected throughput > 0")
	}
}

func TestBenchmarkNoManifests(t *testing.T) {
	yml, _ := GenerateTemplate("minimal")
	eng, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}

	r := Benchmark(eng, nil, 10)

	if r.TotalEvaluations != 0 {
		t.Errorf("expected 0 evaluations with no manifests, got %d", r.TotalEvaluations)
	}
}

func TestBenchmarkFormatText(t *testing.T) {
	r := BenchmarkResult{
		TotalEvaluations: 300,
		TotalDurationMS:  15,
		AvgPerEvalUS:     50.00,
		MinPerEvalUS:     10.00,
		MaxPerEvalUS:     200.00,
		EvalPerSecond:    20000.00,
		Agents:           3,
		Tools:            10,
		Iterations:       10,
	}
	text := FormatBenchmarkText(r)
	if text == "" {
		t.Error("expected non-empty text")
	}
	for _, want := range []string{"Agents:", "Tools:", "Iterations:", "Evaluations:", "Avg:", "Min:", "Max:", "Throughput:"} {
		if !strings.Contains(text, want) {
			t.Errorf("text should contain %q", want)
		}
	}
}

func testManifests(eng *Engine) []model.ToolManifest {
	p := eng.Policy()
	toolSet := map[string]bool{}
	for _, name := range sortedAgentNames(p) {
		a := p.Agents[name]
		for _, t := range a.AllowedTools {
			toolSet[t] = true
		}
	}
	var ms []model.ToolManifest
	for t := range toolSet {
		if t == "*" {
			continue
		}
		ms = append(ms, model.ToolManifest{
			Name:         t,
			Version:      "1.0.0",
			RiskTier:     model.TierLow,
			Capabilities: []string{"http.read"},
		})
	}
	return ms
}

func sortedAgentNames(p model.Policy) []string {
	names := make([]string, 0, len(p.Agents))
	for n := range p.Agents {
		names = append(names, n)
	}
	return names
}
