package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/modeldrift"
)

func init() {
	Register("model-drift", "detect behavioral drift across sequential runs", runModelDrift)
}

func runModelDrift(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-drift", stderr)
	snapFile := fs.String("snapshots", "", "JSON file with array of run snapshots (required)")
	format := fs.String("format", "text", "text | json")
	threshold := fs.Float64("threshold", 0.3, "drift threshold for flagging (0.0-1.0)")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-drift: unknown --format %q (text | json)\n", *format)
		return ExitError
	}
	if *snapFile == "" {
		fmt.Fprintln(stderr, "assay model-drift: --snapshots is required")
		return ExitError
	}

	data, err := os.ReadFile(*snapFile)
	if err != nil {
		return fail(stderr, "model-drift", err)
	}

	var snapshots []modeldrift.RunSnapshot
	if err := json.Unmarshal(data, &snapshots); err != nil {
		return fail(stderr, "model-drift", fmt.Errorf("parse snapshots: %w", err))
	}

	report := modeldrift.Analyze(snapshots, *threshold)

	var output string
	switch *format {
	case "json":
		output = modeldrift.FormatJSON(report)
	default:
		output = modeldrift.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-drift", err)
	}

	return ExitPass
}
