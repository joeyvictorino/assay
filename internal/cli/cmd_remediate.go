package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/remediate"
)

func init() {
	Register("remediate", "generate severity-ordered remediations for a findings file", runRemediate)
}

// runRemediate: assay remediate --findings FILE [--out FILE] [--ground-truth FILE] [--fixes-dir DIR]
func runRemediate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("remediate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	findingsPath := fs.String("findings", "", "findings JSON file (array of findings, or - for stdin)")
	outPath := fs.String("out", "", "output file (default stdout)")
	gtPath := fs.String("ground-truth", "", "ground truth JSON; findings matched by class and path get code diffs")
	fixesDir := fs.String("fixes-dir", remediate.DefaultFixesDir, "directory with <ground-truth-id>.diff files")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	findings, err := readFindings(*findingsPath, os.Stdin)
	if err != nil {
		return fail(stderr, "remediate", err)
	}
	var entries []labs.GroundTruthEntry
	if *gtPath != "" {
		entries, _, err = labs.LoadGroundTruth(*gtPath)
		if err != nil {
			return fail(stderr, "remediate", err)
		}
	}
	gen := &remediate.Generator{FixesDir: *fixesDir}
	ordered := remediate.Order(findings)
	withDiff := 0
	for i := range ordered {
		f := &ordered[i]
		var gt *labs.GroundTruthEntry
		if entries != nil {
			gt = labs.FindByClass(entries, f.Class, f.Location.PathTemplate)
		}
		rem := gen.Generate(*f, gt)
		if rem.CodeDiff != "" {
			withDiff++
		}
		f.Remediation = &rem
	}
	if err := writeJSON(*outPath, ordered, stdout); err != nil {
		return fail(stderr, "remediate", err)
	}
	fmt.Fprintf(stderr, "assay remediate: %d findings, %d with code diffs\n", len(ordered), withDiff)
	return ExitPass
}
