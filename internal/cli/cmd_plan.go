package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/joeyvictorino/assay/internal/orchestrate"
)

func init() {
	Register("plan", "validate an orchestration plan or print its waves: validate | waves", runPlan)
}

func runPlan(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		planUsage(stderr)
		return ExitError
	}
	switch args[0] {
	case "validate":
		return planValidate(args[1:], stdout, stderr)
	case "waves":
		return planWaves(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		planUsage(stdout)
		return ExitPass
	}
	fmt.Fprintf(stderr, "assay plan: unknown action %q\n", args[0])
	planUsage(stderr)
	return ExitError
}

func planUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  assay plan validate --plan FILE")
	fmt.Fprintln(w, "  assay plan waves    --plan FILE [--json]")
	fmt.Fprintln(w, "  FILE is a YAML plan (see evals/plans/example.yaml)")
	fmt.Fprintln(w, "  exit 0 valid, 1 invalid plan, 2 error")
}

func loadPlanArg(name string, args []string, stderr io.Writer) (orchestrate.Plan, bool, int) {
	fs := newFlagSet(name, stderr)
	file := fs.String("plan", "", "plan YAML file (required)")
	asJSON := fs.Bool("json", false, "print JSON instead of text")
	if err := fs.Parse(args); err != nil {
		return orchestrate.Plan{}, false, ExitError
	}
	if *file == "" {
		fmt.Fprintf(stderr, "%s: --plan is required\n", name)
		return orchestrate.Plan{}, false, ExitError
	}
	p, err := orchestrate.Load(*file)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		if errors.Is(err, orchestrate.ErrInvalidPlan) {
			return orchestrate.Plan{}, false, ExitBlocked
		}
		return orchestrate.Plan{}, false, ExitError
	}
	return p, *asJSON, ExitPass
}

func planValidate(args []string, stdout, stderr io.Writer) int {
	p, _, code := loadPlanArg("assay plan validate", args, stderr)
	if code != ExitPass {
		return code
	}
	waves, err := p.Waves()
	if err != nil {
		fmt.Fprintf(stderr, "assay plan validate: %v\n", err)
		return ExitBlocked
	}
	fmt.Fprintf(stdout, "plan OK: %d tasks in %d waves\n", len(p.Tasks), len(waves))
	return ExitPass
}

func planWaves(args []string, stdout, stderr io.Writer) int {
	p, asJSON, code := loadPlanArg("assay plan waves", args, stderr)
	if code != ExitPass {
		return code
	}
	waves, err := p.Waves()
	if err != nil {
		fmt.Fprintf(stderr, "assay plan waves: %v\n", err)
		return ExitBlocked
	}
	if asJSON {
		if err := writeJSON("", waves, stdout); err != nil {
			return fail(stderr, "plan waves", err)
		}
		return ExitPass
	}
	for i, w := range waves {
		ids := make([]string, 0, len(w))
		for _, t := range w {
			ids = append(ids, t.ID)
		}
		fmt.Fprintf(stdout, "wave %d: %s\n", i, strings.Join(ids, ", "))
	}
	return ExitPass
}
