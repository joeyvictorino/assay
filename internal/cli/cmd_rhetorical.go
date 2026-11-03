package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/rhetorical"
)

func init() {
	Register("rhetorical", "score rhetorical quality: logos, ethos, pathos, structure, and clarity", runRhetorical)
}

func runRhetorical(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay rhetorical", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay rhetorical: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay rhetorical: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "rhetorical", err)
	}

	report := rhetorical.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = rhetorical.FormatJSON(report)
	default:
		output = rhetorical.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "rhetorical", err)
	}

	return ExitPass
}
