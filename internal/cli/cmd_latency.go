package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/latency"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("latency", "profile model latency and detect bottlenecks", runLatency)
}

func runLatency(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("latency", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("path", "", "path to run report JSON (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" {
		fmt.Fprintln(stderr, "error: --path is required")
		return 2
	}

	data, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintf(stderr, "error reading file: %v\n", err)
		return 1
	}
	var rr model.RunReport
	if err := json.Unmarshal(data, &rr); err != nil {
		fmt.Fprintf(stderr, "error parsing JSON: %v\n", err)
		return 1
	}

	report := latency.Analyze(rr)

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "error encoding output: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprint(stdout, latency.FormatText(report))
	}
	return 0
}
