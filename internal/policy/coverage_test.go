package policy

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestCoverageFullCoverage(t *testing.T) {
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
  - id: allow-all
    effect: allow
    match:
      agents: [probe]
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow},
		{Name: "http_post", RiskTier: model.TierHigh},
	}
	r := Coverage(eng, manifests)
	if r.TotalPairs != 2 {
		t.Fatalf("total = %d, want 2", r.TotalPairs)
	}
	if r.CoveredPairs != 2 {
		t.Fatalf("covered = %d, want 2", r.CoveredPairs)
	}
	if r.Percentage != 100 {
		t.Fatalf("pct = %.1f, want 100", r.Percentage)
	}
	if len(r.Uncovered) != 0 {
		t.Fatalf("expected no uncovered pairs, got %v", r.Uncovered)
	}
}

func TestCoveragePartial(t *testing.T) {
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
  - id: allow-get
    effect: allow
    match:
      tools: [http_get]
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow},
		{Name: "http_post", RiskTier: model.TierHigh},
	}
	r := Coverage(eng, manifests)
	if r.TotalPairs != 2 {
		t.Fatalf("total = %d, want 2", r.TotalPairs)
	}
	if r.CoveredPairs != 1 {
		t.Fatalf("covered = %d, want 1", r.CoveredPairs)
	}
	if len(r.Uncovered) != 1 {
		t.Fatalf("expected 1 uncovered pair, got %d", len(r.Uncovered))
	}
	if r.Uncovered[0].Tool != "http_post" {
		t.Fatalf("uncovered tool = %q, want http_post", r.Uncovered[0].Tool)
	}
	if r.Uncovered[0].Effect != "deny" {
		t.Fatalf("uncovered effect = %q, want deny", r.Uncovered[0].Effect)
	}
}

func TestCoverageWildcardAgent(t *testing.T) {
	eng := policyYAML(`
version: "1"
default_effect: allow
agents:
  god:
    max_tier: high
    allowed_tools: ["*"]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: deny-post
    effect: deny
    match:
      tools: [http_post]
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow},
		{Name: "http_post", RiskTier: model.TierHigh},
	}
	r := Coverage(eng, manifests)
	if r.TotalPairs != 2 {
		t.Fatalf("total = %d, want 2", r.TotalPairs)
	}
	if r.CoveredPairs != 1 {
		t.Fatalf("covered = %d, want 1 (deny-post matches http_post)", r.CoveredPairs)
	}
	if len(r.Uncovered) != 1 || r.Uncovered[0].Tool != "http_get" {
		t.Fatalf("expected http_get uncovered with default allow, got %v", r.Uncovered)
	}
}

func TestCoverageMultiAgent(t *testing.T) {
	eng := policyYAML(`
version: "1"
default_effect: deny
agents:
  recon:
    max_tier: low
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-read
    effect: allow
    match:
      capabilities: [http.read]
      max_tier: low
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow, Capabilities: []string{"http.read"}},
		{Name: "http_post", RiskTier: model.TierHigh, Capabilities: []string{"http.read", "http.write"}},
	}
	r := Coverage(eng, manifests)
	// recon: http_get (covered by allow-read)
	// probe: http_get (covered by allow-read), http_post (NOT covered — tier high > max_tier low)
	if r.TotalPairs != 3 {
		t.Fatalf("total = %d, want 3", r.TotalPairs)
	}
	if r.CoveredPairs != 2 {
		t.Fatalf("covered = %d, want 2", r.CoveredPairs)
	}
	if len(r.Uncovered) != 1 {
		t.Fatalf("expected 1 uncovered, got %d: %v", len(r.Uncovered), r.Uncovered)
	}
	if r.Uncovered[0].Agent != "probe" || r.Uncovered[0].Tool != "http_post" {
		t.Fatalf("wrong uncovered pair: %v", r.Uncovered[0])
	}
}

func TestCoverageEmptyManifests(t *testing.T) {
	eng := policyYAML(`
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
	r := Coverage(eng, nil)
	if r.TotalPairs != 0 {
		t.Fatalf("expected 0 pairs with no manifests, got %d", r.TotalPairs)
	}
	if r.Percentage != 0 {
		t.Fatalf("expected 0%% coverage, got %.1f%%", r.Percentage)
	}
}

func TestCoverageAgentRulesPopulated(t *testing.T) {
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
  - id: allow-probe
    effect: allow
    match:
      agents: [probe]
  - id: deny-post
    effect: deny
    match:
      tools: [http_post]
`)
	manifests := []model.ToolManifest{
		{Name: "http_get", RiskTier: model.TierLow},
	}
	r := Coverage(eng, manifests)
	if len(r.Agents) != 1 {
		t.Fatalf("expected 1 agent in report, got %d", len(r.Agents))
	}
	ac := r.Agents[0]
	if ac.Agent != "probe" {
		t.Fatalf("agent = %q", ac.Agent)
	}
	if len(ac.Rules) != 1 || ac.Rules[0] != "allow-probe" {
		t.Fatalf("expected rules=[allow-probe], got %v", ac.Rules)
	}
}
