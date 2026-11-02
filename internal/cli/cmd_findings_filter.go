package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func init() {
	Register("findings-filter", "filter findings by severity, checker, class, or field match", runFindingsFilter)
}

type filterResult struct {
	Total    int `json:"total"`
	Matched  int `json:"matched"`
	Excluded int `json:"excluded"`
}

func runFindingsFilter(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-filter", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	output := fs.String("output", "", "write filtered findings to file (default: stdout)")
	severity := fs.String("severity", "", "minimum severity: low, medium, high, critical")
	checker := fs.String("checker", "", "comma-separated checker names to include")
	class := fs.String("class", "", "comma-separated classes to include")
	field := fs.String("field", "", "field=value filter (e.g. state=open)")
	invert := fs.Bool("invert", false, "invert the filter (exclude matches)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-filter: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-filter: %v\n", err)
		return ExitError
	}

	var findings []map[string]any
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-filter: invalid JSON: %v\n", err)
		return ExitError
	}

	sevOrder := map[string]int{
		"critical": 4, "high": 3, "medium": 2, "low": 1,
	}
	minSev := 0
	if *severity != "" {
		s, ok := sevOrder[strings.ToLower(*severity)]
		if !ok {
			fmt.Fprintf(stderr, "assay findings-filter: invalid severity %q\n", *severity)
			return ExitError
		}
		minSev = s
	}

	checkerSet := make(map[string]bool)
	if *checker != "" {
		for _, c := range strings.Split(*checker, ",") {
			checkerSet[strings.TrimSpace(c)] = true
		}
	}

	classSet := make(map[string]bool)
	if *class != "" {
		for _, c := range strings.Split(*class, ",") {
			classSet[strings.TrimSpace(c)] = true
		}
	}

	var fieldKey, fieldVal string
	if *field != "" {
		parts := strings.SplitN(*field, "=", 2)
		if len(parts) != 2 {
			fmt.Fprintf(stderr, "assay findings-filter: --field requires KEY=VALUE format\n")
			return ExitError
		}
		fieldKey, fieldVal = parts[0], parts[1]
	}

	var matched []map[string]any
	for _, f := range findings {
		keep := true

		if minSev > 0 {
			fs, _ := f["severity"].(string)
			if sevOrder[strings.ToLower(fs)] < minSev {
				keep = false
			}
		}

		if keep && len(checkerSet) > 0 {
			cn, _ := f["checker"].(string)
			if !checkerSet[cn] {
				keep = false
			}
		}

		if keep && len(classSet) > 0 {
			cl, _ := f["class"].(string)
			if !classSet[cl] {
				keep = false
			}
		}

		if keep && fieldKey != "" {
			fv, _ := f[fieldKey].(string)
			if fv != fieldVal {
				keep = false
			}
		}

		if *invert {
			keep = !keep
		}

		if keep {
			matched = append(matched, f)
		}
	}

	res := filterResult{
		Total:    len(findings),
		Matched:  len(matched),
		Excluded: len(findings) - len(matched),
	}

	if matched == nil {
		matched = []map[string]any{}
	}
	outData, _ := json.MarshalIndent(matched, "", "  ")

	if *output != "" {
		if err := os.WriteFile(*output, append(outData, '\n'), 0o644); err != nil {
			fmt.Fprintf(stderr, "assay findings-filter: %v\n", err)
			return ExitError
		}
	} else if *format != "json" {
		fmt.Fprintf(stdout, "%s\n", outData)
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	case "text":
		fmt.Fprintf(stdout, "\nFiltered: %d/%d findings matched", res.Matched, res.Total)
		if *severity != "" {
			fmt.Fprintf(stdout, " (severity >= %s)", *severity)
		}
		if *checker != "" {
			fmt.Fprintf(stdout, " (checker in %s)", *checker)
		}
		fmt.Fprintln(stdout)
	default:
		fmt.Fprintf(stderr, "assay findings-filter: unknown format %q\n", *format)
		return ExitError
	}

	return ExitPass
}
