package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/report"
)

func init() {
	Register("schema", "print the JSON Schema for run.json", runSchema)
}

func runSchema(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay schema", stderr)
	which := fs.String("type", "run", "schema to print: run")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *which {
	case "run":
		fmt.Fprintln(stdout, report.RunJSONSchema)
	default:
		fmt.Fprintf(stderr, "assay schema: unknown --type %q (run)\n", *which)
		return ExitError
	}
	return ExitPass
}
