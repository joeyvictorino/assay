package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/injdefense"
)

func init() {
	Register("injdefense", "measure prompt injection defense effectiveness", runInjdefense)
}

func runInjdefense(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay injdefense", stderr)
	withDef := fs.String("with-defense", "", "findings JSON with defenses active (required)")
	withoutDef := fs.String("without-defense", "", "findings JSON without defenses / baseline (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay injdefense: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *withDef == "" {
		fmt.Fprintln(stderr, "assay injdefense: --with-defense is required")
		return ExitError
	}
	if *withoutDef == "" {
		fmt.Fprintln(stderr, "assay injdefense: --without-defense is required")
		return ExitError
	}

	defended, err := readFindings(*withDef, nil)
	if err != nil {
		return fail(stderr, "injdefense", err)
	}

	baseline, err := readFindings(*withoutDef, nil)
	if err != nil {
		return fail(stderr, "injdefense", err)
	}

	report := injdefense.Analyze(defended, baseline)

	var output string
	switch *format {
	case "json":
		output = injdefense.FormatJSON(report)
	default:
		output = injdefense.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "injdefense", err)
	}

	return ExitPass
}
