package policy

import (
	"context"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

// Adversarial suite adapted from the sovereign-assist assurance harness
// (same owner). Cases that belong to the scope gate (replay, expiry, session
// and target binding) are S3's and are not repeated here.
//
// The policy under test is inline so each case is self-contained.
const adversarialPolicy = `
version: "test-1"
default_effect: deny
dangerous_combinations:
  - [http.write, delegate]
  - [auth.login, report.external]
  - [fs.read, http.write]
agents:
  orchestrator:
    max_tier: medium
    allowed_tools: ["*"]
    max_delegation_depth: 0
    budget: {max_usd: 10, max_latency_ms: 1000}
  recon:
    max_tier: low
    allowed_tools: [http_get, inspect_headers, report_finding]
    max_delegation_depth: 1
    budget: {max_usd: 2, max_latency_ms: 1000}
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post, login_as, report_finding, delegate]
    max_delegation_depth: 3
    budget: {max_usd: 4, max_latency_ms: 1000}
  broke:
    max_tier: low
    allowed_tools: [http_get]
    max_delegation_depth: 1
    budget: {max_usd: 0, max_latency_ms: 1000}
rules:
  - id: deny-external-report
    effect: deny
    match: {capabilities: [report.external]}
    rationale: Results leave only through the committed pipeline
  - id: deny-recon-writes
    effect: deny
    match: {agents: [recon], capabilities: [http.write, auth.login]}
    rationale: Recon is read only
  - id: deny-prod-lab
    effect: deny
    match: {labs: [production]}
    rationale: Nothing runs against production
  - id: allow-read
    effect: allow
    match: {capabilities: [http.read, report], max_tier: medium}
    rationale: Read-only tools are the baseline
  - id: allow-probe-high
    effect: allow
    match: {agents: [probe], tools: [http_post, login_as], max_tier: high}
    rationale: Probe may exercise auth in the lab
  - id: allow-all-recon-tools
    effect: allow
    match: {agents: [recon]}
    rationale: Recon tools are pre-vetted
  - id: allow-delegate
    effect: allow
    match: {tools: [delegate]}
    rationale: Delegation for agents whose boundary permits it
`

func tool(name string, tier model.RiskTier, caps ...string) model.ToolManifest {
	return model.ToolManifest{Name: name, Version: "1", RiskTier: tier, Capabilities: caps}
}

func TestAdversarialCases(t *testing.T) {
	e, err := Parse([]byte(adversarialPolicy))
	if err != nil {
		t.Fatal(err)
	}
	okReq := func() model.PolicyRequest {
		return model.PolicyRequest{
			Agent:       "recon",
			Tool:        tool("http_get", model.TierLow, "http.read"),
			Lab:         "lab-01",
			Depth:       0,
			SpentUSD:    0.5,
			SignatureOK: true,
		}
	}

	cases := []struct {
		name      string
		req       func() model.PolicyRequest
		effect    model.Effect
		reason    string // exact reason, or prefix when reasonPrefix is set
		prefix    bool
		matched   []string
		rationale string // substring that must appear
	}{
		{
			name: "01 unsigned manifest",
			req: func() model.PolicyRequest {
				r := okReq()
				r.SignatureOK, r.SignatureReason = false, "UNSIGNED"
				return r
			},
			effect: model.EffectDeny, reason: "UNSIGNED", rationale: "no verified signature",
		},
		{
			name: "02 invalid signature (tampered manifest)",
			req: func() model.PolicyRequest {
				r := okReq()
				r.SignatureOK, r.SignatureReason = false, "SIGNATURE_INVALID"
				return r
			},
			effect: model.EffectDeny, reason: "SIGNATURE_INVALID",
		},
		{
			name: "03 wrong signer not in trust store",
			req: func() model.PolicyRequest {
				r := okReq()
				r.SignatureOK, r.SignatureReason = false, "UNKNOWN_SIGNER"
				return r
			},
			effect: model.EffectDeny, reason: "UNKNOWN_SIGNER",
		},
		{
			name: "04 signature never checked",
			req: func() model.PolicyRequest {
				r := okReq()
				r.SignatureOK = false
				return r
			},
			effect: model.EffectDeny, reason: ReasonSignatureUnverified,
		},
		{
			name: "05 signature flag cannot be overridden by allow rules",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "orchestrator"
				r.SignatureOK, r.SignatureReason = false, "UNSIGNED"
				return r
			},
			effect: model.EffectDeny, reason: "UNSIGNED",
		},
		{
			name: "06 unknown agent",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "intruder"
				return r
			},
			effect: model.EffectDeny, reason: ReasonUnknownAgent, rationale: `Agent "intruder"`,
		},
		{
			name: "07 empty agent id",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = ""
				return r
			},
			effect: model.EffectDeny, reason: ReasonUnknownAgent,
		},
		{
			name: "08 agent role confusion: agent name case variant",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "Recon"
				return r
			},
			effect: model.EffectDeny, reason: ReasonUnknownAgent,
		},
		{
			name: "09 tool not in agent allow list",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Tool = tool("login_as", model.TierLow, "http.read")
				return r
			},
			effect: model.EffectDeny, reason: ReasonToolNotAllowed, rationale: `"login_as"`,
		},
		{
			name: "10 dynamic tool discovery: unknown tool name",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Tool = tool("discover_tools", model.TierLow)
				return r
			},
			effect: model.EffectDeny, reason: ReasonToolNotAllowed,
		},
		{
			name: "11 wildcard allow list admits any tool name but not any tier",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "orchestrator"
				r.Tool = tool("brand.new", model.TierHigh, "http.read")
				return r
			},
			effect: model.EffectDeny, reason: ReasonTierExceeds,
		},
		{
			name: "12 manifest restricts agents",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Tool.AllowedAgents = []string{"probe"}
				return r
			},
			effect: model.EffectDeny, reason: ReasonAgentNotAllowed,
		},
		{
			name: "13 tier escalation above agent boundary",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Tool = tool("http_get", model.TierHigh, "http.read")
				return r
			},
			effect: model.EffectDeny, reason: ReasonTierExceeds, rationale: `at most tier "low"`,
		},
		{
			name: "14 missing or bogus tier fails closed",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Tool = tool("http_get", "", "http.read")
				return r
			},
			effect: model.EffectDeny, reason: ReasonTierExceeds,
		},
		{
			name: "15 dangerous combination http.write+delegate",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "probe"
				r.Tool = tool("delegate", model.TierMedium, "delegate", "http.write")
				return r
			},
			effect: model.EffectDeny, reason: "DANGEROUS_COMBINATION:delegate+http.write", rationale: "forbids",
		},
		{
			name: "16 dangerous combination auth.login+report.external beats rules",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "probe"
				r.Tool = tool("login_as", model.TierHigh, "auth.login", "report.external")
				return r
			},
			effect: model.EffectDeny, reason: "DANGEROUS_COMBINATION:auth.login+report.external",
		},
		{
			name: "17 dangerous combination fs.read+http.write hidden among benign caps",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "probe"
				r.Tool = tool("http_post", model.TierHigh, "http.read", "fs.read", "report", "http.write")
				return r
			},
			effect: model.EffectDeny, reason: "DANGEROUS_COMBINATION:fs.read+http.write",
		},
		{
			name: "18 delegation depth 4 exceeds probe maximum 3",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "probe"
				r.Depth = 4
				r.Ancestors = []string{"orchestrator", "probe-1", "probe-2", "probe-3"}
				return r
			},
			effect: model.EffectDeny, reason: ReasonDepthExceeded, rationale: "depth 4",
		},
		{
			name: "19 depth at the limit is allowed",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "probe"
				r.Depth = 3
				r.Ancestors = []string{"orchestrator", "x", "y"}
				return r
			},
			effect: model.EffectAllow, reason: "ALLOW_RULE:allow-read", matched: []string{"allow-read"},
		},
		{
			name: "20 orchestrator may not be delegated to at all",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "orchestrator"
				r.Depth = 1
				return r
			},
			effect: model.EffectDeny, reason: ReasonDepthExceeded,
		},
		{
			name: "21 delegation cycle: agent in its own ancestry",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "probe"
				r.Depth = 2
				r.Ancestors = []string{"orchestrator", "probe"}
				return r
			},
			effect: model.EffectDeny, reason: ReasonCycleDetected,
		},
		{
			name: "22 budget exhausted exactly at cap",
			req: func() model.PolicyRequest {
				r := okReq()
				r.SpentUSD = 2.0
				return r
			},
			effect: model.EffectDeny, reason: ReasonBudgetExceeded, rationale: "2.0000 USD",
		},
		{
			name: "23 budget exceeded",
			req: func() model.PolicyRequest {
				r := okReq()
				r.SpentUSD = 99
				return r
			},
			effect: model.EffectDeny, reason: ReasonBudgetExceeded,
		},
		{
			name: "24 zero budget denies the first call",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "broke"
				r.SpentUSD = 0
				return r
			},
			effect: model.EffectDeny, reason: ReasonBudgetExceeded,
		},
		{
			name: "25 deny rule wins over matching allow rules",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Tool = tool("http_get", model.TierLow, "http.read", "http.write")
				return r
			},
			effect: model.EffectDeny, reason: "DENY_RULE:deny-recon-writes",
			matched:   []string{"deny-recon-writes", "allow-read", "allow-all-recon-tools"},
			rationale: "Recon is read only",
		},
		{
			name: "26 lab-scoped deny rule",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Lab = "production"
				return r
			},
			effect: model.EffectDeny, reason: "DENY_RULE:deny-prod-lab",
		},
		{
			name: "27 default deny when nothing matches",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "probe"
				r.Tool = tool("report_finding", model.TierHigh, "report.local")
				return r
			},
			effect: model.EffectDeny, reason: ReasonDefaultDeny, rationale: "default is deny",
		},
		{
			name: "28 allow rule with tier ceiling does not cover a higher tier",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "probe"
				r.Tool = tool("http_get", model.TierHigh, "http.read")
				return r
			},
			effect: model.EffectDeny, reason: ReasonDefaultDeny,
		},
		{
			name: "29 allow rule for probe auth",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "probe"
				r.Tool = tool("login_as", model.TierHigh, "auth.login")
				return r
			},
			effect: model.EffectAllow, reason: "ALLOW_RULE:allow-probe-high", matched: []string{"allow-probe-high"},
		},
		{
			name: "30 external reporting denied for every agent",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "orchestrator"
				r.Tool = tool("report_finding", model.TierLow, "report", "report.external")
				return r
			},
			effect: model.EffectDeny, reason: "DENY_RULE:deny-external-report",
		},
		{
			name:   "31 valid baseline request",
			req:    okReq,
			effect: model.EffectAllow, reason: "ALLOW_RULE:allow-read",
			matched: []string{"allow-read", "allow-all-recon-tools"}, rationale: "baseline",
		},
		{
			name: "32 delegation allowed for probe within depth",
			req: func() model.PolicyRequest {
				r := okReq()
				r.Agent = "probe"
				r.Tool = tool("delegate", model.TierMedium, "delegate")
				r.Depth = 1
				r.Ancestors = []string{"orchestrator"}
				return r
			},
			effect: model.EffectAllow, reason: "ALLOW_RULE:allow-delegate",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := e.Evaluate(context.Background(), tc.req())
			if d.Effect != tc.effect {
				t.Fatalf("effect %q want %q (reason %s: %s)", d.Effect, tc.effect, d.Reason, d.Rationale)
			}
			if tc.prefix {
				if !strings.HasPrefix(d.Reason, tc.reason) {
					t.Fatalf("reason %q want prefix %q", d.Reason, tc.reason)
				}
			} else if d.Reason != tc.reason {
				t.Fatalf("reason %q want %q", d.Reason, tc.reason)
			}
			if d.PolicyHash != e.Hash() {
				t.Fatalf("policy hash %q", d.PolicyHash)
			}
			if d.Rationale == "" || !strings.HasSuffix(d.Rationale, ".") {
				t.Fatalf("rationale must be one sentence: %q", d.Rationale)
			}
			if tc.rationale != "" && !strings.Contains(d.Rationale, tc.rationale) {
				t.Fatalf("rationale %q lacks %q", d.Rationale, tc.rationale)
			}
			if tc.matched != nil && strings.Join(d.MatchedRules, ",") != strings.Join(tc.matched, ",") {
				t.Fatalf("matched %v want %v", d.MatchedRules, tc.matched)
			}
		})
	}
}

// TestEvaluationOrder pins the precedence: an earlier check's reason is
// reported even when later checks would also fail.
func TestEvaluationOrder(t *testing.T) {
	e, err := Parse([]byte(adversarialPolicy))
	if err != nil {
		t.Fatal(err)
	}
	// Everything wrong at once.
	r := model.PolicyRequest{
		Agent:       "intruder",
		Tool:        tool("rm.rf", model.TierCritical, "http.write", "delegate", "fs.read", "report.external"),
		Lab:         "production",
		Depth:       9,
		Ancestors:   []string{"intruder"},
		SpentUSD:    1e6,
		SignatureOK: false,
	}
	steps := []struct {
		fix    func()
		reason string
	}{
		{func() {}, ReasonSignatureUnverified},
		{func() { r.SignatureOK = true }, ReasonUnknownAgent},
		{func() { r.Agent = "probe"; r.Ancestors = []string{"probe"} }, ReasonToolNotAllowed},
		{func() { r.Tool.Name = "delegate"; r.Tool.AllowedAgents = []string{"orchestrator"} }, ReasonAgentNotAllowed},
		{func() { r.Tool.AllowedAgents = nil }, ReasonTierExceeds},
		{func() { r.Tool.RiskTier = model.TierMedium }, "DANGEROUS_COMBINATION:delegate+http.write"},
		{func() { r.Tool.Capabilities = []string{"delegate", "report.external"} }, ReasonDepthExceeded},
		{func() { r.Depth = 3 }, ReasonCycleDetected},
		{func() { r.Ancestors = []string{"orchestrator"} }, ReasonBudgetExceeded},
		{func() { r.SpentUSD = 0 }, "DENY_RULE:deny-external-report"},
		{func() { r.Tool.Capabilities = []string{"delegate"} }, "DENY_RULE:deny-prod-lab"},
		{func() { r.Lab = "lab-01" }, "ALLOW_RULE:allow-delegate"},
	}
	for i, s := range steps {
		s.fix()
		d := e.Evaluate(context.Background(), r)
		if d.Reason != s.reason {
			t.Fatalf("step %d: reason %q want %q (%s)", i, d.Reason, s.reason, d.Rationale)
		}
	}
}

func TestDefaultAllowPolicyStillDeniesBoundaries(t *testing.T) {
	y := strings.Replace(adversarialPolicy, "default_effect: deny", "default_effect: allow", 1)
	e, err := Parse([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	// No rule matches: default allow applies.
	d := e.Evaluate(context.Background(), model.PolicyRequest{
		Agent: "probe", SignatureOK: true, SpentUSD: 0,
		Tool: tool("report_finding", model.TierHigh, "report.local"),
	})
	if d.Effect != model.EffectAllow || d.Reason != ReasonDefaultAllow {
		t.Fatalf("%+v", d)
	}
	// Boundaries are still enforced before the default applies.
	d = e.Evaluate(context.Background(), model.PolicyRequest{
		Agent: "recon", SignatureOK: true,
		Tool: tool("http_get", model.TierCritical, "http.read"),
	})
	if d.Effect != model.EffectDeny || d.Reason != ReasonTierExceeds {
		t.Fatalf("%+v", d)
	}
}
