package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/checkermatrix"
)

func init() {
	Register("checker-matrix", "show which checkers ran against which models", runCheckerMatrix)
}

func runCheckerMatrix(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay checker-matrix", stderr)
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay checker-matrix: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay checker-matrix: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	p := filepath.Join(*runDir, findingsJSON)
	if !fileExists(p) {
		fmt.Fprintf(stderr, "assay checker-matrix: no findings file at %s\n", p)
		return ExitError
	}
	findings, err := readFindings(p, nil)
	if err != nil {
		return fail(stderr, "checker-matrix", err)
	}

	m := checkermatrix.Build(findings)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return fail(stderr, "checker-matrix", err)
		}
		output = string(b) + "\n"
	default:
		output = checkermatrix.FormatText(m)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "checker-matrix", err)
	}
	return ExitPass
}
