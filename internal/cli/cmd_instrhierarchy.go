package cli

import (
	"github.com/joeyvictorino/assay/internal/instrhierarchy"
)

func init() {
	Register("instr-hierarchy", "score instruction priority compliance across models", runInstrHierarchy)
}

func runInstrHierarchy(args []string) int {
	fs := newFlagSet("instr-hierarchy")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	findings, err := readFindings(fs.Args())
	if err != nil {
		writeText("Error: %v\n", err)
		return ExitError
	}

	report := instrhierarchy.Analyze(findings)

	switch *format {
	case "json":
		writeText("%s\n", instrhierarchy.FormatJSON(report))
	default:
		writeText("%s", instrhierarchy.FormatText(report))
	}
	return ExitPass
}
