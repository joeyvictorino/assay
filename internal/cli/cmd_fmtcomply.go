package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/fmtcomply"
)

func init() {
	Register("format-compliance", "check model output format compliance", runFmtComply)
}

func runFmtComply(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay format-compliance", stderr)
	samplesPath := fs.String("samples", "", "path to samples JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *samplesPath == "" {
		fmt.Fprintln(stderr, "assay format-compliance: --samples is required")
		return ExitError
	}

	raw, err := os.ReadFile(*samplesPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay format-compliance: %v\n", err)
		return ExitError
	}

	var samples []fmtcomply.Sample
	if err := json.Unmarshal(raw, &samples); err != nil {
		fmt.Fprintf(stderr, "assay format-compliance: %v\n", err)
		return ExitError
	}

	r := fmtcomply.Analyze(samples)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r)
	default:
		fmt.Fprint(stdout, fmtcomply.FormatText(r))
	}
	return ExitPass
}
