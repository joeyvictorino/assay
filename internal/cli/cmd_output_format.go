package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/outputformat"
)

func init() {
	Register("output-format", "check finding output format compliance", runOutputFormat)
}

func runOutputFormat(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay output-format", stderr)
	findingsPath := fs.String("findings", "", "findings JSON file (required)")
	fields := fs.String("fields", "", "comma-separated required fields (default: id,class,severity,state,summary,location)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay output-format: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	findings, err := readFindings(*findingsPath, nil)
	if err != nil {
		return fail(stderr, "output-format", err)
	}

	var requiredFields []string
	if *fields != "" {
		requiredFields = splitCSV(*fields)
	}

	report := outputformat.Analyze(findings, requiredFields)

	var output string
	switch *format {
	case "json":
		output = outputformat.FormatJSON(report)
	default:
		output = outputformat.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "output-format", err)
	}

	return ExitPass
}
