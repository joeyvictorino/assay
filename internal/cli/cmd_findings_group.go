package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

func init() {
	Register("findings-group", "group findings by class, state, or summary field", runFindingsGroup)
}

type groupResult struct {
	Field  string         `json:"field"`
	Groups map[string]int `json:"groups"`
	Total  int            `json:"total"`
}

func runFindingsGroup(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-group", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	by := fs.String("by", "class", "field to group by: class, state, or summary")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-group: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-group: %v\n", err)
		return ExitError
	}

	var findings []map[string]any
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-group: invalid JSON: %v\n", err)
		return ExitError
	}

	groups := make(map[string]int)
	for _, f := range findings {
		key := "(empty)"
		if v, ok := f[*by]; ok {
			if s, ok := v.(string); ok && s != "" {
				key = s
			}
		}
		groups[key]++
	}

	res := groupResult{
		Field:  *by,
		Groups: groups,
		Total:  len(findings),
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(stderr, "assay findings-group: %v\n", err)
			return ExitError
		}
	case "text":
		fmt.Fprintf(stdout, "Grouped %d findings by %q:\n\n", res.Total, res.Field)
		keys := make([]string, 0, len(groups))
		for k := range groups {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(stdout, "  %-30s %d\n", k, groups[k])
		}
	default:
		fmt.Fprintf(stderr, "assay findings-group: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}
