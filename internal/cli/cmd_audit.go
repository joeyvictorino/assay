package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/audit"
)

func init() {
	Register("audit", "verify or export the encrypted audit log: verify | export | keygen", runAudit)
}

func runAudit(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		auditUsage(stderr)
		return ExitError
	}
	switch args[0] {
	case "verify":
		return auditVerify(args[1:], stdout, stderr)
	case "export":
		return auditExport(args[1:], stdout, stderr)
	case "keygen":
		return auditKeygen(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		auditUsage(stdout)
		return ExitPass
	}
	fmt.Fprintf(stderr, "assay audit: unknown action %q\n", args[0])
	auditUsage(stderr)
	return ExitError
}

func auditUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  assay audit verify --log FILE [--key-env NAME]   check the chain; with a key also decrypt and count")
	fmt.Fprintln(w, "  assay audit export --log FILE --key-env NAME     write decrypted records as JSON lines to stdout")
	fmt.Fprintln(w, "  assay audit keygen                               print a new base64 32-byte master key")
	fmt.Fprintln(w, "  the master key is read from the named environment variable, never from a flag")
}

func auditVerify(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay audit verify", stderr)
	logPath := fs.String("log", "", "audit log file")
	keyEnv := fs.String("key-env", "", "environment variable holding the master key (optional)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *logPath == "" {
		fmt.Fprintln(stderr, "assay audit verify: --log is required")
		return ExitError
	}
	var master []byte
	if *keyEnv != "" {
		var err error
		master, err = audit.MasterFromEnv(*keyEnv)
		if err != nil {
			fmt.Fprintf(stderr, "assay audit verify: %v\n", err)
			return ExitError
		}
	}
	sum, err := audit.Verify(*logPath, master)
	if err != nil {
		fmt.Fprintf(stderr, "assay audit verify: FAIL: %v\n", err)
		return ExitError
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sum); err != nil {
		fmt.Fprintf(stderr, "assay audit verify: %v\n", err)
		return ExitError
	}
	return ExitPass
}

func auditExport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay audit export", stderr)
	logPath := fs.String("log", "", "audit log file")
	keyEnv := fs.String("key-env", "", "environment variable holding the master key")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *logPath == "" || *keyEnv == "" {
		fmt.Fprintln(stderr, "assay audit export: --log and --key-env are required")
		return ExitError
	}
	master, err := audit.MasterFromEnv(*keyEnv)
	if err != nil {
		fmt.Fprintf(stderr, "assay audit export: %v\n", err)
		return ExitError
	}
	sum, err := audit.Export(*logPath, master, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "assay audit export: FAIL: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(stderr, "exported %d records, head %s\n", sum.Records, sum.HeadHash)
	return ExitPass
}

func auditKeygen(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay audit keygen", stderr)
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "assay audit keygen: takes no arguments")
		return ExitError
	}
	fmt.Fprintln(stdout, audit.EncodeMaster(audit.GenerateMaster()))
	return ExitPass
}
