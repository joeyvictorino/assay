package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/threshold"
)

func init() {
	Register("threshold", "manage and enforce score thresholds", runThreshold)
}

func runThreshold(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		thresholdUsage(stderr)
		return ExitError
	}
	sub := args[0]
	rest := args[1:]

	switch sub {
	case "check":
		return thresholdCheck(rest, stdout, stderr)
	case "init":
		return thresholdInit(rest, stdout, stderr)
	case "show":
		return thresholdShow(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "assay threshold: unknown subcommand %q\n", sub)
		thresholdUsage(stderr)
		return ExitError
	}
}

func thresholdUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: assay threshold <subcommand> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  check   check a run against threshold config (exit 1 on violations)")
	fmt.Fprintln(w, "  init    generate a default threshold config file")
	fmt.Fprintln(w, "  show    display a threshold config")
}

func thresholdCheck(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay threshold check", stderr)
	configPath := fs.String("config", "", "path to threshold config JSON (required)")
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *configPath == "" || *runDir == "" {
		fmt.Fprintln(stderr, "assay threshold check: --config and --path are required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay threshold check: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	cfg, err := threshold.LoadConfig(*configPath)
	if err != nil {
		return fail(stderr, "threshold check", err)
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "threshold check", err)
	}

	var findings []model.Finding
	if p := filepath.Join(*runDir, findingsJSON); fileExists(p) {
		if findings, err = readFindings(p, nil); err != nil {
			return fail(stderr, "threshold check", err)
		}
	}

	result := threshold.Check(cfg, rr, findings)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fail(stderr, "threshold check", err)
		}
		output = string(b) + "\n"
	default:
		output = threshold.FormatText(result)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "threshold check", err)
	}
	if !result.Passed {
		return ExitFail
	}
	return ExitPass
}

func thresholdInit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay threshold init", stderr)
	out := fs.String("output", "", "path to write config file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	cfg := threshold.GenerateDefault()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fail(stderr, "threshold init", err)
	}
	output := string(b) + "\n"

	if *out != "" {
		if err := os.WriteFile(*out, []byte(output), 0644); err != nil {
			return fail(stderr, "threshold init", err)
		}
		fmt.Fprintf(stdout, "threshold: wrote default config to %s\n", *out)
		return ExitPass
	}
	fmt.Fprint(stdout, output)
	return ExitPass
}

func thresholdShow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay threshold show", stderr)
	configPath := fs.String("config", "", "path to threshold config JSON (required)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *configPath == "" {
		fmt.Fprintln(stderr, "assay threshold show: --config is required")
		fs.Usage()
		return ExitError
	}

	cfg, err := threshold.LoadConfig(*configPath)
	if err != nil {
		return fail(stderr, "threshold show", err)
	}

	fmt.Fprint(stdout, threshold.FormatConfig(cfg))
	return ExitPass
}
