package policy

import (
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// Merge combines multiple policies into one. Later policies take precedence:
// rules with the same ID are replaced, agents with the same name are replaced,
// dangerous_combinations are unioned, and weights are merged with later values
// winning. Version and default_effect come from the last policy.
func Merge(policies []model.Policy) model.Policy {
	if len(policies) == 0 {
		return model.Policy{}
	}
	if len(policies) == 1 {
		return policies[0]
	}

	out := model.Policy{
		Agents:  map[string]model.AgentBoundary{},
		Weights: map[string]float64{},
	}

	rulesMap := map[string]model.Rule{}
	var rulesOrder []string
	comboSet := map[string][]string{}

	for _, p := range policies {
		out.Version = p.Version
		out.DefaultEffect = p.DefaultEffect

		for _, r := range p.Rules {
			if _, exists := rulesMap[r.ID]; !exists {
				rulesOrder = append(rulesOrder, r.ID)
			}
			rulesMap[r.ID] = r
		}

		for name, boundary := range p.Agents {
			out.Agents[name] = boundary
		}

		for _, combo := range p.DangerousCombos {
			key := comboKey(combo)
			if _, exists := comboSet[key]; !exists {
				comboSet[key] = combo
			}
		}

		for k, v := range p.Weights {
			out.Weights[k] = v
		}
	}

	for _, id := range rulesOrder {
		out.Rules = append(out.Rules, rulesMap[id])
	}

	var comboKeys []string
	for k := range comboSet {
		comboKeys = append(comboKeys, k)
	}
	sort.Strings(comboKeys)
	for _, k := range comboKeys {
		out.DangerousCombos = append(out.DangerousCombos, comboSet[k])
	}

	if len(out.Agents) == 0 {
		out.Agents = nil
	}
	if len(out.Weights) == 0 {
		out.Weights = nil
	}

	return out
}

func comboKey(combo []string) string {
	sorted := append([]string(nil), combo...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x00")
}
