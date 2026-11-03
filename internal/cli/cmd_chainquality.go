package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/joeyvictorino/assay/internal/chainquality"
)

func init() {
	Register("chain-quality", "score reasoning chain quality and logical coherence", runChainQuality)
}

func runChainQuality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay chain-quality", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay chain-quality: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay chain-quality: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "chain-quality", err)
	}

	var texts []string
	for _, f := range data {
		var parts []string
		parts = append(parts, f.Summary)
		for _, e := range f.Evidence {
			if e.Marker != "" {
				parts = append(parts, e.Marker)
			}
		}
		texts = append(texts, strings.Join(parts, "\n"))
	}

	batch := chainquality.AnalyzeBatch(texts)

	var output string
	switch *format {
	case "json":
		b, _ := json.MarshalIndent(batch, "", "  ")
		output = string(b) + "\n"
	default:
		var sb strings.Builder
		sb.WriteString("Reasoning Chain Quality Analysis\n")
		sb.WriteString(strings.Repeat("=", 50) + "\n\n")
		fmt.Fprintf(&sb, "  Samples analyzed:     %d\n", len(batch.Results))
		fmt.Fprintf(&sb, "  Mean score:           %.1f\n", batch.MeanScore)
		fmt.Fprintf(&sb, "  Best grade:           %s\n", batch.BestGrade)
		fmt.Fprintf(&sb, "  Worst grade:          %s\n\n", batch.WorstGrade)
		for i, r := range batch.Results {
			fmt.Fprintf(&sb, "  Sample %d: [%s] %.1f/100\n", i+1, r.Grade, r.OverallScore)
			fmt.Fprintf(&sb, "    Weakest dimension:  %s\n", r.WeakestLink)
			fmt.Fprintf(&sb, "    Steps detected:     %d\n", r.StepCount)
			for _, d := range r.Dimensions {
				fmt.Fprintf(&sb, "    %-24s [%s] %.1f\n", d.Name, d.Grade, d.Score)
			}
			sb.WriteString("\n")
		}
		output = sb.String()
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "chain-quality", err)
	}

	return ExitPass
}
