package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/coveragemap"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("coverage-map", "show checker coverage across security areas", runCoverageMap)
}

func runCoverageMap(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay coverage-map", stderr)
	inputDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	showGaps := fs.Bool("show-gaps", false, "highlight coverage gaps only")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *inputDir == "" {
		fmt.Fprintln(stderr, "assay coverage-map: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay coverage-map: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*inputDir, runJSON))
	if err != nil {
		return fail(stderr, "coverage-map", err)
	}

	var findings []model.Finding
	if p := filepath.Join(*inputDir, findingsJSON); fileExists(p) {
		if findings, err = readFindings(p, nil); err != nil {
			return fail(stderr, "coverage-map", err)
		}
	}

	m := coveragemap.BuildMap(rr, findings)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return fail(stderr, "coverage-map", err)
		}
		output = string(b) + "\n"
	default:
		if *showGaps && len(m.Gaps) > 0 {
			output = fmt.Sprintf("Coverage gaps (%d areas with no checker):\n", len(m.Gaps))
			for _, g := range m.Gaps {
				output += fmt.Sprintf("  - %s\n", g)
			}
		} else {
			output = coveragemap.FormatText(m)
		}
	}

	w := stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return fail(stderr, "coverage-map", err)
		}
		defer f.Close()
		w = f
	}
	fmt.Fprint(w, output)
	return ExitPass
}
