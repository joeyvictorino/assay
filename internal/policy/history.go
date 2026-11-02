package policy

import (
	"fmt"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// HistoryEntry describes one version in a policy changelog.
type HistoryEntry struct {
	Version int      `json:"version"`
	Hash    string   `json:"hash"`
	Changes []string `json:"changes"`
}

// BuildHistory takes a sequence of policy engines (earliest first) and
// produces a changelog by diffing consecutive pairs.
func BuildHistory(versions []*Engine) []HistoryEntry {
	if len(versions) == 0 {
		return nil
	}

	entries := make([]HistoryEntry, 0, len(versions))

	entries = append(entries, HistoryEntry{
		Version: 1,
		Hash:    versions[0].Hash(),
		Changes: []string{"initial version"},
	})

	for i := 1; i < len(versions); i++ {
		d := Diff(versions[i-1], versions[i])
		entry := HistoryEntry{
			Version: i + 1,
			Hash:    versions[i].Hash(),
			Changes: summarizeDiff(d),
		}
		entries = append(entries, entry)
	}

	return entries
}

func summarizeDiff(d DiffReport) []string {
	if d.Identical {
		return []string{"no changes"}
	}

	var changes []string

	if d.VersionChanged != nil {
		changes = append(changes, fmt.Sprintf("version changed: %s -> %s", d.VersionChanged.Before, d.VersionChanged.After))
	}
	if d.DefaultEffectChanged != nil {
		changes = append(changes, fmt.Sprintf("default_effect changed: %s -> %s", d.DefaultEffectChanged.Before, d.DefaultEffectChanged.After))
	}
	if len(d.RulesAdded) > 0 {
		ids := make([]string, len(d.RulesAdded))
		for i, r := range d.RulesAdded {
			ids[i] = r.ID
		}
		changes = append(changes, fmt.Sprintf("rules added: %s", strings.Join(ids, ", ")))
	}
	if len(d.RulesRemoved) > 0 {
		ids := make([]string, len(d.RulesRemoved))
		for i, r := range d.RulesRemoved {
			ids[i] = r.ID
		}
		changes = append(changes, fmt.Sprintf("rules removed: %s", strings.Join(ids, ", ")))
	}
	if len(d.RulesChanged) > 0 {
		ids := make([]string, len(d.RulesChanged))
		for i, c := range d.RulesChanged {
			ids[i] = c.ID
		}
		changes = append(changes, fmt.Sprintf("rules modified: %s", strings.Join(ids, ", ")))
	}
	if len(d.AgentsAdded) > 0 {
		names := make([]string, len(d.AgentsAdded))
		for i, a := range d.AgentsAdded {
			names[i] = a.Name
		}
		changes = append(changes, fmt.Sprintf("agents added: %s", strings.Join(names, ", ")))
	}
	if len(d.AgentsRemoved) > 0 {
		names := make([]string, len(d.AgentsRemoved))
		for i, a := range d.AgentsRemoved {
			names[i] = a.Name
		}
		changes = append(changes, fmt.Sprintf("agents removed: %s", strings.Join(names, ", ")))
	}
	if len(d.AgentsChanged) > 0 {
		names := make([]string, len(d.AgentsChanged))
		for i, c := range d.AgentsChanged {
			names[i] = c.Name
		}
		changes = append(changes, fmt.Sprintf("agents modified: %s", strings.Join(names, ", ")))
	}
	if len(d.CombosAdded) > 0 {
		changes = append(changes, fmt.Sprintf("%d dangerous combination(s) added", len(d.CombosAdded)))
	}
	if len(d.CombosRemoved) > 0 {
		changes = append(changes, fmt.Sprintf("%d dangerous combination(s) removed", len(d.CombosRemoved)))
	}
	if len(d.WeightsAdded) > 0 {
		changes = append(changes, fmt.Sprintf("%d weight(s) added", len(d.WeightsAdded)))
	}
	if len(d.WeightsRemoved) > 0 {
		changes = append(changes, fmt.Sprintf("%d weight(s) removed", len(d.WeightsRemoved)))
	}
	if len(d.WeightsChanged) > 0 {
		changes = append(changes, fmt.Sprintf("%d weight(s) modified", len(d.WeightsChanged)))
	}

	return changes
}

// FormatHistoryText renders history entries as human-readable text.
func FormatHistoryText(entries []HistoryEntry) string {
	var b strings.Builder
	b.WriteString("Policy history:\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "\n  v%d [%s]\n", e.Version, e.Hash[:12])
		for _, c := range e.Changes {
			fmt.Fprintf(&b, "    - %s\n", c)
		}
	}
	return b.String()
}

// DiffSummaryFromPolicies is a convenience that loads raw model.Policy values
// (rather than Engines) and produces a changelog. Each policy is wrapped in
// a temporary engine with an empty hash — use BuildHistory when you have real
// engines loaded from files.
func DiffSummaryFromPolicies(versions []model.Policy) []HistoryEntry {
	engines := make([]*Engine, len(versions))
	for i, p := range versions {
		engines[i] = &Engine{p: p}
	}
	return BuildHistory(engines)
}
