package policy

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestMerge_Empty(t *testing.T) {
	out := Merge(nil)
	if len(out.Rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(out.Rules))
	}
}

func TestMerge_Single(t *testing.T) {
	p := model.Policy{
		Version:       "1",
		DefaultEffect: model.EffectAllow,
		Rules:         []model.Rule{{ID: "r1", Effect: model.EffectDeny}},
	}
	out := Merge([]model.Policy{p})
	if out.Version != "1" {
		t.Fatalf("expected version 1, got %s", out.Version)
	}
	if len(out.Rules) != 1 || out.Rules[0].ID != "r1" {
		t.Fatalf("expected 1 rule r1")
	}
}

func TestMerge_RulesOverride(t *testing.T) {
	base := model.Policy{
		Version:       "1",
		DefaultEffect: model.EffectDeny,
		Rules: []model.Rule{
			{ID: "r1", Effect: model.EffectDeny, Rationale: "base"},
			{ID: "r2", Effect: model.EffectDeny, Rationale: "base"},
		},
	}
	overlay := model.Policy{
		Version:       "2",
		DefaultEffect: model.EffectAllow,
		Rules: []model.Rule{
			{ID: "r1", Effect: model.EffectAllow, Rationale: "override"},
			{ID: "r3", Effect: model.EffectDeny, Rationale: "new"},
		},
	}
	out := Merge([]model.Policy{base, overlay})

	if out.Version != "2" {
		t.Errorf("expected version 2, got %s", out.Version)
	}
	if out.DefaultEffect != model.EffectAllow {
		t.Errorf("expected default_effect allow, got %s", out.DefaultEffect)
	}
	if len(out.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(out.Rules))
	}
	ruleMap := map[string]model.Rule{}
	for _, r := range out.Rules {
		ruleMap[r.ID] = r
	}
	if ruleMap["r1"].Rationale != "override" {
		t.Errorf("r1 should be overridden, got rationale=%s", ruleMap["r1"].Rationale)
	}
	if ruleMap["r2"].Rationale != "base" {
		t.Errorf("r2 should be from base, got rationale=%s", ruleMap["r2"].Rationale)
	}
	if ruleMap["r3"].Rationale != "new" {
		t.Errorf("r3 should be new, got rationale=%s", ruleMap["r3"].Rationale)
	}
}

func TestMerge_AgentsOverride(t *testing.T) {
	base := model.Policy{
		Version: "1",
		Agents: map[string]model.AgentBoundary{
			"a1": {MaxTier: "medium"},
			"a2": {MaxTier: "low"},
		},
	}
	overlay := model.Policy{
		Version: "2",
		Agents: map[string]model.AgentBoundary{
			"a1": {MaxTier: "high"},
			"a3": {MaxTier: "low"},
		},
	}
	out := Merge([]model.Policy{base, overlay})
	if len(out.Agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(out.Agents))
	}
	if out.Agents["a1"].MaxTier != "high" {
		t.Errorf("a1 should be high, got %s", out.Agents["a1"].MaxTier)
	}
	if out.Agents["a2"].MaxTier != "low" {
		t.Errorf("a2 should be low, got %s", out.Agents["a2"].MaxTier)
	}
}

func TestMerge_CombosUnion(t *testing.T) {
	base := model.Policy{
		Version:         "1",
		DangerousCombos: [][]string{{"shell", "http"}},
	}
	overlay := model.Policy{
		Version:         "2",
		DangerousCombos: [][]string{{"shell", "http"}, {"shell", "file_write"}},
	}
	out := Merge([]model.Policy{base, overlay})
	if len(out.DangerousCombos) != 2 {
		t.Fatalf("expected 2 combos (union), got %d", len(out.DangerousCombos))
	}
}

func TestMerge_WeightsOverride(t *testing.T) {
	base := model.Policy{
		Version: "1",
		Weights: map[string]float64{"w1": 1.0, "w2": 2.0},
	}
	overlay := model.Policy{
		Version: "2",
		Weights: map[string]float64{"w1": 5.0, "w3": 3.0},
	}
	out := Merge([]model.Policy{base, overlay})
	if out.Weights["w1"] != 5.0 {
		t.Errorf("w1 should be 5.0, got %f", out.Weights["w1"])
	}
	if out.Weights["w2"] != 2.0 {
		t.Errorf("w2 should be 2.0, got %f", out.Weights["w2"])
	}
	if out.Weights["w3"] != 3.0 {
		t.Errorf("w3 should be 3.0, got %f", out.Weights["w3"])
	}
}

func TestMerge_RuleOrder(t *testing.T) {
	p1 := model.Policy{Rules: []model.Rule{{ID: "a"}, {ID: "b"}}}
	p2 := model.Policy{Rules: []model.Rule{{ID: "c"}, {ID: "a"}}}
	out := Merge([]model.Policy{p1, p2})
	if len(out.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(out.Rules))
	}
	if out.Rules[0].ID != "a" || out.Rules[1].ID != "b" || out.Rules[2].ID != "c" {
		t.Errorf("expected order a,b,c but got %s,%s,%s", out.Rules[0].ID, out.Rules[1].ID, out.Rules[2].ID)
	}
}
