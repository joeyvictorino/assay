package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func init() {
	Register("findings-export", "export findings to CSV, SARIF, or Markdown", runFindingsExport)
}

func runFindingsExport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-export", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	format := fs.String("format", "", "export format: csv, sarif, or markdown (required)")
	output := fs.String("output", "", "write to file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" || *format == "" {
		fmt.Fprintln(stderr, "assay findings-export: --input and --format are required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-export: %v\n", err)
		return ExitError
	}

	var findings []map[string]any
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-export: invalid JSON: %v\n", err)
		return ExitError
	}

	var w io.Writer = stdout
	var f *os.File
	if *output != "" {
		f, err = os.Create(*output)
		if err != nil {
			fmt.Fprintf(stderr, "assay findings-export: %v\n", err)
			return ExitError
		}
		defer f.Close()
		w = f
	}

	switch *format {
	case "csv":
		return exportCSV(w, findings, stderr)
	case "sarif":
		return exportSARIF(w, findings, stderr)
	case "markdown":
		return exportMarkdown(w, findings, stderr)
	default:
		fmt.Fprintf(stderr, "assay findings-export: unknown format %q (use csv, sarif, or markdown)\n", *format)
		return ExitError
	}
}

func exportCSV(w io.Writer, findings []map[string]any, stderr io.Writer) int {
	cw := csv.NewWriter(w)
	cw.Write([]string{"checker", "class", "severity", "message"})
	for _, f := range findings {
		cw.Write([]string{
			strVal(f, "checker"),
			strVal(f, "class"),
			strVal(f, "severity"),
			strVal(f, "message"),
		})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		fmt.Fprintf(stderr, "assay findings-export: csv: %v\n", err)
		return ExitError
	}
	return ExitPass
}

func exportSARIF(w io.Writer, findings []map[string]any, stderr io.Writer) int {
	var results []map[string]any
	for _, f := range findings {
		results = append(results, map[string]any{
			"ruleId": strVal(f, "checker"),
			"level":  sarifLevel(strVal(f, "severity")),
			"message": map[string]any{
				"text": strVal(f, "message"),
			},
		})
	}

	sarif := map[string]any{
		"version": "2.1.0",
		"$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		"runs": []map[string]any{{
			"tool": map[string]any{
				"driver": map[string]any{
					"name":    "assay",
					"version": "dev",
				},
			},
			"results": results,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sarif); err != nil {
		fmt.Fprintf(stderr, "assay findings-export: sarif: %v\n", err)
		return ExitError
	}
	return ExitPass
}

func exportMarkdown(w io.Writer, findings []map[string]any, stderr io.Writer) int {
	fmt.Fprintln(w, "| Checker | Class | Severity | Message |")
	fmt.Fprintln(w, "| ------- | ----- | -------- | ------- |")
	for _, f := range findings {
		fmt.Fprintf(w, "| %s | %s | %s | %s |\n",
			escMd(strVal(f, "checker")),
			escMd(strVal(f, "class")),
			escMd(strVal(f, "severity")),
			escMd(strVal(f, "message")),
		)
	}
	return ExitPass
}

func strVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

func sarifLevel(sev string) string {
	switch strings.ToLower(sev) {
	case "critical", "high":
		return "error"
	case "medium":
		return "warning"
	default:
		return "note"
	}
}

func escMd(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
