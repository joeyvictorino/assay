package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/tokenleakage"
)

func init() {
	Register("token-leakage", "detect credentials and tokens in model outputs", runTokenLeakage)
}

func runTokenLeakage(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay token-leakage", stderr)
	samplesPath := fs.String("samples", "", "path to samples JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *samplesPath == "" {
		fmt.Fprintln(stderr, "assay token-leakage: --samples is required")
		return ExitError
	}

	raw, err := os.ReadFile(*samplesPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay token-leakage: %v\n", err)
		return ExitError
	}

	var samples []tokenleakage.Sample
	if err := json.Unmarshal(raw, &samples); err != nil {
		fmt.Fprintf(stderr, "assay token-leakage: %v\n", err)
		return ExitError
	}

	r := tokenleakage.Analyze(samples)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r)
	default:
		fmt.Fprint(stdout, tokenleakage.FormatText(r))
	}
	return ExitPass
}
