package policy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/tools"
	"github.com/joeyvictorino/assay/internal/toolsig"
)

// These tests evaluate the manifests and the policy that ship in the
// repository against each other, not fixtures. A manifest's allowed_agents is
// checked before any policy rule, so a manifest that names the wrong agents
// makes a rule that allows the right ones unreachable.

func defaultEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := Load(filepath.Join("..", "..", "policy", "default.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func evalCommitted(t *testing.T, e *Engine, agent, toolName string) model.Decision {
	t.Helper()
	m, err := tools.Manifest(toolName)
	if err != nil {
		t.Fatal(err)
	}
	return e.Evaluate(context.Background(), model.PolicyRequest{
		Agent: agent, Tool: m, SignatureOK: true, SignatureReason: "PROVENANCE_VALID",
	})
}

// The default policy lets the orchestrator, and only the orchestrator, fan
// work out (rule allow-orchestrator-delegate). The shipped delegate manifest
// used to list recon and probe instead, so the orchestrator was refused before
// the rule was reached while the two agents the policy denies were the ones
// the manifest named.
func TestOrchestratorMayDelegateUnderTheShippedManifestAndPolicy(t *testing.T) {
	d := evalCommitted(t, defaultEngine(t), "orchestrator", "delegate")
	if d.Effect != model.EffectAllow || d.Reason != "ALLOW_RULE:allow-orchestrator-delegate" {
		t.Fatalf("orchestrator delegate: %s %s (%s)", d.Effect, d.Reason, d.Rationale)
	}
}

func TestWorkerAgentsMayNotDelegate(t *testing.T) {
	e := defaultEngine(t)
	for _, agent := range []string{"recon", "probe", "validate", "remediate", "score"} {
		if d := evalCommitted(t, e, agent, "delegate"); d.Effect != model.EffectDeny {
			t.Errorf("%s may delegate: %s %s", agent, d.Effect, d.Reason)
		}
	}
}

// The manifest should not even be offered to the agents the policy forbids
// from using it: `assay run` hands each agent the manifests that list it.
func TestDelegateManifestNamesOnlyTheOrchestrator(t *testing.T) {
	m, err := tools.Manifest("delegate")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.AllowedAgents) != 1 || m.AllowedAgents[0] != "orchestrator" {
		t.Fatalf("delegate allowed_agents = %v, want [orchestrator]", m.AllowedAgents)
	}
}

// The four manifest/policy disagreements that were documented as known
// limitations: report_finding and score_task for the orchestrator, and
// http_get and report_finding for the remediate agent. The manifests now
// include those agents in allowed_agents, so the policy rules are reachable.
func TestFormerlyDisagreedManifestsNowAlign(t *testing.T) {
	e := defaultEngine(t)
	cases := []struct {
		agent, tool, wantReason string
	}{
		{"orchestrator", "report_finding", "ALLOW_RULE:allow-read-only-tools"},
		{"orchestrator", "score_task", "ALLOW_RULE:allow-scoring"},
		{"remediate", "http_get", "ALLOW_RULE:allow-read-only-tools"},
		{"remediate", "report_finding", "ALLOW_RULE:allow-read-only-tools"},
	}
	for _, tc := range cases {
		t.Run(tc.agent+"/"+tc.tool, func(t *testing.T) {
			d := evalCommitted(t, e, tc.agent, tc.tool)
			if d.Effect != model.EffectAllow || d.Reason != tc.wantReason {
				t.Fatalf("%s %s: %s %s (%s)", tc.agent, tc.tool, d.Effect, d.Reason, d.Rationale)
			}
		})
	}
}

// The remediate agent must not gain write or auth capabilities even though
// it can now use http_get and report_finding.
func TestRemediateStillDeniedWriteAndAuth(t *testing.T) {
	e := defaultEngine(t)
	for _, tool := range []string{"http_post", "login_as"} {
		d := evalCommitted(t, e, "remediate", tool)
		if d.Effect != model.EffectDeny {
			t.Errorf("remediate %s: %s %s", tool, d.Effect, d.Reason)
		}
	}
}

// Every shipped manifest, delegate included after it is re-signed, must still
// carry a valid signature from a key in trust/keys.
func TestShippedManifestsVerifyAgainstTheTrustStore(t *testing.T) {
	ts, err := toolsig.LoadTrustStore(filepath.Join("..", "..", "trust", "keys"))
	if err != nil {
		t.Fatal(err)
	}
	ms, err := tools.Manifests()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		if _, reason, err := ts.Verify(m); err != nil {
			t.Errorf("%s: %s: %v", m.Name, reason, err)
		}
	}
}
