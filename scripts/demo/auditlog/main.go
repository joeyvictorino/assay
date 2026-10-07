// Command auditlog writes a small, well-formed encrypted audit log so the
// demo recording (docs/demo.tape) has something for `assay audit verify`
// to check. It is a demo helper, not part of the harness.
//
//	ASSAY_AUDIT_KEY=$(assay audit keygen) go run ./scripts/demo/auditlog -out /tmp/demo-audit.log
//
// The master key is read from the environment variable named by -key-env
// (default ASSAY_AUDIT_KEY), never from a flag.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joeyvictorino/assay/internal/audit"
	"github.com/joeyvictorino/assay/scripts/demo/auditlog/demolog"
)

func main() {
	out := flag.String("out", "demo-audit.log", "path of the log to write (must not exist)")
	keyEnv := flag.String("key-env", "ASSAY_AUDIT_KEY", "environment variable holding the base64 master key")
	runID := flag.String("run", "demo-run", "run id to record")
	flag.Parse()

	master, err := audit.MasterFromEnv(*keyEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "auditlog: %v\n", err)
		os.Exit(2)
	}
	if _, err := os.Stat(*out); err == nil {
		fmt.Fprintf(os.Stderr, "auditlog: %s already exists; refusing to append to a demo log\n", *out)
		os.Exit(2)
	}
	sum, err := demolog.Write(*out, *runID, master)
	if err != nil {
		fmt.Fprintf(os.Stderr, "auditlog: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("auditlog: wrote %s (%d records, head %s)\n", *out, sum.Records, sum.HeadHash)
}
