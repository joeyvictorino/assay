package policy

import (
	"sort"

	"github.com/joeyvictorino/assay/internal/model"
)

type BudgetRange struct {
	MinUSD       float64 `json:"min_usd"`
	MaxUSD       float64 `json:"max_usd"`
	MinLatencyMS int64   `json:"min_latency_ms"`
	MaxLatencyMS int64   `json:"max_latency_ms"`
}

type PolicyStats struct {
	RuleCount           int            `json:"rule_count"`
	RulesByEffect       map[string]int `json:"rules_by_effect"`
	AgentCount          int            `json:"agent_count"`
	AgentNames          []string       `json:"agent_names"`
	BudgetRange         BudgetRange    `json:"budget_range"`
	DangerousComboCount int            `json:"dangerous_combo_count"`
	WeightCount         int            `json:"weight_count"`
	Capabilities        []string       `json:"capabilities"`
	Tools               []string       `json:"tools"`
}

func Stats(p model.Policy) PolicyStats {
	s := PolicyStats{
		RuleCount:     len(p.Rules),
		RulesByEffect: map[string]int{},
	}

	for _, r := range p.Rules {
		s.RulesByEffect[string(r.Effect)]++
	}

	agents := make([]string, 0, len(p.Agents))
	for name := range p.Agents {
		agents = append(agents, name)
	}
	sort.Strings(agents)
	s.AgentCount = len(agents)
	s.AgentNames = agents

	first := true
	for _, name := range agents {
		a := p.Agents[name]
		if first {
			s.BudgetRange.MinUSD = a.Budget.MaxUSD
			s.BudgetRange.MaxUSD = a.Budget.MaxUSD
			s.BudgetRange.MinLatencyMS = a.Budget.MaxLatencyMS
			s.BudgetRange.MaxLatencyMS = a.Budget.MaxLatencyMS
			first = false
		} else {
			if a.Budget.MaxUSD < s.BudgetRange.MinUSD {
				s.BudgetRange.MinUSD = a.Budget.MaxUSD
			}
			if a.Budget.MaxUSD > s.BudgetRange.MaxUSD {
				s.BudgetRange.MaxUSD = a.Budget.MaxUSD
			}
			if a.Budget.MaxLatencyMS < s.BudgetRange.MinLatencyMS {
				s.BudgetRange.MinLatencyMS = a.Budget.MaxLatencyMS
			}
			if a.Budget.MaxLatencyMS > s.BudgetRange.MaxLatencyMS {
				s.BudgetRange.MaxLatencyMS = a.Budget.MaxLatencyMS
			}
		}
	}

	s.DangerousComboCount = len(p.DangerousCombos)
	s.WeightCount = len(p.Weights)

	capSet := map[string]bool{}
	for _, r := range p.Rules {
		for _, c := range r.Match.Capabilities {
			capSet[c] = true
		}
	}
	caps := make([]string, 0, len(capSet))
	for c := range capSet {
		caps = append(caps, c)
	}
	sort.Strings(caps)
	s.Capabilities = caps

	toolSet := map[string]bool{}
	for _, name := range agents {
		a := p.Agents[name]
		for _, t := range a.AllowedTools {
			toolSet[t] = true
		}
	}
	tools := make([]string, 0, len(toolSet))
	for t := range toolSet {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	s.Tools = tools

	return s
}
