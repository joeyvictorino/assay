// Package policy loads the deny-wins policy document and evaluates tool
// requests against it.
//
// Evaluation is a fixed sequence of checks, each with its own reason code,
// so that an audit record of a denial says exactly which boundary was
// crossed. The structure (reason codes, dangerous capability combinations,
// default deny) is ported from the sovereign-assist controller (same owner).
package policy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/joeyvictorino/assay/internal/model"
)

// Reason codes emitted by Evaluate. Rule-derived codes carry a suffix:
// DENY_RULE:<id>, ALLOW_RULE:<id>, DANGEROUS_COMBINATION:<a>+<b>.
const (
	ReasonUnknownAgent         = "UNKNOWN_AGENT"
	ReasonToolNotAllowed       = "TOOL_NOT_ALLOWED_FOR_AGENT"
	ReasonAgentNotAllowed      = "AGENT_NOT_ALLOWED_FOR_TOOL"
	ReasonTierExceeds          = "TIER_EXCEEDS_AGENT_BOUNDARY"
	ReasonDangerousCombination = "DANGEROUS_COMBINATION"
	ReasonDepthExceeded        = "DEPTH_EXCEEDED"
	ReasonCycleDetected        = "CYCLE_DETECTED"
	ReasonBudgetExceeded       = "BUDGET_EXCEEDED"
	ReasonDenyRule             = "DENY_RULE"
	ReasonAllowRule            = "ALLOW_RULE"
	ReasonDefaultDeny          = "DEFAULT_DENY"
	ReasonDefaultAllow         = "DEFAULT_ALLOW"
	ReasonSignatureUnverified  = "SIGNATURE_UNVERIFIED"
)

// Wildcard in an agent's allowed_tools admits every tool name.
const Wildcard = "*"

// ErrInvalidPolicy wraps every validation failure of a policy document.
var ErrInvalidPolicy = errors.New("policy: invalid policy")

// Engine is an immutable, loaded policy.
type Engine struct {
	p    model.Policy
	hash string
}

// Load reads a YAML policy file. Any parse or validation problem is an
// error; there is no partially loaded engine.
func Load(path string) (*Engine, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("policy: read %s: %w", path, err)
	}
	e, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("policy: %s: %w", path, err)
	}
	return e, nil
}

// Parse builds an Engine from YAML (or JSON, which is valid YAML) bytes.
// Field names follow the json tags of model.Policy. Unknown fields are
// rejected so that a misspelled key cannot silently widen the policy.
func Parse(raw []byte) (*Engine, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("%w: empty document", ErrInvalidPolicy)
	}
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("%w: yaml: %v", ErrInvalidPolicy, err)
	}
	// yaml.v3 yields map[string]any for string-keyed maps, so a JSON
	// round-trip applies model.Policy's json tags and strictness.
	js, err := json.Marshal(generic)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPolicy, err)
	}
	dec := json.NewDecoder(bytes.NewReader(js))
	dec.DisallowUnknownFields()
	var p model.Policy
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPolicy, err)
	}
	if err := validate(p); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return &Engine{p: p, hash: hex.EncodeToString(sum[:])}, nil
}

func validEffect(e model.Effect) bool { return e == model.EffectAllow || e == model.EffectDeny }

func validTier(t model.RiskTier) bool { return t.Rank() > 0 }

func validate(p model.Policy) error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidPolicy, fmt.Sprintf(format, args...))
	}
	if strings.TrimSpace(p.Version) == "" {
		return bad("version is required")
	}
	if !validEffect(p.DefaultEffect) {
		return bad("default_effect must be allow or deny, got %q", p.DefaultEffect)
	}
	if len(p.Agents) == 0 {
		return bad("at least one agent boundary is required")
	}
	for name, a := range p.Agents {
		if strings.TrimSpace(name) == "" {
			return bad("agent name is empty")
		}
		if !validTier(a.MaxTier) {
			return bad("agent %q: max_tier %q is not a risk tier", name, a.MaxTier)
		}
		if a.MaxDelegationDepth < 0 {
			return bad("agent %q: max_delegation_depth must be >= 0", name)
		}
		if a.Budget.MaxUSD < 0 {
			return bad("agent %q: budget.max_usd must be >= 0", name)
		}
	}
	seen := map[string]bool{}
	for i, r := range p.Rules {
		if strings.TrimSpace(r.ID) == "" {
			return bad("rule %d has no id", i)
		}
		if seen[r.ID] {
			return bad("duplicate rule id %q", r.ID)
		}
		seen[r.ID] = true
		if !validEffect(r.Effect) {
			return bad("rule %q: effect must be allow or deny", r.ID)
		}
		if r.Match.MaxTier != "" && !validTier(r.Match.MaxTier) {
			return bad("rule %q: match.max_tier %q is not a risk tier", r.ID, r.Match.MaxTier)
		}
	}
	for i, combo := range p.DangerousCombos {
		if len(combo) < 2 {
			return bad("dangerous_combinations[%d] needs at least two capabilities", i)
		}
		for _, c := range combo {
			if strings.TrimSpace(c) == "" {
				return bad("dangerous_combinations[%d] has an empty capability", i)
			}
		}
	}
	return nil
}

// Hash is the hex SHA-256 of the policy file bytes as loaded. Every
// Decision carries it so an audit record can be tied to the exact policy
// text in force.
func (e *Engine) Hash() string { return e.hash }

// Policy returns a copy of the loaded document.
func (e *Engine) Policy() model.Policy { return e.p }

// Evaluate implements model.PolicyEvaluator. Checks run in a fixed order and
// the first failing check decides; deny rules always beat allow rules.
func (e *Engine) Evaluate(_ context.Context, req model.PolicyRequest) model.Decision {
	deny := func(reason, rationale string, matched ...string) model.Decision {
		return model.Decision{Effect: model.EffectDeny, Reason: reason, Rationale: rationale, MatchedRules: matched, PolicyHash: e.hash}
	}
	tool := req.Tool.Name

	if !req.SignatureOK {
		reason := req.SignatureReason
		if reason == "" {
			reason = ReasonSignatureUnverified
		}
		return deny(reason, fmt.Sprintf("Tool %q has no verified signature (%s), so nothing about it can be trusted.", tool, reason))
	}

	agent, ok := e.p.Agents[req.Agent]
	if !ok {
		return deny(ReasonUnknownAgent, fmt.Sprintf("Agent %q has no boundary in the policy, so it may not run any tool.", req.Agent))
	}

	if !toolAllowed(agent.AllowedTools, tool) {
		return deny(ReasonToolNotAllowed, fmt.Sprintf("Agent %q is not allowed to use tool %q.", req.Agent, tool))
	}

	if len(req.Tool.AllowedAgents) > 0 && !contains(req.Tool.AllowedAgents, req.Agent) {
		return deny(ReasonAgentNotAllowed, fmt.Sprintf("Tool %q restricts itself to agents %s, which excludes %q.", tool, strings.Join(req.Tool.AllowedAgents, ","), req.Agent))
	}

	if req.Tool.RiskTier.Rank() == 0 || req.Tool.RiskTier.Rank() > agent.MaxTier.Rank() {
		return deny(ReasonTierExceeds, fmt.Sprintf("Tool %q is tier %q but agent %q may use at most tier %q.", tool, req.Tool.RiskTier, req.Agent, agent.MaxTier))
	}

	if combo := e.dangerousCombination(req.Tool.Capabilities); combo != nil {
		return deny(ReasonDangerousCombination+":"+strings.Join(combo, "+"),
			fmt.Sprintf("Tool %q combines capabilities %s, which the policy forbids in one tool.", tool, strings.Join(combo, " and ")))
	}

	if req.Depth > agent.MaxDelegationDepth {
		return deny(ReasonDepthExceeded, fmt.Sprintf("Delegation depth %d exceeds agent %q's maximum of %d.", req.Depth, req.Agent, agent.MaxDelegationDepth))
	}

	if contains(req.Ancestors, req.Agent) {
		return deny(ReasonCycleDetected, fmt.Sprintf("Agent %q already appears in its own delegation chain.", req.Agent))
	}

	if req.SpentUSD >= agent.Budget.MaxUSD {
		return deny(ReasonBudgetExceeded, fmt.Sprintf("Agent %q has spent %.4f USD of a %.4f USD budget.", req.Agent, req.SpentUSD, agent.Budget.MaxUSD))
	}

	var denies, allows []string
	for _, r := range e.p.Rules {
		if !e.matches(r.Match, req) {
			continue
		}
		if r.Effect == model.EffectDeny {
			denies = append(denies, r.ID)
		} else {
			allows = append(allows, r.ID)
		}
	}
	matched := append(append([]string{}, denies...), allows...)
	if len(denies) > 0 {
		r := e.rule(denies[0])
		return deny(ReasonDenyRule+":"+r.ID, ruleRationale(r, "denies"), matched...)
	}
	if len(allows) > 0 {
		r := e.rule(allows[0])
		return model.Decision{Effect: model.EffectAllow, Reason: ReasonAllowRule + ":" + r.ID, Rationale: ruleRationale(r, "allows"), MatchedRules: matched, PolicyHash: e.hash}
	}
	if e.p.DefaultEffect == model.EffectAllow {
		return model.Decision{Effect: model.EffectAllow, Reason: ReasonDefaultAllow, Rationale: fmt.Sprintf("No rule matched tool %q for agent %q and the policy default is allow.", tool, req.Agent), PolicyHash: e.hash}
	}
	return deny(ReasonDefaultDeny, fmt.Sprintf("No rule matched tool %q for agent %q and the policy default is deny.", tool, req.Agent))
}

func ruleRationale(r model.Rule, verb string) string {
	if strings.TrimSpace(r.Rationale) != "" {
		return fmt.Sprintf("Rule %q %s this request: %s.", r.ID, verb, strings.TrimRight(r.Rationale, "."))
	}
	return fmt.Sprintf("Rule %q %s this request.", r.ID, verb)
}

func (e *Engine) rule(id string) model.Rule {
	for _, r := range e.p.Rules {
		if r.ID == id {
			return r
		}
	}
	return model.Rule{ID: id}
}

// matches reports whether every populated selector of m accepts req. Empty
// selectors match everything. Capabilities matches when the tool holds any
// of the listed capabilities. MaxTier matches tools at or below that tier.
func (e *Engine) matches(m model.RuleMatch, req model.PolicyRequest) bool {
	if len(m.Tools) > 0 && !contains(m.Tools, req.Tool.Name) {
		return false
	}
	if len(m.Agents) > 0 && !contains(m.Agents, req.Agent) {
		return false
	}
	if len(m.Labs) > 0 && !contains(m.Labs, req.Lab) {
		return false
	}
	if len(m.Capabilities) > 0 && !intersects(m.Capabilities, req.Tool.Capabilities) {
		return false
	}
	if m.MaxTier != "" && req.Tool.RiskTier.Rank() > m.MaxTier.Rank() {
		return false
	}
	return true
}

// dangerousCombination returns the first forbidden combination fully present
// in caps, sorted, or nil.
func (e *Engine) dangerousCombination(caps []string) []string {
	have := map[string]bool{}
	for _, c := range caps {
		have[c] = true
	}
	for _, combo := range e.p.DangerousCombos {
		all := true
		for _, c := range combo {
			if !have[c] {
				all = false
				break
			}
		}
		if all {
			out := append([]string(nil), combo...)
			sort.Strings(out)
			return out
		}
	}
	return nil
}

func toolAllowed(allowed []string, tool string) bool {
	for _, a := range allowed {
		if a == Wildcard || a == tool {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func intersects(a, b []string) bool {
	for _, x := range a {
		if contains(b, x) {
			return true
		}
	}
	return false
}
