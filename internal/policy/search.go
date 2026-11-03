package policy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// SearchQuery specifies filters for searching policy rules. All non-empty
// fields must match (AND). Within a field, matching is substring/contains.
type SearchQuery struct {
	Keyword    string `json:"keyword,omitempty"`
	Effect     string `json:"effect,omitempty"`
	Agent      string `json:"agent,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Capability string `json:"capability,omitempty"`
}

// SearchResult is one rule that matched a SearchQuery.
type SearchResult struct {
	RuleIndex  int        `json:"rule_index"`
	Rule       model.Rule `json:"rule"`
	MatchField string     `json:"match_field"`
	MatchValue string     `json:"match_value"`
}

// Search returns all rules in p that match every non-empty field of q.
func Search(p model.Policy, q SearchQuery) []SearchResult {
	var results []SearchResult
	for i, r := range p.Rules {
		field, value, ok := ruleMatches(r, q)
		if ok {
			results = append(results, SearchResult{
				RuleIndex:  i,
				Rule:       r,
				MatchField: field,
				MatchValue: value,
			})
		}
	}
	return results
}

func ruleMatches(r model.Rule, q SearchQuery) (field, value string, ok bool) {
	if q.Effect != "" && string(r.Effect) != strings.ToLower(q.Effect) {
		return "", "", false
	}
	if q.Agent != "" && !containsStr(r.Match.Agents, q.Agent) {
		return "", "", false
	}
	if q.Tool != "" && !containsStr(r.Match.Tools, q.Tool) {
		return "", "", false
	}
	if q.Capability != "" && !containsStr(r.Match.Capabilities, q.Capability) {
		return "", "", false
	}
	if q.Keyword != "" {
		kw := strings.ToLower(q.Keyword)
		if strings.Contains(strings.ToLower(r.ID), kw) {
			return "id", r.ID, true
		}
		if strings.Contains(strings.ToLower(r.Rationale), kw) {
			return "rationale", r.Rationale, true
		}
		if strings.Contains(strings.ToLower(string(r.Effect)), kw) {
			return "effect", string(r.Effect), true
		}
		for _, a := range r.Match.Agents {
			if strings.Contains(strings.ToLower(a), kw) {
				return "agent", a, true
			}
		}
		for _, t := range r.Match.Tools {
			if strings.Contains(strings.ToLower(t), kw) {
				return "tool", t, true
			}
		}
		for _, c := range r.Match.Capabilities {
			if strings.Contains(strings.ToLower(c), kw) {
				return "capability", c, true
			}
		}
		return "", "", false
	}

	// No keyword filter — determine the primary match field
	if q.Effect != "" {
		return "effect", string(r.Effect), true
	}
	if q.Agent != "" {
		return "agent", q.Agent, true
	}
	if q.Tool != "" {
		return "tool", q.Tool, true
	}
	if q.Capability != "" {
		return "capability", q.Capability, true
	}
	return "all", "", true
}

func containsStr(ss []string, target string) bool {
	t := strings.ToLower(target)
	for _, s := range ss {
		if strings.ToLower(s) == t {
			return true
		}
	}
	return false
}

// FormatSearchText renders search results as human-readable text.
func FormatSearchText(results []SearchResult) string {
	if len(results) == 0 {
		return "No matching rules found.\n"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d rule(s) matched:\n\n", len(results)))
	for _, r := range results {
		b.WriteString(fmt.Sprintf("  [%d] %s (%s)\n", r.RuleIndex, r.Rule.ID, r.Rule.Effect))
		if r.MatchField != "" && r.MatchField != "all" {
			b.WriteString(fmt.Sprintf("       matched %s: %s\n", r.MatchField, r.MatchValue))
		}
		if r.Rule.Rationale != "" {
			b.WriteString(fmt.Sprintf("       %s\n", r.Rule.Rationale))
		}
		if len(r.Rule.Match.Agents) > 0 {
			sort.Strings(r.Rule.Match.Agents)
			b.WriteString(fmt.Sprintf("       agents: %s\n", strings.Join(r.Rule.Match.Agents, ", ")))
		}
		if len(r.Rule.Match.Capabilities) > 0 {
			sort.Strings(r.Rule.Match.Capabilities)
			b.WriteString(fmt.Sprintf("       capabilities: %s\n", strings.Join(r.Rule.Match.Capabilities, ", ")))
		}
		b.WriteString("\n")
	}
	return b.String()
}
