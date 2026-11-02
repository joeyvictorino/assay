package policy

import (
	"strings"
	"testing"
)

func TestGraphDOTStandard(t *testing.T) {
	yaml, _ := GenerateTemplate("standard")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	dot := GraphDOT(eng.Policy())

	if !strings.Contains(dot, "digraph policy") {
		t.Error("DOT should start with digraph")
	}
	for _, agent := range []string{"recon", "probe", "validate"} {
		if !strings.Contains(dot, `"agent:`+agent+`"`) {
			t.Errorf("DOT should contain agent node %q", agent)
		}
	}
	for _, tool := range []string{"http_get", "http_post"} {
		if !strings.Contains(dot, `"tool:`+tool+`"`) {
			t.Errorf("DOT should contain tool node %q", tool)
		}
	}
	if !strings.Contains(dot, "color=green") {
		t.Error("DOT should have green (allow) edges")
	}
	if !strings.Contains(dot, "color=red") {
		t.Error("DOT should have red (deny) edges")
	}
	if !strings.Contains(dot, "dangerous") {
		t.Error("DOT should have dangerous combo edges")
	}
	if !strings.Contains(dot, "shape=box") {
		t.Error("DOT should have box-shaped agent nodes")
	}
	if !strings.Contains(dot, "shape=ellipse") {
		t.Error("DOT should have ellipse-shaped tool nodes")
	}
}

func TestGraphDOTMinimal(t *testing.T) {
	yaml, _ := GenerateTemplate("minimal")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	dot := GraphDOT(eng.Policy())

	if !strings.Contains(dot, `"agent:default"`) {
		t.Error("DOT should contain agent:default")
	}
	if strings.Contains(dot, "dangerous") {
		t.Error("minimal template has no dangerous combos")
	}
}

func TestGraphMermaidStandard(t *testing.T) {
	yaml, _ := GenerateTemplate("standard")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	mm := GraphMermaid(eng.Policy())

	if !strings.Contains(mm, "flowchart LR") {
		t.Error("Mermaid should start with flowchart LR")
	}
	for _, agent := range []string{"recon", "probe", "validate"} {
		if !strings.Contains(mm, "agent_"+agent) {
			t.Errorf("Mermaid should contain agent node %q", agent)
		}
	}
	if !strings.Contains(mm, "-->|") {
		t.Error("Mermaid should have labeled edges")
	}
	if !strings.Contains(mm, "-.->|dangerous|") {
		t.Error("Mermaid should have dashed dangerous combo edges")
	}
}

func TestGraphMermaidMinimal(t *testing.T) {
	yaml, _ := GenerateTemplate("minimal")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	mm := GraphMermaid(eng.Policy())

	if !strings.Contains(mm, "agent_default") {
		t.Error("Mermaid should contain agent_default")
	}
}

func TestGraphDOTStrict(t *testing.T) {
	yaml, _ := GenerateTemplate("strict")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	dot := GraphDOT(eng.Policy())

	greenCount := strings.Count(dot, "color=green")
	redCount := strings.Count(dot, "color=red")
	if greenCount == 0 || redCount == 0 {
		t.Errorf("strict template should have both allow and deny edges (green=%d, red=%d)", greenCount, redCount)
	}
	dangerousCount := strings.Count(dot, "dangerous")
	if dangerousCount < 3 {
		t.Errorf("strict template has 3 dangerous combos, got %d dangerous edges", dangerousCount)
	}
}
