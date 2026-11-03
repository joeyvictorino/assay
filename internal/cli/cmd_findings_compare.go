package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func init() {
	Register("findings-compare", "compare findings between two run files", runFindingsCompare)
}

type findingsDiff struct {
	FileA       string           `json:"file_a"`
	FileB       string           `json:"file_b"`
	CountA      int              `json:"count_a"`
	CountB      int              `json:"count_b"`
	OnlyInA     []map[string]any `json:"only_in_a"`
	OnlyInB     []map[string]any `json:"only_in_b"`
	Common      int              `json:"common"`
	AddedCount  int              `json:"added_count"`
	RemovedCount int             `json:"removed_count"`
}

func findingFingerprint(f map[string]any) string {
	return fmt.Sprintf("%s:%s:%s:%s", f["checker"], f["class"], f["severity"], f["message"])
}

func runFindingsCompare(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-compare", stderr)
	fileA := fs.String("a", "", "first findings JSON file (required)")
	fileB := fs.String("b", "", "second findings JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *fileA == "" || *fileB == "" {
		fmt.Fprintln(stderr, "assay findings-compare: --a and --b are required")
		return ExitError
	}

	loadFindings := func(path string) ([]map[string]any, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var findings []map[string]any
		if err := json.Unmarshal(data, &findings); err != nil {
			return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
		}
		return findings, nil
	}

	findingsA, err := loadFindings(*fileA)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-compare: %v\n", err)
		return ExitError
	}
	findingsB, err := loadFindings(*fileB)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-compare: %v\n", err)
		return ExitError
	}

	fpA := make(map[string]map[string]any)
	for _, f := range findingsA {
		fpA[findingFingerprint(f)] = f
	}
	fpB := make(map[string]map[string]any)
	for _, f := range findingsB {
		fpB[findingFingerprint(f)] = f
	}

	var onlyA, onlyB []map[string]any
	common := 0

	for fp, f := range fpA {
		if _, ok := fpB[fp]; ok {
			common++
		} else {
			onlyA = append(onlyA, f)
		}
	}
	for fp, f := range fpB {
		if _, ok := fpA[fp]; !ok {
			onlyB = append(onlyB, f)
		}
		_ = fp
	}

	diff := findingsDiff{
		FileA:        *fileA,
		FileB:        *fileB,
		CountA:       len(findingsA),
		CountB:       len(findingsB),
		OnlyInA:      onlyA,
		OnlyInB:      onlyB,
		Common:       common,
		AddedCount:   len(onlyB),
		RemovedCount: len(onlyA),
	}

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(diff)
	} else {
		fmt.Fprintf(stdout, "Findings Comparison\n")
		fmt.Fprintf(stdout, "A: %s (%d findings)\n", diff.FileA, diff.CountA)
		fmt.Fprintf(stdout, "B: %s (%d findings)\n\n", diff.FileB, diff.CountB)
		fmt.Fprintf(stdout, "Common:  %d\n", diff.Common)
		fmt.Fprintf(stdout, "Added:   %d (in B only)\n", diff.AddedCount)
		fmt.Fprintf(stdout, "Removed: %d (in A only)\n", diff.RemovedCount)

		if len(onlyA) > 0 {
			fmt.Fprintf(stdout, "\nRemoved findings:\n")
			for _, f := range onlyA {
				sev := strings.ToLower(fmt.Sprint(f["severity"]))
				fmt.Fprintf(stdout, "  - [%s] %s: %s\n", sev, f["checker"], f["message"])
			}
		}
		if len(onlyB) > 0 {
			fmt.Fprintf(stdout, "\nAdded findings:\n")
			for _, f := range onlyB {
				sev := strings.ToLower(fmt.Sprint(f["severity"]))
				fmt.Fprintf(stdout, "  + [%s] %s: %s\n", sev, f["checker"], f["message"])
			}
		}
	}
	return ExitPass
}
