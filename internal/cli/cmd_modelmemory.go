package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/modelmemory"
)

func init() {
	Register("model-memory", "test context retention across conversation turns", runModelMemory)
}

func runModelMemory(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-memory", stderr)
	input := fs.String("input", "", "path to conversation JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay model-memory: --input is required")
		fs.Usage()
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		return fail(stderr, "model-memory", err)
	}

	var turns []modelmemory.Turn
	if err := json.Unmarshal(data, &turns); err != nil {
		return fail(stderr, "model-memory", fmt.Errorf("parsing turns: %w", err))
	}

	report := modelmemory.Analyze(turns)

	var output string
	switch *format {
	case "json":
		output = modelmemory.FormatJSON(report)
	default:
		output = modelmemory.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-memory", err)
	}
	return ExitPass
}
