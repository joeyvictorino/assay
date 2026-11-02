package policy

import (
	"sort"

	"github.com/joeyvictorino/assay/internal/model"
)

// CoverageReport describes how thoroughly the policy's explicit rules cover
// the agent×tool matrix defined by the agent boundaries and tool manifests.
type CoverageReport struct {
	TotalPairs   int             `json:"total_pairs"`
	CoveredPairs int             `json:"covered_pairs"`
	Percentage   float64         `json:"coverage_pct"`
	Agents       []AgentCoverage `json:"agents"`
	Uncovered    []Pair          `json:"uncovered"`
}

// AgentCoverage summarises one agent's rule coverage.
type AgentCoverage struct {
	Agent        string   `json:"agent"`
	AllowedTools []string `json:"allowed_tools"`
	CoveredTools []string `json:"covered_tools"`
	Rules        []string `json:"rules"`
}

// Pair is an agent+tool combination with no explicit rule match.
type Pair struct {
	Agent  string `json:"agent"`
	Tool   string `json:"tool"`
	Effect string `json:"default_effect"`
}

// Coverage builds a coverage report by evaluating which agent+tool pairs
// from the boundary map have at least one explicit rule match (allow or deny).
// Pairs that fall through to the default_effect are uncovered.
func Coverage(eng *Engine, manifests []model.ToolManifest) CoverageReport {
	p := eng.Policy()

	toolNames := make([]string, 0, len(manifests))
	for _, m := range manifests {
		toolNames = append(toolNames, m.Name)
	}
	sort.Strings(toolNames)

	toolByName := map[string]model.ToolManifest{}
	for _, m := range manifests {
		toolByName[m.Name] = m
	}

	agentNames := make([]string, 0, len(p.Agents))
	for a := range p.Agents {
		agentNames = append(agentNames, a)
	}
	sort.Strings(agentNames)

	var agents []AgentCoverage
	var uncovered []Pair
	total, covered := 0, 0

	for _, agentName := range agentNames {
		boundary := p.Agents[agentName]
		allowed := reachableTools(boundary.AllowedTools, toolNames)
		sort.Strings(allowed)

		var coveredTools []string
		var matchedRules []string
		ruleSet := map[string]bool{}

		for _, toolName := range allowed {
			total++
			m, ok := toolByName[toolName]
			if !ok {
				continue
			}
			req := model.PolicyRequest{
				Agent:           agentName,
				Tool:            m,
				SignatureOK:     true,
				SignatureReason: "COVERAGE_CHECK",
			}

			matched := matchingRules(eng, req)
			if len(matched) > 0 {
				covered++
				coveredTools = append(coveredTools, toolName)
				for _, rid := range matched {
					if !ruleSet[rid] {
						ruleSet[rid] = true
						matchedRules = append(matchedRules, rid)
					}
				}
			} else {
				uncovered = append(uncovered, Pair{
					Agent:  agentName,
					Tool:   toolName,
					Effect: string(p.DefaultEffect),
				})
			}
		}

		sort.Strings(matchedRules)
		agents = append(agents, AgentCoverage{
			Agent:        agentName,
			AllowedTools: allowed,
			CoveredTools: coveredTools,
			Rules:        matchedRules,
		})
	}

	pct := 0.0
	if total > 0 {
		pct = float64(covered) / float64(total) * 100
	}

	return CoverageReport{
		TotalPairs:   total,
		CoveredPairs: covered,
		Percentage:   pct,
		Agents:       agents,
		Uncovered:    uncovered,
	}
}

// reachableTools returns the subset of known tool names that the boundary
// allows. A wildcard admits every known tool.
func reachableTools(allowed, known []string) []string {
	for _, a := range allowed {
		if a == Wildcard {
			out := make([]string, len(known))
			copy(out, known)
			return out
		}
	}
	set := map[string]bool{}
	for _, t := range known {
		set[t] = true
	}
	var out []string
	for _, a := range allowed {
		if set[a] {
			out = append(out, a)
		}
	}
	return out
}

// matchingRules returns the IDs of rules whose match selector accepts req,
// skipping the pre-rule checks (signature, boundary, tier, etc.) that
// Evaluate performs. This isolates rule coverage from structural checks.
func matchingRules(eng *Engine, req model.PolicyRequest) []string {
	var ids []string
	for _, r := range eng.p.Rules {
		if eng.matches(r.Match, req) {
			ids = append(ids, r.ID)
		}
	}
	return ids
}
