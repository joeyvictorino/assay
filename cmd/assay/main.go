// Command assay is a multi-model continuous security-validation harness.
//
// Exit codes: 0 pass, 1 blocked (regression, eval miss, incomplete run),
// 2 tool or configuration error (gate denial, parse error, unsigned manifest,
// audit chain break). See docs/adr for the design record.
package main

import (
	"os"

	"github.com/joeyvictorino/assay/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
