package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/report"
)

func init() {
	Register("schema", "print the JSON Schema for run.json or findings.redacted.json", runSchema)
}

func runSchema(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay schema", stderr)
	which := fs.String("type", "run", "schema to print: run | findings")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *which {
	case "run":
		fmt.Fprintln(stdout, report.RunJSONSchema)
	case "findings":
		fmt.Fprintln(stdout, report.FindingsJSONSchema)
	default:
		fmt.Fprintf(stderr, "assay schema: unknown --type %q (run | findings)\n", *which)
		return ExitError
	}
	return ExitPass
}
