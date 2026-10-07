package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/chain"
)

func init() {
	Register("chain", "derive attack paths from a findings file", runChain)
}

// runChain: assay chain --findings FILE [--out FILE] [--mermaid FILE]
func runChain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("chain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	findingsPath := fs.String("findings", "", "findings JSON file (array of findings, or - for stdin)")
	outPath := fs.String("out", "", "attack paths JSON output (default stdout)")
	mermaidPath := fs.String("mermaid", "", "also write a Mermaid flowchart here (- for stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	findings, err := readFindings(*findingsPath, os.Stdin)
	if err != nil {
		return fail(stderr, "chain", err)
	}
	paths := chain.Build(findings)
	if err := writeJSON(*outPath, paths, stdout); err != nil {
		return fail(stderr, "chain", err)
	}
	if *mermaidPath != "" {
		if err := writeText(*mermaidPath, chain.Mermaid(paths), stdout); err != nil {
			return fail(stderr, "chain", err)
		}
	}
	fmt.Fprintf(stderr, "assay chain: %d findings -> %d attack paths\n", len(findings), len(paths))
	return ExitPass
}
