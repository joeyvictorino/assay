package policy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// LintIssue describes a single policy lint finding.
type LintIssue struct {
	RuleID   string `json:"rule_id,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Kind     string `json:"kind"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "warning" or "error"
}

// Lint checks a loaded policy against tool manifests and reports issues:
//   - rules referencing tool names absent from all manifests
//   - rules referencing agent names not in the policy's agents map
//   - allow rules shadowed by a deny rule with the same or broader match
func Lint(eng *Engine, manifests []model.ToolManifest) []LintIssue {
	p := eng.Policy()

	toolNames := map[string]bool{}
	for _, m := range manifests {
		toolNames[m.Name] = true
	}

	agentNames := map[string]bool{}
	for a := range p.Agents {
		agentNames[a] = true
	}

	var issues []LintIssue

	// Check agent boundary allowed_tools against manifests.
	for name, boundary := range p.Agents {
		for _, tool := range boundary.AllowedTools {
			if tool == Wildcard {
				continue
			}
			if !toolNames[tool] {
				issues = append(issues, LintIssue{
					Agent:    name,
					Kind:     "unknown_tool_in_agent",
					Message:  fmt.Sprintf("agent %q allows tool %q but no manifest defines it", name, tool),
					Severity: "warning",
				})
			}
		}
	}

	// Check rules.
	for _, r := range p.Rules {
		for _, tool := range r.Match.Tools {
			if !toolNames[tool] {
				issues = append(issues, LintIssue{
					RuleID:   r.ID,
					Kind:     "unknown_tool_in_rule",
					Message:  fmt.Sprintf("rule %q matches tool %q but no manifest defines it", r.ID, tool),
					Severity: "warning",
				})
			}
		}
		for _, agent := range r.Match.Agents {
			if !agentNames[agent] {
				issues = append(issues, LintIssue{
					RuleID:   r.ID,
					Kind:     "unknown_agent_in_rule",
					Message:  fmt.Sprintf("rule %q matches agent %q but the policy has no boundary for it", r.ID, agent),
					Severity: "error",
				})
			}
		}
	}

	// Detect shadowed allow rules: an allow that is preceded by a deny
	// with the same or broader match (superset selectors).
	for i, allow := range p.Rules {
		if allow.Effect != model.EffectAllow {
			continue
		}
		for j := 0; j < i; j++ {
			deny := p.Rules[j]
			if deny.Effect != model.EffectDeny {
				continue
			}
			if subsumes(deny.Match, allow.Match) {
				issues = append(issues, LintIssue{
					RuleID:   allow.ID,
					Kind:     "shadowed_allow",
					Message:  fmt.Sprintf("allow rule %q is shadowed by deny rule %q (same or broader match)", allow.ID, deny.ID),
					Severity: "warning",
				})
			}
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Severity != issues[j].Severity {
			return issues[i].Severity == "error"
		}
		return issues[i].Kind < issues[j].Kind
	})

	return issues
}

// subsumes reports whether deny's match criteria are equal to or broader
// than allow's. An empty selector matches everything (broadest). A deny
// subsumes an allow when every non-empty deny selector is a superset of
// or equal to the corresponding allow selector.
func subsumes(deny, allow model.RuleMatch) bool {
	if !listSubsumes(deny.Tools, allow.Tools) {
		return false
	}
	if !listSubsumes(deny.Agents, allow.Agents) {
		return false
	}
	if !listSubsumes(deny.Labs, allow.Labs) {
		return false
	}
	if !listSubsumes(deny.Capabilities, allow.Capabilities) {
		return false
	}
	if deny.MaxTier != "" && allow.MaxTier != "" && deny.MaxTier.Rank() < allow.MaxTier.Rank() {
		return false
	}
	if deny.MaxTier != "" && allow.MaxTier == "" {
		return false
	}
	return true
}

// listSubsumes checks if deny list is a superset of allow list. An empty
// deny matches everything, so it always subsumes. An empty allow matches
// everything too, so a non-empty deny can never subsume it.
func listSubsumes(deny, allow []string) bool {
	if len(deny) == 0 {
		return true
	}
	if len(allow) == 0 {
		return false
	}
	set := map[string]bool{}
	for _, s := range deny {
		set[strings.TrimSpace(s)] = true
	}
	for _, s := range allow {
		if !set[strings.TrimSpace(s)] {
			return false
		}
	}
	return true
}
