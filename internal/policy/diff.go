package policy

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// DiffReport describes what changed between two policy versions.
type DiffReport struct {
	Identical            bool                  `json:"identical"`
	DefaultEffectChanged *EffectChange         `json:"default_effect_changed,omitempty"`
	VersionChanged       *StringChange         `json:"version_changed,omitempty"`
	RulesAdded           []model.Rule          `json:"rules_added,omitempty"`
	RulesRemoved         []model.Rule          `json:"rules_removed,omitempty"`
	RulesChanged         []RuleChange          `json:"rules_changed,omitempty"`
	AgentsAdded          []AgentEntry          `json:"agents_added,omitempty"`
	AgentsRemoved        []AgentEntry          `json:"agents_removed,omitempty"`
	AgentsChanged        []AgentChange         `json:"agents_changed,omitempty"`
	CombosAdded          [][]string            `json:"combos_added,omitempty"`
	CombosRemoved        [][]string            `json:"combos_removed,omitempty"`
	WeightsAdded         map[string]float64    `json:"weights_added,omitempty"`
	WeightsRemoved       map[string]float64    `json:"weights_removed,omitempty"`
	WeightsChanged       map[string][2]float64 `json:"weights_changed,omitempty"`
}

// StringChange records a before/after string value.
type StringChange struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// EffectChange records a before/after default_effect value.
type EffectChange struct {
	Before model.Effect `json:"before"`
	After  model.Effect `json:"after"`
}

// RuleChange records what changed in a rule that exists in both versions.
type RuleChange struct {
	ID              string           `json:"id"`
	EffectChanged   *EffectChange    `json:"effect_changed,omitempty"`
	MatchChanged    bool             `json:"match_changed,omitempty"`
	MatchBefore     *model.RuleMatch `json:"match_before,omitempty"`
	MatchAfter      *model.RuleMatch `json:"match_after,omitempty"`
	RationaleBefore string           `json:"rationale_before,omitempty"`
	RationaleAfter  string           `json:"rationale_after,omitempty"`
}

// AgentEntry pairs an agent name with its boundary.
type AgentEntry struct {
	Name     string              `json:"name"`
	Boundary model.AgentBoundary `json:"boundary"`
}

// AgentChange records what changed in an agent boundary.
type AgentChange struct {
	Name    string              `json:"name"`
	Before  model.AgentBoundary `json:"before"`
	After   model.AgentBoundary `json:"after"`
	Details []string            `json:"details"`
}

// Diff compares two loaded policy engines and produces a report of changes.
func Diff(before, after *Engine) DiffReport {
	b := before.Policy()
	a := after.Policy()

	r := DiffReport{Identical: true}

	if b.Version != a.Version {
		r.Identical = false
		r.VersionChanged = &StringChange{Before: b.Version, After: a.Version}
	}

	if b.DefaultEffect != a.DefaultEffect {
		r.Identical = false
		r.DefaultEffectChanged = &EffectChange{Before: b.DefaultEffect, After: a.DefaultEffect}
	}

	diffRules(b.Rules, a.Rules, &r)
	diffAgents(b.Agents, a.Agents, &r)
	diffCombos(b.DangerousCombos, a.DangerousCombos, &r)
	diffWeights(b.Weights, a.Weights, &r)

	return r
}

func diffRules(before, after []model.Rule, r *DiffReport) {
	beforeByID := map[string]model.Rule{}
	for _, rule := range before {
		beforeByID[rule.ID] = rule
	}
	afterByID := map[string]model.Rule{}
	for _, rule := range after {
		afterByID[rule.ID] = rule
	}

	for _, rule := range after {
		if _, ok := beforeByID[rule.ID]; !ok {
			r.Identical = false
			r.RulesAdded = append(r.RulesAdded, rule)
		}
	}

	for _, rule := range before {
		if _, ok := afterByID[rule.ID]; !ok {
			r.Identical = false
			r.RulesRemoved = append(r.RulesRemoved, rule)
		}
	}

	for _, bRule := range before {
		aRule, ok := afterByID[bRule.ID]
		if !ok {
			continue
		}
		change := RuleChange{ID: bRule.ID}
		changed := false

		if bRule.Effect != aRule.Effect {
			change.EffectChanged = &EffectChange{Before: bRule.Effect, After: aRule.Effect}
			changed = true
		}
		if !matchEqual(bRule.Match, aRule.Match) {
			change.MatchChanged = true
			bm, am := bRule.Match, aRule.Match
			change.MatchBefore = &bm
			change.MatchAfter = &am
			changed = true
		}
		if bRule.Rationale != aRule.Rationale {
			change.RationaleBefore = bRule.Rationale
			change.RationaleAfter = aRule.Rationale
			changed = true
		}

		if changed {
			r.Identical = false
			r.RulesChanged = append(r.RulesChanged, change)
		}
	}
}

func matchEqual(a, b model.RuleMatch) bool {
	return sliceEqual(a.Tools, b.Tools) &&
		sliceEqual(a.Agents, b.Agents) &&
		sliceEqual(a.Labs, b.Labs) &&
		sliceEqual(a.Capabilities, b.Capabilities) &&
		a.MaxTier == b.MaxTier
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func diffAgents(before, after map[string]model.AgentBoundary, r *DiffReport) {
	afterNames := make([]string, 0, len(after))
	for name := range after {
		afterNames = append(afterNames, name)
	}
	sort.Strings(afterNames)

	beforeNames := make([]string, 0, len(before))
	for name := range before {
		beforeNames = append(beforeNames, name)
	}
	sort.Strings(beforeNames)

	for _, name := range afterNames {
		if _, ok := before[name]; !ok {
			r.Identical = false
			r.AgentsAdded = append(r.AgentsAdded, AgentEntry{Name: name, Boundary: after[name]})
		}
	}

	for _, name := range beforeNames {
		if _, ok := after[name]; !ok {
			r.Identical = false
			r.AgentsRemoved = append(r.AgentsRemoved, AgentEntry{Name: name, Boundary: before[name]})
		}
	}

	for _, name := range beforeNames {
		bBound := before[name]
		aBound, ok := after[name]
		if !ok {
			continue
		}
		var details []string
		if bBound.MaxTier != aBound.MaxTier {
			details = append(details, fmt.Sprintf("max_tier: %s -> %s", bBound.MaxTier, aBound.MaxTier))
		}
		if !sliceEqual(bBound.AllowedTools, aBound.AllowedTools) {
			details = append(details, fmt.Sprintf("allowed_tools changed"))
		}
		if bBound.MaxDelegationDepth != aBound.MaxDelegationDepth {
			details = append(details, fmt.Sprintf("max_delegation_depth: %d -> %d", bBound.MaxDelegationDepth, aBound.MaxDelegationDepth))
		}
		if bBound.Budget.MaxUSD != aBound.Budget.MaxUSD {
			details = append(details, fmt.Sprintf("budget.max_usd: %.4f -> %.4f", bBound.Budget.MaxUSD, aBound.Budget.MaxUSD))
		}
		if bBound.Budget.MaxLatencyMS != aBound.Budget.MaxLatencyMS {
			details = append(details, fmt.Sprintf("budget.max_latency_ms: %d -> %d", bBound.Budget.MaxLatencyMS, aBound.Budget.MaxLatencyMS))
		}
		if bBound.Budget.MaxTokens != aBound.Budget.MaxTokens {
			details = append(details, fmt.Sprintf("budget.max_tokens: %d -> %d", bBound.Budget.MaxTokens, aBound.Budget.MaxTokens))
		}
		if len(details) > 0 {
			r.Identical = false
			r.AgentsChanged = append(r.AgentsChanged, AgentChange{
				Name:    name,
				Before:  bBound,
				After:   aBound,
				Details: details,
			})
		}
	}
}

func diffCombos(before, after [][]string, r *DiffReport) {
	normalize := func(combos [][]string) []string {
		var keys []string
		for _, combo := range combos {
			sorted := append([]string(nil), combo...)
			sort.Strings(sorted)
			keys = append(keys, strings.Join(sorted, "+"))
		}
		return keys
	}

	bKeys := normalize(before)
	aKeys := normalize(after)

	bSet := map[string]bool{}
	for _, k := range bKeys {
		bSet[k] = true
	}
	aSet := map[string]bool{}
	for _, k := range aKeys {
		aSet[k] = true
	}

	for i, k := range aKeys {
		if !bSet[k] {
			r.Identical = false
			r.CombosAdded = append(r.CombosAdded, after[i])
		}
	}
	for i, k := range bKeys {
		if !aSet[k] {
			r.Identical = false
			r.CombosRemoved = append(r.CombosRemoved, before[i])
		}
	}
}

func diffWeights(before, after map[string]float64, r *DiffReport) {
	if len(before) == 0 && len(after) == 0 {
		return
	}
	for k, v := range after {
		bv, ok := before[k]
		if !ok {
			r.Identical = false
			if r.WeightsAdded == nil {
				r.WeightsAdded = map[string]float64{}
			}
			r.WeightsAdded[k] = v
		} else if bv != v {
			r.Identical = false
			if r.WeightsChanged == nil {
				r.WeightsChanged = map[string][2]float64{}
			}
			r.WeightsChanged[k] = [2]float64{bv, v}
		}
	}
	for k, v := range before {
		if _, ok := after[k]; !ok {
			r.Identical = false
			if r.WeightsRemoved == nil {
				r.WeightsRemoved = map[string]float64{}
			}
			r.WeightsRemoved[k] = v
		}
	}
}

// FormatText returns a human-readable text summary of the diff.
func FormatDiffText(d DiffReport) string {
	if d.Identical {
		return "Policies are identical.\n"
	}

	var b strings.Builder
	b.WriteString("Policy diff:\n")

	if d.VersionChanged != nil {
		fmt.Fprintf(&b, "  version: %s -> %s\n", d.VersionChanged.Before, d.VersionChanged.After)
	}
	if d.DefaultEffectChanged != nil {
		fmt.Fprintf(&b, "  default_effect: %s -> %s\n", d.DefaultEffectChanged.Before, d.DefaultEffectChanged.After)
	}

	if len(d.RulesAdded) > 0 {
		b.WriteString("\n  Rules added:\n")
		for _, r := range d.RulesAdded {
			fmt.Fprintf(&b, "    + %s (%s)\n", r.ID, r.Effect)
		}
	}
	if len(d.RulesRemoved) > 0 {
		b.WriteString("\n  Rules removed:\n")
		for _, r := range d.RulesRemoved {
			fmt.Fprintf(&b, "    - %s (%s)\n", r.ID, r.Effect)
		}
	}
	if len(d.RulesChanged) > 0 {
		b.WriteString("\n  Rules changed:\n")
		for _, c := range d.RulesChanged {
			fmt.Fprintf(&b, "    ~ %s\n", c.ID)
			if c.EffectChanged != nil {
				fmt.Fprintf(&b, "        effect: %s -> %s\n", c.EffectChanged.Before, c.EffectChanged.After)
			}
			if c.MatchChanged {
				b.WriteString("        match criteria changed\n")
			}
			if c.RationaleBefore != c.RationaleAfter && (c.RationaleBefore != "" || c.RationaleAfter != "") {
				b.WriteString("        rationale changed\n")
			}
		}
	}

	if len(d.AgentsAdded) > 0 {
		b.WriteString("\n  Agents added:\n")
		for _, a := range d.AgentsAdded {
			fmt.Fprintf(&b, "    + %s (max_tier: %s)\n", a.Name, a.Boundary.MaxTier)
		}
	}
	if len(d.AgentsRemoved) > 0 {
		b.WriteString("\n  Agents removed:\n")
		for _, a := range d.AgentsRemoved {
			fmt.Fprintf(&b, "    - %s (max_tier: %s)\n", a.Name, a.Boundary.MaxTier)
		}
	}
	if len(d.AgentsChanged) > 0 {
		b.WriteString("\n  Agents changed:\n")
		for _, c := range d.AgentsChanged {
			fmt.Fprintf(&b, "    ~ %s\n", c.Name)
			for _, d := range c.Details {
				fmt.Fprintf(&b, "        %s\n", d)
			}
		}
	}

	if len(d.CombosAdded) > 0 {
		b.WriteString("\n  Dangerous combinations added:\n")
		for _, c := range d.CombosAdded {
			fmt.Fprintf(&b, "    + %s\n", strings.Join(c, " + "))
		}
	}
	if len(d.CombosRemoved) > 0 {
		b.WriteString("\n  Dangerous combinations removed:\n")
		for _, c := range d.CombosRemoved {
			fmt.Fprintf(&b, "    - %s\n", strings.Join(c, " + "))
		}
	}

	if len(d.WeightsAdded) > 0 || len(d.WeightsRemoved) > 0 || len(d.WeightsChanged) > 0 {
		b.WriteString("\n  Weights changed:\n")
		for k, v := range d.WeightsAdded {
			fmt.Fprintf(&b, "    + %s: %.4f\n", k, v)
		}
		for k, v := range d.WeightsRemoved {
			fmt.Fprintf(&b, "    - %s: %.4f\n", k, v)
		}
		for k, v := range d.WeightsChanged {
			fmt.Fprintf(&b, "    ~ %s: %.4f -> %.4f\n", k, v[0], v[1])
		}
	}

	return b.String()
}
