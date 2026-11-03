package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/privacyaudit"
)

func init() {
	Register("privacy-audit", "evaluate model PII handling and data minimization", runPrivacyAudit)
}

func runPrivacyAudit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay privacy-audit", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay privacy-audit: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay privacy-audit: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "privacy-audit", err)
	}

	report := privacyaudit.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = privacyaudit.FormatJSON(report)
	default:
		output = privacyaudit.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "privacy-audit", err)
	}

	return ExitPass
}
