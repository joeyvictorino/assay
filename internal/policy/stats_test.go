package policy

import (
	"testing"
)

func TestStatsFromStrictTemplate(t *testing.T) {
	yaml, _ := GenerateTemplate("strict")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	s := Stats(eng.Policy())

	if s.RuleCount != 6 {
		t.Errorf("rule count = %d, want 6", s.RuleCount)
	}
	if s.RulesByEffect["allow"] != 3 {
		t.Errorf("allow rules = %d, want 3", s.RulesByEffect["allow"])
	}
	if s.RulesByEffect["deny"] != 3 {
		t.Errorf("deny rules = %d, want 3", s.RulesByEffect["deny"])
	}
	if s.AgentCount != 3 {
		t.Errorf("agent count = %d, want 3", s.AgentCount)
	}
	if s.DangerousComboCount != 3 {
		t.Errorf("dangerous combo count = %d, want 3", s.DangerousComboCount)
	}
	if s.WeightCount != 4 {
		t.Errorf("weight count = %d, want 4", s.WeightCount)
	}
	if len(s.Capabilities) == 0 {
		t.Error("should have capabilities")
	}
	if len(s.Tools) == 0 {
		t.Error("should have tools")
	}
}

func TestStatsMinimalTemplate(t *testing.T) {
	yaml, _ := GenerateTemplate("minimal")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	s := Stats(eng.Policy())

	if s.AgentCount != 1 {
		t.Errorf("agent count = %d, want 1", s.AgentCount)
	}
	if s.BudgetRange.MinUSD != s.BudgetRange.MaxUSD {
		t.Error("single agent should have equal min/max budget")
	}
	if s.DangerousComboCount != 0 {
		t.Errorf("minimal should have 0 dangerous combos, got %d", s.DangerousComboCount)
	}
	if s.WeightCount != 0 {
		t.Errorf("minimal should have 0 weights, got %d", s.WeightCount)
	}
}

func TestStatsBudgetRange(t *testing.T) {
	yaml, _ := GenerateTemplate("standard")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	s := Stats(eng.Policy())

	if s.BudgetRange.MinUSD >= s.BudgetRange.MaxUSD {
		t.Errorf("min USD %.2f should be less than max USD %.2f", s.BudgetRange.MinUSD, s.BudgetRange.MaxUSD)
	}
	if s.BudgetRange.MinLatencyMS >= s.BudgetRange.MaxLatencyMS {
		t.Errorf("min latency %d should be less than max latency %d", s.BudgetRange.MinLatencyMS, s.BudgetRange.MaxLatencyMS)
	}
}
