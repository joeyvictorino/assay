package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

func init() {
	Register("findings-dedup", "deduplicate findings by configurable key fields", runFindingsDedup)
}

func runFindingsDedup(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-dedup", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	output := fs.String("output", "", "write deduplicated findings to file (default: stdout)")
	keys := fs.String("keys", "checker,class,severity,message", "comma-separated fields for dedup key")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-dedup: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-dedup: %v\n", err)
		return ExitError
	}

	var findings []map[string]any
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-dedup: invalid JSON: %v\n", err)
		return ExitError
	}

	keyFields := strings.Split(*keys, ",")
	for i := range keyFields {
		keyFields[i] = strings.TrimSpace(keyFields[i])
	}

	seen := make(map[string]bool)
	var deduped []map[string]any
	var duplicateCount int

	for _, f := range findings {
		h := dedupHash(f, keyFields)
		if seen[h] {
			duplicateCount++
			continue
		}
		seen[h] = true
		deduped = append(deduped, f)
	}

	result := map[string]any{
		"total":      len(findings),
		"unique":     len(deduped),
		"duplicates": duplicateCount,
		"key_fields": keyFields,
		"findings":   deduped,
	}

	if *output != "" {
		out, _ := json.MarshalIndent(deduped, "", "  ")
		if err := os.WriteFile(*output, out, 0644); err != nil {
			fmt.Fprintf(stderr, "assay findings-dedup: write: %v\n", err)
			return ExitError
		}
	}

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		fmt.Fprintf(stdout, "Findings Deduplication\n")
		fmt.Fprintf(stdout, "Key fields: %s\n", strings.Join(keyFields, ", "))
		fmt.Fprintf(stdout, "Total:      %d\n", len(findings))
		fmt.Fprintf(stdout, "Unique:     %d\n", len(deduped))
		fmt.Fprintf(stdout, "Duplicates: %d\n", duplicateCount)
	}
	return ExitPass
}

func dedupHash(f map[string]any, keys []string) string {
	var parts []string
	for _, k := range keys {
		v, ok := f[k]
		if !ok {
			parts = append(parts, k+"=<nil>")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(parts)
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return fmt.Sprintf("%x", h[:16])
}
