package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/outlier"
)

func init() {
	Register("outlier", "detect statistical outliers in checker scores", runOutlier)
}

func runOutlier(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay outlier", stderr)
	inputDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	thresholdStr := fs.String("threshold", "2.0", "z-score threshold for outlier detection")
	filterType := fs.String("type", "all", "filter outlier type: high, low, or all")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *inputDir == "" {
		fmt.Fprintln(stderr, "assay outlier: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay outlier: unknown --format %q (text | json)\n", *format)
		return ExitError
	}
	switch *filterType {
	case "all", "high", "low":
	default:
		fmt.Fprintf(stderr, "assay outlier: unknown --type %q (all | high | low)\n", *filterType)
		return ExitError
	}

	threshold, err := strconv.ParseFloat(*thresholdStr, 64)
	if err != nil || threshold <= 0 {
		fmt.Fprintf(stderr, "assay outlier: --threshold must be a positive number\n")
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*inputDir, runJSON))
	if err != nil {
		return fail(stderr, "outlier", err)
	}

	var findings []model.Finding
	if p := filepath.Join(*inputDir, findingsJSON); fileExists(p) {
		if findings, err = readFindings(p, nil); err != nil {
			return fail(stderr, "outlier", err)
		}
	}

	report := outlier.DetectOutliers(rr, findings, threshold)

	if *filterType != "all" {
		var filtered []outlier.Outlier
		for _, o := range report.Outliers {
			if o.Type == *filterType {
				filtered = append(filtered, o)
			}
		}
		report.Outliers = filtered
	}

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "outlier", err)
		}
		output = string(b) + "\n"
	default:
		output = outlier.FormatText(report)
	}

	w := stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return fail(stderr, "outlier", err)
		}
		defer f.Close()
		w = f
	}
	fmt.Fprint(w, output)
	return ExitPass
}
