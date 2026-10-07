package policy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

// defaultPath is the committed example policy, relative to this package.
const defaultPath = "../../policy/default.yaml"

func loadDefault(t *testing.T) *Engine {
	t.Helper()
	e, err := Load(defaultPath)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestLoadDefaultPolicy(t *testing.T) {
	e := loadDefault(t)
	if len(e.Hash()) != 64 {
		t.Fatalf("hash %q", e.Hash())
	}
	p := e.Policy()
	if p.DefaultEffect != model.EffectDeny {
		t.Fatalf("default effect %q", p.DefaultEffect)
	}
	if len(p.DangerousCombos) != 3 {
		t.Fatalf("combos %v", p.DangerousCombos)
	}
	if _, ok := p.Agents["recon"]; !ok {
		t.Fatal("recon agent missing")
	}
	var _ model.PolicyEvaluator = e

	// Hash is over the file bytes: a whitespace-only change is a new policy.
	raw, err := os.ReadFile(defaultPath)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := Parse(append(raw, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	if e2.Hash() == e.Hash() {
		t.Fatal("hash should change with file bytes")
	}
}

func TestLoadFailsClosed(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"empty", ""},
		{"whitespace", "   \n"},
		{"not yaml", "version: [unclosed"},
		{"unknown top-level key", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}}}\nextra: 1\n"},
		{"unknown agent key", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}, typo: 1}}\n"},
		{"missing version", "default_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}}}\n"},
		{"bad default effect", "version: '1'\ndefault_effect: maybe\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}}}\n"},
		{"no agents", "version: '1'\ndefault_effect: deny\nagents: {}\n"},
		{"bad agent tier", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: extreme, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}}}\n"},
		{"negative depth", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: -1, budget: {max_usd: 1, max_latency_ms: 1}}}\n"},
		{"negative budget", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: -1, max_latency_ms: 1}}}\n"},
		{"rule without id", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}}}\nrules: [{effect: deny, match: {}, rationale: x}]\n"},
		{"duplicate rule id", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}}}\nrules: [{id: r, effect: deny, match: {}, rationale: x}, {id: r, effect: allow, match: {}, rationale: y}]\n"},
		{"bad rule effect", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}}}\nrules: [{id: r, effect: permit, match: {}, rationale: x}]\n"},
		{"bad rule tier", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}}}\nrules: [{id: r, effect: deny, match: {max_tier: huge}, rationale: x}]\n"},
		{"single-item combo", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}}}\ndangerous_combinations: [[delegate]]\n"},
		{"empty capability in combo", "version: '1'\ndefault_effect: deny\nagents: {a: {max_tier: low, allowed_tools: [], max_delegation_depth: 0, budget: {max_usd: 1, max_latency_ms: 1}}}\ndangerous_combinations: [[delegate, '']]\n"},
		{"wrong type", "version: '1'\ndefault_effect: deny\nagents: [a, b]\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.yaml)); !errors.Is(err, ErrInvalidPolicy) {
				t.Fatalf("err = %v, want ErrInvalidPolicy", err)
			}
		})
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing file should error")
	}
}

func TestRuleMatchSemantics(t *testing.T) {
	e := loadDefault(t)
	base := model.PolicyRequest{Agent: "probe", Lab: "lab-a", Tool: model.ToolManifest{Name: "http.get", RiskTier: model.TierLow, Capabilities: []string{"http.read"}}}
	cases := []struct {
		name  string
		match model.RuleMatch
		want  bool
	}{
		{"empty matches all", model.RuleMatch{}, true},
		{"tool listed", model.RuleMatch{Tools: []string{"x", "http.get"}}, true},
		{"tool not listed", model.RuleMatch{Tools: []string{"x"}}, false},
		{"agent listed", model.RuleMatch{Agents: []string{"probe"}}, true},
		{"agent not listed", model.RuleMatch{Agents: []string{"recon"}}, false},
		{"lab listed", model.RuleMatch{Labs: []string{"lab-a"}}, true},
		{"lab not listed", model.RuleMatch{Labs: []string{"lab-b"}}, false},
		{"capability intersects", model.RuleMatch{Capabilities: []string{"delegate", "http.read"}}, true},
		{"capability disjoint", model.RuleMatch{Capabilities: []string{"delegate"}}, false},
		{"tier at ceiling", model.RuleMatch{MaxTier: model.TierLow}, true},
		{"tier below ceiling", model.RuleMatch{MaxTier: model.TierHigh}, true},
		{"all selectors must hold", model.RuleMatch{Tools: []string{"http.get"}, Agents: []string{"recon"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := e.matches(tc.match, base); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	high := base
	high.Tool.RiskTier = model.TierCritical
	if e.matches(model.RuleMatch{MaxTier: model.TierHigh}, high) {
		t.Fatal("critical tool should not match max_tier high")
	}
}

func TestDecisionShape(t *testing.T) {
	e := loadDefault(t)
	d := e.Evaluate(context.Background(), model.PolicyRequest{
		Agent: "recon", SignatureOK: true,
		Tool: model.ToolManifest{Name: "http.get", RiskTier: model.TierLow, Capabilities: []string{"http.read"}},
	})
	if d.Effect != model.EffectAllow || d.Reason != "ALLOW_RULE:allow-read-only-tools" {
		t.Fatalf("%+v", d)
	}
	if d.PolicyHash != e.Hash() {
		t.Fatal("policy hash not set")
	}
	if !strings.HasSuffix(d.Rationale, ".") || strings.Count(d.Rationale, ". ") != 0 {
		t.Fatalf("rationale should be one sentence: %q", d.Rationale)
	}
	if len(d.MatchedRules) != 1 || d.MatchedRules[0] != "allow-read-only-tools" {
		t.Fatalf("matched %v", d.MatchedRules)
	}
}

func TestDangerousCombinationIsSortedAndFirstMatchWins(t *testing.T) {
	e := loadDefault(t)
	got := e.dangerousCombination([]string{"delegate", "http.write", "fs.read"})
	if strings.Join(got, "+") != "delegate+http.write" {
		t.Fatalf("got %v", got)
	}
	if e.dangerousCombination([]string{"http.write"}) != nil {
		t.Fatal("single capability is not a combination")
	}
	if e.dangerousCombination(nil) != nil {
		t.Fatal("nil capabilities")
	}
}
