package modelver

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type VersionEntry struct {
	RunID   string `json:"run_id"`
	Model   string `json:"model"`
	Version string `json:"version"`
}

type ChangeEvent struct {
	Model       string  `json:"model"`
	FromVersion string  `json:"from_version"`
	ToVersion   string  `json:"to_version"`
	FromRunID   string  `json:"from_run_id"`
	ToRunID     string  `json:"to_run_id"`
	Direction   string  `json:"direction"`
	DeltaScore  float64 `json:"delta_score"`
}

type ModelSummary struct {
	Model          string   `json:"model"`
	VersionsSeen   []string `json:"versions_seen"`
	CurrentVersion string   `json:"current_version"`
	ChangeCount    int      `json:"change_count"`
	Unrecorded     int      `json:"unrecorded_runs"`
}

type PerformanceDelta struct {
	Model       string  `json:"model"`
	FromVersion string  `json:"from_version"`
	ToVersion   string  `json:"to_version"`
	FromScore   float64 `json:"from_score"`
	ToScore     float64 `json:"to_score"`
	Delta       float64 `json:"delta"`
	Improved    bool    `json:"improved"`
}

type Report struct {
	Timeline     []VersionEntry     `json:"timeline"`
	Changes      []ChangeEvent      `json:"changes"`
	Models       []ModelSummary     `json:"models"`
	Performance  []PerformanceDelta `json:"performance"`
	TotalRuns    int                `json:"total_runs"`
	TotalChanges int                `json:"total_changes"`
	Unrecorded   int                `json:"unrecorded_runs"`
}

func extractVersion(modelStr string) string {
	parts := strings.Split(modelStr, "-")
	for i := len(parts) - 1; i >= 0; i-- {
		p := parts[i]
		if len(p) > 0 && p[0] >= '0' && p[0] <= '9' {
			return strings.Join(parts[i:], "-")
		}
	}
	return ""
}

func modelBase(modelStr string) string {
	ver := extractVersion(modelStr)
	if ver == "" {
		return modelStr
	}
	return strings.TrimSuffix(strings.TrimSuffix(modelStr, ver), "-")
}

type runFindings struct {
	runID    string
	findings []model.Finding
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	runMap := make(map[string][]model.Finding)
	runOrder := make([]string, 0)
	seen := make(map[string]bool)
	for _, f := range findings {
		if !seen[f.RunID] {
			runOrder = append(runOrder, f.RunID)
			seen[f.RunID] = true
		}
		runMap[f.RunID] = append(runMap[f.RunID], f)
	}

	var timeline []VersionEntry
	modelVersions := make(map[string][]VersionEntry)
	unrecorded := 0

	for _, runID := range runOrder {
		ff := runMap[runID]
		modelsInRun := make(map[string]bool)
		for _, f := range ff {
			mdl := f.Model.Model
			if mdl == "" {
				continue
			}
			if modelsInRun[mdl] {
				continue
			}
			modelsInRun[mdl] = true

			ver := extractVersion(mdl)
			entry := VersionEntry{
				RunID:   runID,
				Model:   mdl,
				Version: ver,
			}
			timeline = append(timeline, entry)

			base := modelBase(mdl)
			modelVersions[base] = append(modelVersions[base], entry)

			if ver == "" {
				unrecorded++
			}
		}
	}

	var changes []ChangeEvent
	for base, entries := range modelVersions {
		for i := 1; i < len(entries); i++ {
			prev := entries[i-1]
			curr := entries[i]
			if prev.Version != curr.Version {
				dir := "unknown"
				if prev.Version != "" && curr.Version != "" {
					if curr.Version > prev.Version {
						dir = "upgrade"
					} else {
						dir = "downgrade"
					}
				}
				changes = append(changes, ChangeEvent{
					Model:       base,
					FromVersion: prev.Version,
					ToVersion:   curr.Version,
					FromRunID:   prev.RunID,
					ToRunID:     curr.RunID,
					Direction:   dir,
				})
			}
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].ToRunID < changes[j].ToRunID
	})

	var models []ModelSummary
	for base, entries := range modelVersions {
		vset := make(map[string]bool)
		unrec := 0
		for _, e := range entries {
			if e.Version != "" {
				vset[e.Version] = true
			} else {
				unrec++
			}
		}
		var versions []string
		for v := range vset {
			versions = append(versions, v)
		}
		sort.Strings(versions)

		changeCount := 0
		for i := 1; i < len(entries); i++ {
			if entries[i-1].Version != entries[i].Version {
				changeCount++
			}
		}

		current := ""
		if len(entries) > 0 {
			current = entries[len(entries)-1].Version
		}

		models = append(models, ModelSummary{
			Model:          base,
			VersionsSeen:   versions,
			CurrentVersion: current,
			ChangeCount:    changeCount,
			Unrecorded:     unrec,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Model < models[j].Model
	})

	var perf []PerformanceDelta
	for base, entries := range modelVersions {
		type verScore struct {
			version string
			total   float64
			count   int
		}
		verScores := make(map[string]*verScore)
		for _, e := range entries {
			if e.Version == "" {
				continue
			}
			vs, ok := verScores[e.Version]
			if !ok {
				vs = &verScore{version: e.Version}
				verScores[e.Version] = vs
			}
			for _, f := range runMap[e.RunID] {
				if modelBase(f.Model.Model) == base {
					vs.total += float64(f.Severity.Rank())
					vs.count++
				}
			}
		}

		var sortedVers []string
		for v := range verScores {
			sortedVers = append(sortedVers, v)
		}
		sort.Strings(sortedVers)

		for i := 1; i < len(sortedVers); i++ {
			from := verScores[sortedVers[i-1]]
			to := verScores[sortedVers[i]]
			fromAvg := 0.0
			if from.count > 0 {
				fromAvg = from.total / float64(from.count)
			}
			toAvg := 0.0
			if to.count > 0 {
				toAvg = to.total / float64(to.count)
			}
			delta := toAvg - fromAvg
			perf = append(perf, PerformanceDelta{
				Model:       base,
				FromVersion: from.version,
				ToVersion:   to.version,
				FromScore:   fromAvg,
				ToScore:     toAvg,
				Delta:       delta,
				Improved:    delta > 0,
			})
		}
	}

	return Report{
		Timeline:     timeline,
		Changes:      changes,
		Models:       models,
		Performance:  perf,
		TotalRuns:    len(runOrder),
		TotalChanges: len(changes),
		Unrecorded:   unrecorded,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Model Version Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total runs:       %d\n", r.TotalRuns)
	fmt.Fprintf(&b, "  Version changes:  %d\n", r.TotalChanges)
	fmt.Fprintf(&b, "  Unrecorded:       %d\n\n", r.Unrecorded)

	if len(r.Models) > 0 {
		b.WriteString("  Models:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, ms := range r.Models {
			fmt.Fprintf(&b, "    %-25s versions: %s (current: %s)\n",
				ms.Model, strings.Join(ms.VersionsSeen, ", "), ms.CurrentVersion)
			if ms.ChangeCount > 0 {
				fmt.Fprintf(&b, "      %d version change(s)\n", ms.ChangeCount)
			}
			if ms.Unrecorded > 0 {
				fmt.Fprintf(&b, "      %d run(s) with no version recorded\n", ms.Unrecorded)
			}
		}
		b.WriteString("\n")
	}

	if len(r.Changes) > 0 {
		b.WriteString("  Version Changes:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, ch := range r.Changes {
			fmt.Fprintf(&b, "    %s: %s -> %s (%s) [run %s -> %s]\n",
				ch.Model, ch.FromVersion, ch.ToVersion, ch.Direction, ch.FromRunID, ch.ToRunID)
		}
		b.WriteString("\n")
	}

	if len(r.Performance) > 0 {
		b.WriteString("  Performance Deltas:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, pd := range r.Performance {
			arrow := "declined"
			if pd.Improved {
				arrow = "improved"
			}
			fmt.Fprintf(&b, "    %s %s->%s: %.2f -> %.2f (%+.2f, %s)\n",
				pd.Model, pd.FromVersion, pd.ToVersion, pd.FromScore, pd.ToScore, pd.Delta, arrow)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
