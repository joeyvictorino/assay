package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/reconcile"
)

func init() {
	Register("reconcile", "join claimed tool calls against the control plane and downgrade unsupported findings", runReconcile)
}

// runReconcile implements:
//
//	assay reconcile --audit-export FILE --findings FILE --out DIR
//
// FILE for --audit-export is the JSON-lines output of `assay audit export`
// (decrypted model.AuditRecord per line). The command writes
// DIR/reconcile.json and DIR/findings.reconciled.json. Exit 0 when every
// tool call id reconciles, 1 when any mismatch was found (findings were
// downgraded), 2 on a tool or input error.
func runReconcile(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay reconcile", stderr)
	exportPath := fs.String("audit-export", "", "decrypted audit export (JSON lines) from `assay audit export`")
	findingsPath := fs.String("findings", "", "JSON array of findings to annotate (\"-\" for stdin)")
	outDir := fs.String("out", "", "directory to write reconcile.json and findings.reconciled.json into")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *exportPath == "" || *findingsPath == "" || *outDir == "" {
		fmt.Fprintln(stderr, "assay reconcile: --audit-export, --findings and --out are required")
		fs.Usage()
		return ExitError
	}
	records, err := readAuditExport(*exportPath)
	if err != nil {
		return fail(stderr, "reconcile", err)
	}
	findings, err := readFindings(*findingsPath, os.Stdin)
	if err != nil {
		return fail(stderr, "reconcile", err)
	}
	rep := reconcile.Reconcile(records)
	downgraded := reconcile.Downgrade(findings, rep)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fail(stderr, "reconcile", err)
	}
	encoded, err := reconcile.Encode(rep)
	if err != nil {
		return fail(stderr, "reconcile", err)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "reconcile.json"), encoded, 0o644); err != nil {
		return fail(stderr, "reconcile", err)
	}
	if err := writeJSON(filepath.Join(*outDir, "findings.reconciled.json"), downgraded, stdout); err != nil {
		return fail(stderr, "reconcile", err)
	}
	changed := 0
	for _, f := range downgraded {
		if !f.Reconcile.Match {
			changed++
		}
	}
	fmt.Fprintf(stdout, "reconcile: records=%d checked=%d mismatches=%d findings=%d downgraded=%d out=%s\n",
		len(records), rep.Checked, len(rep.Mismatches), len(downgraded), changed, *outDir)
	if len(rep.Mismatches) > 0 {
		return ExitBlocked
	}
	return ExitPass
}

// readAuditExport parses the JSON-lines output of `assay audit export`.
// Blank lines are skipped; any other unparsable line is an error with its
// line number.
func readAuditExport(path string) ([]model.AuditRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []model.AuditRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		var rec model.AuditRecord
		if err := json.Unmarshal(b, &rec); err != nil {
			return nil, fmt.Errorf("audit export line %d: %w", line, err)
		}
		if rec.Kind == "" {
			return nil, fmt.Errorf("audit export line %d: record has no kind", line)
		}
		out = append(out, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("audit export is empty")
	}
	return out, nil
}
