package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/modelrank"
)

func init() {
	Register("model-rank", "rank models by aggregate checker scores", runModelRank)
}

func runModelRank(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-rank", stderr)
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	top := fs.String("top", "", "show top N models only")
	checker := fs.String("checker", "", "filter to a specific checker")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay model-rank: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-rank: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "model-rank", err)
	}

	var findings []model.Finding
	if p := filepath.Join(*runDir, findingsJSON); fileExists(p) {
		if findings, err = readFindings(p, nil); err != nil {
			return fail(stderr, "model-rank", err)
		}
	}

	lb := modelrank.BuildLeaderboard(rr, findings)

	if *checker != "" {
		lb = modelrank.FilterByChecker(lb, *checker)
	}
	if *top != "" {
		n, err := strconv.Atoi(*top)
		if err != nil {
			fmt.Fprintf(stderr, "assay model-rank: --top must be an integer: %v\n", err)
			return ExitError
		}
		lb = modelrank.TopN(lb, n)
	}

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(lb, "", "  ")
		if err != nil {
			return fail(stderr, "model-rank", err)
		}
		output = string(b) + "\n"
	default:
		output = modelrank.FormatText(lb)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-rank", err)
	}
	return ExitPass
}
