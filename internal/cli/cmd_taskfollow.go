package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/taskfollow"
)

func init() {
	Register("taskfollow", "score instruction following accuracy and task compliance", runTaskfollow)
}

func runTaskfollow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay taskfollow", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay taskfollow: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay taskfollow: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "taskfollow", err)
	}

	report := taskfollow.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = taskfollow.FormatJSON(report)
	default:
		output = taskfollow.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "taskfollow", err)
	}

	return ExitPass
}
