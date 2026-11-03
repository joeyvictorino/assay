package policy

import (
	"strings"
	"testing"
)

func TestDiffIdentical(t *testing.T) {
	yaml := `
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-probe
    effect: allow
    match:
      agents: [probe]
`
	before := policyYAML(yaml)
	after := policyYAML(yaml)
	r := Diff(before, after)
	if !r.Identical {
		t.Fatal("expected identical policies")
	}
	if r.DefaultEffectChanged != nil {
		t.Fatal("no default_effect change expected")
	}
	if len(r.RulesAdded) > 0 || len(r.RulesRemoved) > 0 || len(r.RulesChanged) > 0 {
		t.Fatal("no rule changes expected")
	}
}

func TestDiffDefaultEffectChanged(t *testing.T) {
	before := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
`)
	after := policyYAML(`
version: "1"
default_effect: allow
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
`)
	r := Diff(before, after)
	if r.Identical {
		t.Fatal("should not be identical")
	}
	if r.DefaultEffectChanged == nil {
		t.Fatal("expected default_effect change")
	}
	if r.DefaultEffectChanged.Before != "deny" || r.DefaultEffectChanged.After != "allow" {
		t.Fatalf("expected deny->allow, got %s->%s", r.DefaultEffectChanged.Before, r.DefaultEffectChanged.After)
	}
}

func TestDiffRulesAddedRemoved(t *testing.T) {
	before := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-get
    effect: allow
    match:
      tools: [http_get]
  - id: deny-post
    effect: deny
    match:
      tools: [http_post]
`)
	after := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-get
    effect: allow
    match:
      tools: [http_get]
  - id: allow-all
    effect: allow
    match:
      agents: [probe]
`)
	r := Diff(before, after)
	if r.Identical {
		t.Fatal("should not be identical")
	}
	if len(r.RulesAdded) != 1 || r.RulesAdded[0].ID != "allow-all" {
		t.Fatalf("expected 1 added rule (allow-all), got %v", r.RulesAdded)
	}
	if len(r.RulesRemoved) != 1 || r.RulesRemoved[0].ID != "deny-post" {
		t.Fatalf("expected 1 removed rule (deny-post), got %v", r.RulesRemoved)
	}
}

func TestDiffRulesChanged(t *testing.T) {
	before := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: r1
    effect: allow
    match:
      tools: [http_get]
    rationale: original rationale
`)
	after := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: r1
    effect: deny
    match:
      agents: [probe]
    rationale: updated rationale
`)
	r := Diff(before, after)
	if r.Identical {
		t.Fatal("should not be identical")
	}
	if len(r.RulesChanged) != 1 {
		t.Fatalf("expected 1 changed rule, got %d", len(r.RulesChanged))
	}
	c := r.RulesChanged[0]
	if c.ID != "r1" {
		t.Fatalf("changed rule id = %q", c.ID)
	}
	if c.EffectChanged == nil || c.EffectChanged.Before != "allow" || c.EffectChanged.After != "deny" {
		t.Fatal("expected effect change allow->deny")
	}
	if !c.MatchChanged {
		t.Fatal("expected match change")
	}
	if c.RationaleBefore != "original rationale" || c.RationaleAfter != "updated rationale" {
		t.Fatal("expected rationale change")
	}
}

func TestDiffAgentsAddedRemovedChanged(t *testing.T) {
	before := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
  recon:
    max_tier: low
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 0.5}
rules: []
`)
	after := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: critical
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 2
    budget: {max_usd: 5}
  validate:
    max_tier: medium
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
`)
	r := Diff(before, after)
	if r.Identical {
		t.Fatal("should not be identical")
	}

	if len(r.AgentsAdded) != 1 || r.AgentsAdded[0].Name != "validate" {
		t.Fatalf("expected 1 added agent (validate), got %v", r.AgentsAdded)
	}
	if len(r.AgentsRemoved) != 1 || r.AgentsRemoved[0].Name != "recon" {
		t.Fatalf("expected 1 removed agent (recon), got %v", r.AgentsRemoved)
	}
	if len(r.AgentsChanged) != 1 || r.AgentsChanged[0].Name != "probe" {
		t.Fatalf("expected 1 changed agent (probe), got %v", r.AgentsChanged)
	}
	details := r.AgentsChanged[0].Details
	if len(details) != 4 {
		t.Fatalf("expected 4 detail lines, got %d: %v", len(details), details)
	}
}

func TestDiffCombosAddedRemoved(t *testing.T) {
	before := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
dangerous_combinations:
  - [http.write, auth.login]
rules: []
`)
	after := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
dangerous_combinations:
  - [http.read, report]
rules: []
`)
	r := Diff(before, after)
	if r.Identical {
		t.Fatal("should not be identical")
	}
	if len(r.CombosAdded) != 1 {
		t.Fatalf("expected 1 combo added, got %d", len(r.CombosAdded))
	}
	if len(r.CombosRemoved) != 1 {
		t.Fatalf("expected 1 combo removed, got %d", len(r.CombosRemoved))
	}
}

func TestDiffVersionChanged(t *testing.T) {
	before := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
`)
	after := policyYAML(`
version: "2"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
`)
	r := Diff(before, after)
	if r.Identical {
		t.Fatal("should not be identical")
	}
	if r.VersionChanged == nil || r.VersionChanged.Before != "1" || r.VersionChanged.After != "2" {
		t.Fatal("expected version 1->2")
	}
}

func TestFormatDiffTextIdentical(t *testing.T) {
	r := DiffReport{Identical: true}
	text := FormatDiffText(r)
	if text != "Policies are identical.\n" {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestFormatDiffTextChanges(t *testing.T) {
	before := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: r1
    effect: allow
    match:
      tools: [http_get]
`)
	after := policyYAML(`
version: "2"
default_effect: allow
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
  validate:
    max_tier: medium
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: r2
    effect: deny
    match:
      tools: [http_get]
`)
	r := Diff(before, after)
	text := FormatDiffText(r)
	if text == "Policies are identical.\n" {
		t.Fatal("should not be identical")
	}
	for _, want := range []string{"version:", "default_effect:", "Rules added:", "Rules removed:", "Agents added:"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected text to contain %q", want)
		}
	}
}
