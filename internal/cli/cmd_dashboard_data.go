package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/dashdata"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("dashboard-data", "generate dashboard JSON from a run", runDashboardData)
}

func runDashboardData(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay dashboard-data", stderr)
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	prevDir := fs.String("prev", "", "path to previous run directory for trend comparison")
	output := fs.String("output", "", "write JSON to file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay dashboard-data: --path is required")
		fs.Usage()
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "dashboard-data", err)
	}

	var prev *model.RunReport
	if *prevDir != "" {
		p, err := readRunReport(filepath.Join(*prevDir, runJSON))
		if err != nil {
			return fail(stderr, "dashboard-data", err)
		}
		prev = &p
	}

	d := dashdata.Generate(rr, prev)

	if err := writeJSON(*output, d, stdout); err != nil {
		return fail(stderr, "dashboard-data", err)
	}
	return ExitPass
}
