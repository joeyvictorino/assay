package cli

import (
	"github.com/joeyvictorino/assay/internal/boundarytest"
)

func init() {
	Register("boundary-test", "test system prompt boundary adherence across models", runBoundaryTest)
}

func runBoundaryTest(args []string) int {
	fs := newFlagSet("boundary-test")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	findings, err := readFindings(fs.Args())
	if err != nil {
		writeText("Error: %v\n", err)
		return ExitError
	}

	report := boundarytest.Analyze(findings)

	switch *format {
	case "json":
		writeText("%s\n", boundarytest.FormatJSON(report))
	default:
		writeText("%s", boundarytest.FormatText(report))
	}
	return ExitPass
}
