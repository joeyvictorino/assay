package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/spatial"
)

func init() {
	Register("spatial", "score spatial reasoning quality, orientation, and geometric understanding", runSpatial)
}

func runSpatial(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay spatial", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay spatial: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay spatial: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "spatial", err)
	}

	report := spatial.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = spatial.FormatJSON(report)
	default:
		output = spatial.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "spatial", err)
	}

	return ExitPass
}
