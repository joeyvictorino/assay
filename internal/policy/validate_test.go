package policy

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func validPolicy() model.Policy {
	return model.Policy{
		Version:       "1",
		DefaultEffect: model.EffectDeny,
		Agents: map[string]model.AgentBoundary{
			"probe": {
				MaxTier:            model.TierHigh,
				AllowedTools:       []string{"http_get"},
				MaxDelegationDepth: 0,
				Budget:             model.Budget{MaxUSD: 1},
			},
		},
		Rules: []model.Rule{
			{ID: "r1", Effect: model.EffectAllow, Match: model.RuleMatch{Tools: []string{"http_get"}}},
		},
	}
}

func TestValidateClean(t *testing.T) {
	issues := Validate(validPolicy())
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %v", len(issues), issues)
	}
}

func TestValidateMissingVersion(t *testing.T) {
	p := validPolicy()
	p.Version = ""
	issues := Validate(p)
	if len(issues) != 1 || issues[0].Field != "version" {
		t.Fatalf("expected version issue, got %v", issues)
	}
}

func TestValidateBadDefaultEffect(t *testing.T) {
	p := validPolicy()
	p.DefaultEffect = "maybe"
	issues := Validate(p)
	found := false
	for _, i := range issues {
		if i.Field == "default_effect" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected default_effect issue, got %v", issues)
	}
}

func TestValidateBadAgentTier(t *testing.T) {
	p := validPolicy()
	p.Agents["probe"] = model.AgentBoundary{MaxTier: "ultra"}
	issues := Validate(p)
	found := false
	for _, i := range issues {
		if i.Field == "agents.probe.max_tier" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected max_tier issue, got %v", issues)
	}
}

func TestValidateNegativeDelegationDepth(t *testing.T) {
	p := validPolicy()
	ab := p.Agents["probe"]
	ab.MaxDelegationDepth = -1
	p.Agents["probe"] = ab
	issues := Validate(p)
	found := false
	for _, i := range issues {
		if i.Field == "agents.probe.max_delegation_depth" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected delegation depth issue, got %v", issues)
	}
}

func TestValidateNegativeBudget(t *testing.T) {
	p := validPolicy()
	ab := p.Agents["probe"]
	ab.Budget.MaxUSD = -0.5
	p.Agents["probe"] = ab
	issues := Validate(p)
	found := false
	for _, i := range issues {
		if i.Field == "agents.probe.budget.max_usd" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected budget issue, got %v", issues)
	}
}

func TestValidateDuplicateRuleIDs(t *testing.T) {
	p := validPolicy()
	p.Rules = append(p.Rules, model.Rule{
		ID: "r1", Effect: model.EffectDeny, Match: model.RuleMatch{Agents: []string{"probe"}},
	})
	issues := Validate(p)
	found := false
	for _, i := range issues {
		if i.Field == "rules[1].id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected duplicate rule id issue, got %v", issues)
	}
}

func TestValidateEmptyRuleID(t *testing.T) {
	p := validPolicy()
	p.Rules[0].ID = ""
	issues := Validate(p)
	found := false
	for _, i := range issues {
		if i.Field == "rules[0].id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected empty rule id issue, got %v", issues)
	}
}

func TestValidateBadEffect(t *testing.T) {
	p := validPolicy()
	p.Rules[0].Effect = "permit"
	issues := Validate(p)
	found := false
	for _, i := range issues {
		if i.Field == "rules[0].effect" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected effect issue, got %v", issues)
	}
}

func TestValidateEmptyMatch(t *testing.T) {
	p := validPolicy()
	p.Rules[0].Match = model.RuleMatch{}
	issues := Validate(p)
	found := false
	for _, i := range issues {
		if i.Field == "rules[0].match" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected match issue, got %v", issues)
	}
}

func TestValidateSmallCombo(t *testing.T) {
	p := validPolicy()
	p.DangerousCombos = [][]string{{"only_one"}}
	issues := Validate(p)
	found := false
	for _, i := range issues {
		if i.Field == "dangerous_combinations[0]" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected combo issue, got %v", issues)
	}
}

func TestValidateNegativeWeight(t *testing.T) {
	p := validPolicy()
	p.Weights = map[string]float64{"severity": -1.0}
	issues := Validate(p)
	found := false
	for _, i := range issues {
		if i.Field == "weights.severity" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected weight issue, got %v", issues)
	}
}
