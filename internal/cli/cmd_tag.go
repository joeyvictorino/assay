package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/tagging"
)

func init() {
	Register("tag", "tag and categorize findings using pattern-based rules", runTag)
}

func runTag(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay tag", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	rulesFile := fs.String("rules", "", "tag rules YAML file (required)")
	filterTag := fs.String("filter", "", "show only findings with this tag")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay tag: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay tag: --findings is required")
		return ExitError
	}
	if *rulesFile == "" {
		fmt.Fprintln(stderr, "assay tag: --rules is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "tag", err)
	}

	rules, err := tagging.LoadRules(*rulesFile)
	if err != nil {
		return fail(stderr, "tag", err)
	}

	report := tagging.Analyze(data, rules)

	if *filterTag != "" {
		report.Findings = tagging.FilterByTag(report.Findings, *filterTag)
	}

	var output string
	switch *format {
	case "json":
		output = tagging.FormatJSON(report)
	default:
		output = tagging.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "tag", err)
	}

	return ExitPass
}
