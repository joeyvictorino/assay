// Command summary prints a short plain-English summary of one results
// directory written by `assay run`. It is the last step of scripts/demo.sh
// and a demo helper, not part of the harness.
//
//	go run ./scripts/demo/summary out/demo
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: summary RESULTS_DIR")
		os.Exit(2)
	}
	if err := summarize(os.Args[1], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "summary: %v\n", err)
		os.Exit(2)
	}
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func summarize(dir string, w io.Writer) error {
	var rr model.RunReport
	if err := readJSON(filepath.Join(dir, "run.json"), &rr); err != nil {
		return err
	}
	var findings []model.Finding
	if err := readJSON(filepath.Join(dir, "findings.redacted.json"), &findings); err != nil {
		return err
	}
	var rec struct {
		Checked    int               `json:"checked"`
		Mismatches []json.RawMessage `json:"mismatches"`
	}
	if err := readJSON(filepath.Join(dir, "reconcile.json"), &rec); err != nil {
		return err
	}

	fmt.Fprintf(w, "  run         %s: %s (config %s)\n", rr.RunID, rr.Verdict, rr.Config)
	fmt.Fprintf(w, "  target      %s\n", strings.Join(rr.Labs, ", "))
	var names []string
	for _, m := range rr.Models {
		name := m.Ref.Model
		if m.Ref.Provider == "fake" {
			name += " (scripted stand-in, no real model called)"
		}
		names = append(names, name)
	}
	fmt.Fprintf(w, "  model       %s; spent $%.4f of a $%.0f cap\n", strings.Join(names, ", "), rr.SpentUSD, rr.BudgetCapUSD)

	counts := map[model.FindingState]int{}
	for _, f := range findings {
		counts[f.State]++
	}
	fmt.Fprintf(w, "  findings    %d reported: %d validated, %d theorized, %d refuted\n",
		len(findings), counts[model.StateValidated], counts[model.StateTheorized], counts[model.StateRefuted])
	for _, f := range findings {
		n := 0
		if f.Validation != nil {
			n = len(f.Validation.Evidence)
		}
		fmt.Fprintf(w, "              %-10s %-18s %-4s %-14s %s, %d checker observations\n",
			f.State, f.Class, f.Location.Method, f.Location.PathTemplate, f.Severity, n)
	}
	fmt.Fprintf(w, "  tool calls  %d claimed by the model, each matched against the control-plane log; %d mismatches\n",
		rec.Checked, len(rec.Mismatches))
	for key, p := range rr.Precision {
		planted := p.TP + p.FN
		fmt.Fprintf(w, "  vs. truth   %s: %d of %d planted weaknesses found, %d false positives\n", key, p.TP, planted, p.FP)
	}
	fmt.Fprintf(w, "  audit log   %d records, hash-chain head %s\n", rr.AuditCount, short(rr.AuditHead))
	return nil
}

func short(h string) string {
	if len(h) > 16 {
		return h[:16] + "..."
	}
	return h
}
