package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/overlap"
)

func init() {
	Register("overlap", "compute cross-model finding overlap (and precision against ground truth)", runOverlap)
}

// runOverlap implements:
//
//	assay overlap --inputs f1.json,f2.json --out FILE [--ground-truth FILE] [--check FILE]
//
// Each input is a JSON array of model.Finding; the model name is taken from
// each finding's model.model field. --check byte-compares the encoded
// report with FILE and exits 1 on any difference.
func runOverlap(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("overlap", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inputs := fs.String("inputs", "", "comma-separated finding JSON files (required)")
	out := fs.String("out", "", "output file for the overlap report (required)")
	groundTruth := fs.String("ground-truth", "", "JSON array of ground-truth entries; adds per-model precision")
	check := fs.String("check", "", "existing report to byte-compare against; exit 1 on difference")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *inputs == "" || *out == "" {
		fmt.Fprintln(stderr, "overlap: --inputs and --out are required")
		fs.Usage()
		return ExitError
	}

	perModel := map[string][]model.Finding{}
	var digests []model.FileDigest
	for _, path := range strings.Split(*inputs, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "overlap: read %s: %v\n", path, err)
			return ExitError
		}
		var findings []model.Finding
		if err := json.Unmarshal(b, &findings); err != nil {
			fmt.Fprintf(stderr, "overlap: parse %s: %v\n", path, err)
			return ExitError
		}
		sum := sha256.Sum256(b)
		digests = append(digests, model.FileDigest{Name: filepath.Base(path), SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(b))})
		if len(findings) == 0 {
			// A model that reported nothing is still in the matrix, as it is
			// in the run; its name comes from findings.<model>.json.
			name := strings.TrimPrefix(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), "findings.")
			perModel[name] = []model.Finding{}
		}
		for _, f := range findings {
			name := f.Model.Model
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			}
			perModel[name] = append(perModel[name], f)
		}
	}
	if len(digests) == 0 {
		fmt.Fprintln(stderr, "overlap: no input files")
		return ExitError
	}

	report := overlap.Report{Overlap: overlap.Compute(perModel, digests)}
	if *groundTruth != "" {
		b, err := os.ReadFile(*groundTruth)
		if err != nil {
			fmt.Fprintf(stderr, "overlap: read ground truth: %v\n", err)
			return ExitError
		}
		var gt []overlap.GroundTruthEntry
		if err := json.Unmarshal(b, &gt); err != nil {
			fmt.Fprintf(stderr, "overlap: parse ground truth: %v\n", err)
			return ExitError
		}
		report.Precision = map[string]model.PRF{}
		for name, fs := range perModel {
			report.Precision[name] = overlap.PRF(fs, gt)
		}
	}
	encoded, err := overlap.EncodeReport(report)
	if err != nil {
		fmt.Fprintf(stderr, "overlap: encode: %v\n", err)
		return ExitError
	}
	if err := os.WriteFile(*out, encoded, 0o644); err != nil {
		fmt.Fprintf(stderr, "overlap: write %s: %v\n", *out, err)
		return ExitError
	}
	m := report.Overlap
	fmt.Fprintf(stdout, "overlap: models=%d union=%d intersection=%d out=%s\n", len(m.Models), m.Union, m.Intersection, *out)
	for name, prf := range report.Precision {
		fmt.Fprintf(stdout, "  %s: tp=%d fp=%d fn=%d precision=%.3f recall=%.3f f1=%.3f\n", name, prf.TP, prf.FP, prf.FN, prf.Precision, prf.Recall, prf.F1)
	}

	if *check != "" {
		want, err := os.ReadFile(*check)
		if err != nil {
			fmt.Fprintf(stderr, "overlap: read check file: %v\n", err)
			return ExitError
		}
		if !bytes.Equal(want, encoded) {
			fmt.Fprintf(stderr, "overlap: %s differs from freshly computed report (%d vs %d bytes)\n", *check, len(want), len(encoded))
			return ExitBlocked
		}
		fmt.Fprintf(stdout, "overlap: check OK (%s matches)\n", *check)
	}
	return ExitPass
}
