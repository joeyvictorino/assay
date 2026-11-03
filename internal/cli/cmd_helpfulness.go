package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/helpfulness"
)

func init() {
	Register("helpfulness", "score model response helpfulness and directness", runHelpfulness)
}

func runHelpfulness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay helpfulness", stderr)
	samplesPath := fs.String("samples", "", "path to samples JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *samplesPath == "" {
		fmt.Fprintln(stderr, "assay helpfulness: --samples is required")
		return ExitError
	}

	raw, err := os.ReadFile(*samplesPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay helpfulness: %v\n", err)
		return ExitError
	}

	var samples []helpfulness.Sample
	if err := json.Unmarshal(raw, &samples); err != nil {
		fmt.Fprintf(stderr, "assay helpfulness: %v\n", err)
		return ExitError
	}

	r := helpfulness.Analyze(samples)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r)
	default:
		fmt.Fprint(stdout, helpfulness.FormatText(r))
	}
	return ExitPass
}
