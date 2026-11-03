package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/replaydiff"
)

func init() {
	Register("replay-diff", "compare two run.json files and show semantic differences", runReplayDiff)
}

func runReplayDiff(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay replay-diff", stderr)
	baseDir := fs.String("base", "", "path to base run directory (required)")
	curDir := fs.String("current", "", "path to current run directory (required)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *baseDir == "" || *curDir == "" {
		fmt.Fprintln(stderr, "assay replay-diff: --base and --current are required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay replay-diff: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	baseRun, err := readRunReport(filepath.Join(*baseDir, runJSON))
	if err != nil {
		return fail(stderr, "replay-diff", err)
	}
	curRun, err := readRunReport(filepath.Join(*curDir, runJSON))
	if err != nil {
		return fail(stderr, "replay-diff", err)
	}

	var baseFindings, curFindings []model.Finding
	if p := filepath.Join(*baseDir, findingsJSON); fileExists(p) {
		if baseFindings, err = readFindings(p, nil); err != nil {
			return fail(stderr, "replay-diff", err)
		}
	}
	if p := filepath.Join(*curDir, findingsJSON); fileExists(p) {
		if curFindings, err = readFindings(p, nil); err != nil {
			return fail(stderr, "replay-diff", err)
		}
	}

	d := replaydiff.Diff(baseRun, curRun, baseFindings, curFindings)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return fail(stderr, "replay-diff", err)
		}
		output = string(b) + "\n"
	default:
		output = replaydiff.FormatText(d)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "replay-diff", err)
	}
	return ExitPass
}
