package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func init() {
	Register("promote", "copy a run directory as the new continuous-mode baseline", runPromote)
}

type promotedMeta struct {
	PromotedAt  string `json:"promoted_at"`
	SourceRunID string `json:"source_run_id"`
	SourceDir   string `json:"source_dir"`
}

func runPromote(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay promote", stderr)
	runDir := fs.String("run", "", "results directory to promote (required)")
	baselineDir := fs.String("baseline-dir", "", "destination directory for the baseline (required)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" || *baselineDir == "" {
		fmt.Fprintln(stderr, "assay promote: --run and --baseline-dir are required")
		fs.Usage()
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "promote", err)
	}

	if err := os.MkdirAll(*baselineDir, 0o755); err != nil {
		return fail(stderr, "promote", err)
	}

	// Copy run.json.
	src := filepath.Join(*runDir, runJSON)
	if err := copyFile(src, filepath.Join(*baselineDir, runJSON)); err != nil {
		return fail(stderr, "promote", err)
	}

	// Copy findings if present.
	src = filepath.Join(*runDir, findingsJSON)
	if fileExists(src) {
		if err := copyFile(src, filepath.Join(*baselineDir, findingsJSON)); err != nil {
			return fail(stderr, "promote", err)
		}
	}

	absSource, _ := filepath.Abs(*runDir)
	meta := promotedMeta{
		PromotedAt:  time.Now().UTC().Format(time.RFC3339),
		SourceRunID: rr.RunID,
		SourceDir:   absSource,
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fail(stderr, "promote", err)
	}
	if err := os.WriteFile(filepath.Join(*baselineDir, "promoted.json"), append(b, '\n'), 0o644); err != nil {
		return fail(stderr, "promote", err)
	}

	fmt.Fprintf(stdout, "promote: %s → %s (run_id=%s)\n", *runDir, *baselineDir, rr.RunID)
	return ExitPass
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
