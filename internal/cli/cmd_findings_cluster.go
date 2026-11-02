package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
)

func init() {
	Register("findings-cluster", "group similar findings by feature vectors", runFindingsCluster)
}

func findingFeatureKey(f map[string]any, fields []string) string {
	parts := make([]string, len(fields))
	for i, field := range fields {
		parts[i] = fmt.Sprint(f[field])
	}
	return strings.Join(parts, "|")
}

type findingCluster struct {
	ID       int              `json:"cluster_id"`
	Key      string           `json:"key"`
	Count    int              `json:"count"`
	Severity string           `json:"dominant_severity"`
	Sample   map[string]any   `json:"sample"`
	Members  []map[string]any `json:"members,omitempty"`
}

func dominantSeverity(findings []map[string]any) string {
	order := []string{"critical", "high", "medium", "low", "info"}
	counts := make(map[string]int)
	for _, f := range findings {
		sev := strings.ToLower(fmt.Sprint(f["severity"]))
		counts[sev]++
	}
	best := ""
	bestIdx := math.MaxInt32
	for sev := range counts {
		for i, o := range order {
			if o == sev && i < bestIdx {
				bestIdx = i
				best = sev
			}
		}
	}
	if best == "" {
		return "unknown"
	}
	return best
}

func runFindingsCluster(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-cluster", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	fields := fs.String("fields", "checker,class", "comma-separated fields for clustering key")
	showMembers := fs.Bool("show-members", false, "include member findings in output")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-cluster: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-cluster: %v\n", err)
		return ExitError
	}

	var findings []map[string]any
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-cluster: invalid JSON: %v\n", err)
		return ExitError
	}

	clusterFields := strings.Split(*fields, ",")
	for i := range clusterFields {
		clusterFields[i] = strings.TrimSpace(clusterFields[i])
	}

	groups := make(map[string][]map[string]any)
	var keyOrder []string
	for _, f := range findings {
		key := findingFeatureKey(f, clusterFields)
		if _, exists := groups[key]; !exists {
			keyOrder = append(keyOrder, key)
		}
		groups[key] = append(groups[key], f)
	}

	var clusters []findingCluster
	for i, key := range keyOrder {
		members := groups[key]
		c := findingCluster{
			ID:       i + 1,
			Key:      key,
			Count:    len(members),
			Severity: dominantSeverity(members),
			Sample:   members[0],
		}
		if *showMembers {
			c.Members = members
		}
		clusters = append(clusters, c)
	}

	sort.SliceStable(clusters, func(i, j int) bool {
		return clusters[i].Count > clusters[j].Count
	})
	for i := range clusters {
		clusters[i].ID = i + 1
	}

	result := map[string]any{
		"total_findings": len(findings),
		"cluster_count":  len(clusters),
		"cluster_fields": clusterFields,
		"clusters":       clusters,
	}

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		fmt.Fprintf(stdout, "Finding Clusters (%d findings -> %d clusters)\n", len(findings), len(clusters))
		fmt.Fprintf(stdout, "Cluster fields: %s\n\n", strings.Join(clusterFields, ", "))
		for _, c := range clusters {
			fmt.Fprintf(stdout, "  #%d [%s] %s (%d findings)\n", c.ID, c.Severity, c.Key, c.Count)
		}
	}
	return ExitPass
}
