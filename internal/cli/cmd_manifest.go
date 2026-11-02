package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("manifest", "manage tool manifests: create | validate | list", runManifest)
}

func runManifest(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		manifestUsage(stderr)
		return ExitError
	}
	switch args[0] {
	case "create":
		return manifestCreate(args[1:], stdout, stderr)
	case "validate":
		return manifestValidate(args[1:], stdout, stderr)
	case "list":
		return manifestList(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		manifestUsage(stdout)
		return ExitPass
	}
	fmt.Fprintf(stderr, "assay manifest: unknown action %q\n", args[0])
	manifestUsage(stderr)
	return ExitError
}

func manifestUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  assay manifest create --name NAME --capabilities CAP1,CAP2 [--version V] [--out FILE]")
	fmt.Fprintln(w, "  assay manifest validate --manifest FILE")
	fmt.Fprintln(w, "  assay manifest list --manifests DIR [--format text|json]")
}

func manifestCreate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay manifest create", stderr)
	name := fs.String("name", "", "tool name (required)")
	caps := fs.String("capabilities", "", "comma-separated capabilities (required)")
	version := fs.String("version", "1.0.0", "tool version")
	outPath := fs.String("out", "-", "output path (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *name == "" || *caps == "" {
		fmt.Fprintln(stderr, "assay manifest create: --name and --capabilities are required")
		return ExitError
	}

	capList := splitTrimmed(*caps)

	m := model.ToolManifest{
		Name:        *name,
		Version:     *version,
		Description: fmt.Sprintf("Tool manifest for %s", *name),
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		RiskTier:     model.TierMedium,
		Capabilities: capList,
		SideEffects:  false,
		TimeoutMS:    30000,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		fmt.Fprintf(stderr, "assay manifest create: %v\n", err)
		return ExitError
	}

	if *outPath == "-" {
		stdout.Write(buf.Bytes())
	} else {
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
			fmt.Fprintf(stderr, "assay manifest create: %v\n", err)
			return ExitError
		}
		if err := os.WriteFile(*outPath, buf.Bytes(), 0o644); err != nil {
			fmt.Fprintf(stderr, "assay manifest create: %v\n", err)
			return ExitError
		}
		fmt.Fprintf(stderr, "assay manifest create: wrote %s\n", *outPath)
	}
	return ExitPass
}

type manifestIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func manifestValidate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay manifest validate", stderr)
	manifestPath := fs.String("manifest", "", "manifest JSON file (required)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *manifestPath == "" {
		fmt.Fprintln(stderr, "assay manifest validate: --manifest is required")
		return ExitError
	}

	raw, err := os.ReadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay manifest validate: %v\n", err)
		return ExitError
	}

	var m model.ToolManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		fmt.Fprintf(stderr, "assay manifest validate: %s: %v\n", *manifestPath, err)
		return ExitError
	}

	var issues []manifestIssue
	if m.Name == "" {
		issues = append(issues, manifestIssue{Field: "name", Message: "required field is empty"})
	}
	if m.Version == "" {
		issues = append(issues, manifestIssue{Field: "version", Message: "required field is empty"})
	}
	if m.Description == "" {
		issues = append(issues, manifestIssue{Field: "description", Message: "required field is empty"})
	}
	if m.RiskTier == "" {
		issues = append(issues, manifestIssue{Field: "risk_tier", Message: "required field is empty"})
	} else {
		validTiers := map[model.RiskTier]bool{model.TierLow: true, model.TierMedium: true, model.TierHigh: true, model.TierCritical: true}
		if !validTiers[m.RiskTier] {
			issues = append(issues, manifestIssue{Field: "risk_tier", Message: fmt.Sprintf("invalid value %q", m.RiskTier)})
		}
	}
	if len(m.Capabilities) == 0 {
		issues = append(issues, manifestIssue{Field: "capabilities", Message: "should have at least one capability"})
	}
	if m.TimeoutMS <= 0 {
		issues = append(issues, manifestIssue{Field: "timeout_ms", Message: "should be positive"})
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(issues); err != nil {
		fmt.Fprintf(stderr, "assay manifest validate: %v\n", err)
		return ExitError
	}

	if len(issues) == 0 {
		fmt.Fprintln(stderr, "assay manifest validate: valid")
		return ExitPass
	}
	fmt.Fprintf(stderr, "assay manifest validate: %d issue(s) found\n", len(issues))
	return ExitBlocked
}

type manifestEntry struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	RiskTier     string   `json:"risk_tier"`
	Capabilities []string `json:"capabilities"`
	File         string   `json:"file"`
}

func manifestList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay manifest list", stderr)
	manifestsDir := fs.String("manifests", "", "directory of manifest JSON files (required)")
	format := fs.String("format", "text", "output format: text (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *manifestsDir == "" {
		fmt.Fprintln(stderr, "assay manifest list: --manifests is required")
		return ExitError
	}

	files, err := loadManifestDir(*manifestsDir)
	if err != nil {
		fmt.Fprintf(stderr, "assay manifest list: %v\n", err)
		return ExitError
	}

	var entries []manifestEntry
	for _, f := range files {
		entries = append(entries, manifestEntry{
			Name:         f.M.Name,
			Version:      f.M.Version,
			RiskTier:     string(f.M.RiskTier),
			Capabilities: f.M.Capabilities,
			File:         f.Path,
		})
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			fmt.Fprintf(stderr, "assay manifest list: %v\n", err)
			return ExitError
		}
	case "text":
		if len(entries) == 0 {
			fmt.Fprintln(stdout, "No manifests found.")
		} else {
			for _, e := range entries {
				fmt.Fprintf(stdout, "%-20s v%-8s %-8s %s\n",
					e.Name, e.Version, e.RiskTier, strings.Join(e.Capabilities, ", "))
			}
			fmt.Fprintf(stdout, "\n%d manifest(s) found.\n", len(entries))
		}
	default:
		fmt.Fprintf(stderr, "assay manifest list: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}

func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
