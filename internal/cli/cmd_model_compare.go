package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/modelcompare"
)

func init() {
	Register("model-compare", "compare findings from two model runs side by side", runModelCompare)
}

func runModelCompare(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-compare", stderr)
	dirA := fs.String("run-a", "", "path to first run directory (required)")
	dirB := fs.String("run-b", "", "path to second run directory (required)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *dirA == "" || *dirB == "" {
		fmt.Fprintln(stderr, "assay model-compare: --run-a and --run-b are required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-compare: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	a, err := modelcompare.LoadRun(filepath.Join(*dirA, runJSON))
	if err != nil {
		return fail(stderr, "model-compare", err)
	}
	b, err := modelcompare.LoadRun(filepath.Join(*dirB, runJSON))
	if err != nil {
		return fail(stderr, "model-compare", err)
	}

	if p := filepath.Join(*dirA, findingsJSON); fileExists(p) {
		if a.Findings, err = modelcompare.LoadFindings(p); err != nil {
			return fail(stderr, "model-compare", err)
		}
	}
	if p := filepath.Join(*dirB, findingsJSON); fileExists(p) {
		if b.Findings, err = modelcompare.LoadFindings(p); err != nil {
			return fail(stderr, "model-compare", err)
		}
	}

	result := modelcompare.Compare(a, b)

	var output string
	switch *format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fail(stderr, "model-compare", err)
		}
		output = string(data) + "\n"
	default:
		output = modelcompare.FormatText(result)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-compare", err)
	}
	return ExitPass
}
