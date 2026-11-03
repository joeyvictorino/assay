package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/policy"
)

func init() {
	Register("init", "scaffold a new assay project directory", runInit)
}

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay init", stderr)
	dir := fs.String("dir", ".", "target directory")
	tmpl := fs.String("template", "standard", "policy template: strict, standard, minimal")
	force := fs.Bool("force", false, "overwrite existing files")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "assay init: %v\n", err)
		return ExitError
	}

	policyFile := filepath.Join(absDir, "assay.yaml")
	manifestDir := filepath.Join(absDir, "manifests")
	stateDir := filepath.Join(absDir, ".assay")

	if !*force {
		for _, p := range []string{policyFile, manifestDir, stateDir} {
			if _, err := os.Stat(p); err == nil {
				fmt.Fprintf(stderr, "assay init: %s already exists (use --force to overwrite)\n", p)
				return ExitError
			}
		}
	}

	yml, err := policy.GenerateTemplate(*tmpl)
	if err != nil {
		fmt.Fprintf(stderr, "assay init: %v\n", err)
		return ExitError
	}

	if err := os.MkdirAll(absDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "assay init: %v\n", err)
		return ExitError
	}
	if err := os.WriteFile(policyFile, []byte(yml), 0o644); err != nil {
		fmt.Fprintf(stderr, "assay init: %v\n", err)
		return ExitError
	}

	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "assay init: %v\n", err)
		return ExitError
	}
	sample := model.ToolManifest{
		Name:         "example-tool",
		Version:      "1.0.0",
		RiskTier:     model.TierLow,
		Capabilities: []string{"http.read"},
	}
	b, _ := json.MarshalIndent(sample, "", "  ")
	if err := os.WriteFile(filepath.Join(manifestDir, "example-tool.json"), b, 0o644); err != nil {
		fmt.Fprintf(stderr, "assay init: %v\n", err)
		return ExitError
	}

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "assay init: %v\n", err)
		return ExitError
	}

	fmt.Fprintf(stdout, "Initialized assay project in %s\n", absDir)
	fmt.Fprintf(stdout, "  assay.yaml          policy (%s template)\n", *tmpl)
	fmt.Fprintf(stdout, "  manifests/          tool manifests\n")
	fmt.Fprintf(stdout, "  .assay/             local state\n")
	return ExitPass
}
