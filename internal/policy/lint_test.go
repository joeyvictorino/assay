package policy

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func policyYAML(yaml string) *Engine {
	e, err := Parse([]byte(yaml))
	if err != nil {
		panic(err)
	}
	return e
}

func TestLintClean(t *testing.T) {
	eng := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-probe
    effect: allow
    match:
      agents: [probe]
      tools: [http_get, http_post]
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow},
		{Name: "http_post", RiskTier: model.TierHigh},
	}
	issues := Lint(eng, manifests)
	if len(issues) != 0 {
		t.Fatalf("expected clean lint, got %d issues: %v", len(issues), issues)
	}
}

func TestLintUnknownToolInRule(t *testing.T) {
	eng := policyYAML(`
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
      tools: [nonexistent_tool]
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow},
	}
	issues := Lint(eng, manifests)
	found := false
	for _, iss := range issues {
		if iss.Kind == "unknown_tool_in_rule" && iss.RuleID == "r1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unknown_tool_in_rule issue, got %v", issues)
	}
}

func TestLintUnknownAgentInRule(t *testing.T) {
	eng := policyYAML(`
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
      agents: [ghost_agent]
`)
	issues := Lint(eng, nil)
	found := false
	for _, iss := range issues {
		if iss.Kind == "unknown_agent_in_rule" && iss.Severity == "error" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unknown_agent_in_rule error, got %v", issues)
	}
}

func TestLintUnknownToolInAgent(t *testing.T) {
	eng := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get, missing_tool]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow},
	}
	issues := Lint(eng, manifests)
	found := false
	for _, iss := range issues {
		if iss.Kind == "unknown_tool_in_agent" && iss.Agent == "probe" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unknown_tool_in_agent issue, got %v", issues)
	}
}

func TestLintShadowedAllow(t *testing.T) {
	eng := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: ["*"]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: deny-all-caps
    effect: deny
    match:
      capabilities: [http.write]
  - id: allow-probe-write
    effect: allow
    match:
      agents: [probe]
      capabilities: [http.write]
`)
	issues := Lint(eng, nil)
	found := false
	for _, iss := range issues {
		if iss.Kind == "shadowed_allow" && iss.RuleID == "allow-probe-write" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected shadowed_allow issue, got %v", issues)
	}
}

func TestLintWildcardAgentToolsSkipped(t *testing.T) {
	eng := policyYAML(`
version: "1"
default_effect: deny
agents:
  god:
    max_tier: high
    allowed_tools: ["*"]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
`)
	issues := Lint(eng, nil)
	for _, iss := range issues {
		if iss.Kind == "unknown_tool_in_agent" && iss.Agent == "god" {
			t.Fatalf("wildcard should not trigger unknown_tool_in_agent: %v", iss)
		}
	}
}

func TestLintAgentHasDangerousCombo(t *testing.T) {
	eng := policyYAML(`
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get, shell_exec]
    max_delegation_depth: 0
    budget: {max_usd: 1}
  safe:
    max_tier: low
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
dangerous_combinations:
  - [http.read, code.exec]
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow, Capabilities: []string{"http.read"}},
		{Name: "shell_exec", RiskTier: model.TierHigh, Capabilities: []string{"code.exec"}},
	}
	issues := Lint(eng, manifests)
	var found []LintIssue
	for _, iss := range issues {
		if iss.Kind == "agent_has_dangerous_combo" {
			found = append(found, iss)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 agent_has_dangerous_combo issue (probe only), got %d: %v", len(found), found)
	}
	if found[0].Agent != "probe" {
		t.Fatalf("expected agent=probe, got %q", found[0].Agent)
	}
}

func TestLintDangerousComboSafe(t *testing.T) {
	eng := policyYAML(`
version: "1"
default_effect: deny
agents:
  safe:
    max_tier: low
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
dangerous_combinations:
  - [http.read, code.exec]
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow, Capabilities: []string{"http.read"}},
	}
	issues := Lint(eng, manifests)
	for _, iss := range issues {
		if iss.Kind == "agent_has_dangerous_combo" {
			t.Fatalf("safe agent should not trigger: %v", iss)
		}
	}
}

func TestLintDangerousComboWildcard(t *testing.T) {
	eng := policyYAML(`
version: "1"
default_effect: deny
agents:
  god:
    max_tier: high
    allowed_tools: ["*"]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
dangerous_combinations:
  - [http.read, code.exec]
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow, Capabilities: []string{"http.read"}},
		{Name: "shell_exec", RiskTier: model.TierHigh, Capabilities: []string{"code.exec"}},
	}
	issues := Lint(eng, manifests)
	found := false
	for _, iss := range issues {
		if iss.Kind == "agent_has_dangerous_combo" && iss.Agent == "god" {
			found = true
		}
	}
	if !found {
		t.Fatalf("wildcard agent should flag dangerous combo when manifests provide the capabilities: %v", issues)
	}
}

func TestSubsumes(t *testing.T) {
	cases := []struct {
		name  string
		deny  model.RuleMatch
		allow model.RuleMatch
		want  bool
	}{
		{"empty deny subsumes anything", model.RuleMatch{}, model.RuleMatch{Tools: []string{"x"}}, true},
		{"non-empty deny cannot subsume empty allow", model.RuleMatch{Tools: []string{"x"}}, model.RuleMatch{}, false},
		{"exact match", model.RuleMatch{Agents: []string{"a"}}, model.RuleMatch{Agents: []string{"a"}}, true},
		{"deny superset of allow", model.RuleMatch{Agents: []string{"a", "b"}}, model.RuleMatch{Agents: []string{"a"}}, true},
		{"deny subset of allow", model.RuleMatch{Agents: []string{"a"}}, model.RuleMatch{Agents: []string{"a", "b"}}, false},
		{"tier: deny broader", model.RuleMatch{MaxTier: model.TierHigh}, model.RuleMatch{MaxTier: model.TierLow}, true},
		{"tier: deny narrower", model.RuleMatch{MaxTier: model.TierLow}, model.RuleMatch{MaxTier: model.TierHigh}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := subsumes(tc.deny, tc.allow)
			if got != tc.want {
				t.Fatalf("subsumes(%+v, %+v) = %v, want %v", tc.deny, tc.allow, got, tc.want)
			}
		})
	}
}
