package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func init() {
	Register("findings-merge", "merge findings from multiple run files", runFindingsMerge)
}

func runFindingsMerge(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-merge", stderr)
	inputs := fs.String("inputs", "", "comma-separated paths to findings JSON files (required)")
	output := fs.String("output", "", "write merged findings to file (default: stdout)")
	dedup := fs.Bool("dedup", true, "deduplicate merged findings")
	keys := fs.String("keys", "checker,class,severity,message", "comma-separated fields for dedup key")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *inputs == "" {
		fmt.Fprintln(stderr, "assay findings-merge: --inputs is required")
		return ExitError
	}

	paths := strings.Split(*inputs, ",")
	for i := range paths {
		paths[i] = strings.TrimSpace(paths[i])
	}

	var all []map[string]any
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintf(stderr, "assay findings-merge: %s: %v\n", p, err)
			return ExitError
		}
		var findings []map[string]any
		if err := json.Unmarshal(data, &findings); err != nil {
			fmt.Fprintf(stderr, "assay findings-merge: %s: invalid JSON: %v\n", p, err)
			return ExitError
		}
		all = append(all, findings...)
	}

	totalBefore := len(all)
	merged := all

	if *dedup {
		keyFields := strings.Split(*keys, ",")
		for i := range keyFields {
			keyFields[i] = strings.TrimSpace(keyFields[i])
		}
		seen := make(map[string]bool)
		var unique []map[string]any
		for _, f := range all {
			h := dedupHash(f, keyFields)
			if !seen[h] {
				seen[h] = true
				unique = append(unique, f)
			}
		}
		merged = unique
	}

	if *output != "" {
		out, _ := json.MarshalIndent(merged, "", "  ")
		if err := os.WriteFile(*output, out, 0644); err != nil {
			fmt.Fprintf(stderr, "assay findings-merge: write: %v\n", err)
			return ExitError
		}
	}

	result := map[string]any{
		"sources":      len(paths),
		"total_before": totalBefore,
		"total_after":  len(merged),
		"findings":     merged,
	}

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		fmt.Fprintf(stdout, "Findings Merge\n")
		fmt.Fprintf(stdout, "Sources:      %d\n", len(paths))
		fmt.Fprintf(stdout, "Total before: %d\n", totalBefore)
		fmt.Fprintf(stdout, "Total after:  %d\n", len(merged))
		if *dedup && totalBefore != len(merged) {
			fmt.Fprintf(stdout, "Duplicates:   %d\n", totalBefore-len(merged))
		}
	}
	return ExitPass
}
