package policy

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestExportMarkdownBasic(t *testing.T) {
	p := model.Policy{
		Version:       "2.1",
		DefaultEffect: model.EffectDeny,
		Agents: map[string]model.AgentBoundary{
			"recon": {
				MaxTier:            model.TierMedium,
				AllowedTools:       []string{"http_get", "dns_lookup"},
				MaxDelegationDepth: 1,
				Budget:             model.Budget{MaxUSD: 0.5, MaxLatencyMS: 30000, MaxTokens: 10000},
			},
			"probe": {
				MaxTier:            model.TierHigh,
				AllowedTools:       []string{"http_get", "http_post"},
				MaxDelegationDepth: 0,
				Budget:             model.Budget{MaxUSD: 2.0},
			},
		},
		Rules: []model.Rule{
			{
				ID: "allow-recon-read", Effect: model.EffectAllow,
				Match:     model.RuleMatch{Tools: []string{"http_get"}, Agents: []string{"recon"}},
				Rationale: "Recon agents need read access for discovery",
			},
			{
				ID: "deny-write-in-recon", Effect: model.EffectDeny,
				Match: model.RuleMatch{Capabilities: []string{"http.write"}, Agents: []string{"recon"}},
			},
		},
		DangerousCombos: [][]string{
			{"http_post", "exec_shell"},
			{"auth.login", "report"},
		},
		Weights: map[string]float64{
			"severity": 1.5,
			"coverage": 0.8,
		},
	}

	md := ExportMarkdown(p)

	checks := []struct {
		label string
		want  string
	}{
		{"version heading", "# Policy: 2.1"},
		{"default effect", "**Default effect:** `deny`"},
		{"agent table header", "| Agent | Max Tier |"},
		{"probe row", "| probe | high |"},
		{"recon row", "| recon | medium |"},
		{"sorted agents", "probe"},
		{"rule heading", "### `allow-recon-read` (allow)"},
		{"rationale", "Recon agents need read access"},
		{"match tools", "- Tools: `http_get`"},
		{"match agents", "- Agents: `recon`"},
		{"match capabilities", "- Capabilities: `http.write`"},
		{"combos section", "## Dangerous Combinations"},
		{"combo entry", "http_post + exec_shell"},
		{"weights section", "## Weights"},
		{"weight entry", "| severity | 1.5000 |"},
		{"recon tools", "http_get, dns_lookup"},
		{"max tokens", "10000"},
		{"max latency", "30000ms"},
	}
	for _, c := range checks {
		if !strings.Contains(md, c.want) {
			t.Errorf("%s: output should contain %q", c.label, c.want)
		}
	}
}

func TestExportMarkdownEmpty(t *testing.T) {
	p := model.Policy{
		Version:       "1",
		DefaultEffect: model.EffectAllow,
	}
	md := ExportMarkdown(p)
	if !strings.Contains(md, "# Policy: 1") {
		t.Error("should contain version heading")
	}
	if strings.Contains(md, "## Agent Boundaries") {
		t.Error("should not contain agent section when empty")
	}
	if strings.Contains(md, "## Rules") {
		t.Error("should not contain rules section when empty")
	}
}

func TestExportMarkdownWildcardTools(t *testing.T) {
	p := model.Policy{
		Version:       "1",
		DefaultEffect: model.EffectDeny,
		Agents: map[string]model.AgentBoundary{
			"admin": {MaxTier: model.TierCritical, Budget: model.Budget{MaxUSD: 10}},
		},
	}
	md := ExportMarkdown(p)
	if !strings.Contains(md, "| * |") {
		t.Error("empty allowed_tools should render as *")
	}
}
