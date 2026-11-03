package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeyvictorino/assay/internal/policy"
	"github.com/joeyvictorino/assay/internal/validate"
	"github.com/joeyvictorino/assay/internal/version"
)

func init() {
	Register("env", "inspect the assay environment: checkers, policy, manifests", runEnv)
}

// EnvReport summarizes the current assay working environment.
type EnvReport struct {
	Version       string   `json:"version"`
	CheckerCount  int      `json:"checker_count"`
	Checkers      []string `json:"checkers"`
	PolicyValid   bool     `json:"policy_valid"`
	PolicyPath    string   `json:"policy_path,omitempty"`
	ManifestCount int      `json:"manifest_count"`
	ManifestDir   string   `json:"manifest_dir,omitempty"`
	Issues        []string `json:"issues,omitempty"`
}

func runEnv(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay env", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (optional)")
	manifestsDir := fs.String("manifests", "", "directory of manifest JSON files (optional)")
	format := fs.String("format", "text", "output format: text (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	report := EnvReport{
		Version: version.Version,
	}

	infos := validate.CheckerInfos()
	report.CheckerCount = len(infos)
	for _, info := range infos {
		report.Checkers = append(report.Checkers, string(info.Class))
	}

	if *policyPath != "" {
		report.PolicyPath = *policyPath
		_, err := policy.Load(*policyPath)
		if err != nil {
			report.PolicyValid = false
			report.Issues = append(report.Issues, fmt.Sprintf("policy: %v", err))
		} else {
			report.PolicyValid = true
		}
	} else {
		defaults := []string{"policy.yaml", "assay-policy.yaml", ".assay/policy.yaml"}
		found := false
		for _, d := range defaults {
			if _, err := os.Stat(d); err == nil {
				report.PolicyPath = d
				_, err := policy.Load(d)
				if err != nil {
					report.PolicyValid = false
					report.Issues = append(report.Issues, fmt.Sprintf("policy: %v", err))
				} else {
					report.PolicyValid = true
				}
				found = true
				break
			}
		}
		if !found {
			report.Issues = append(report.Issues, "no policy file found (specify with --policy)")
		}
	}

	if *manifestsDir != "" {
		report.ManifestDir = *manifestsDir
		count, err := countManifests(*manifestsDir)
		if err != nil {
			report.Issues = append(report.Issues, fmt.Sprintf("manifests: %v", err))
		} else {
			report.ManifestCount = count
			if count == 0 {
				report.Issues = append(report.Issues, "manifests directory is empty")
			}
		}
	} else {
		defaults := []string{"manifests", ".assay/manifests", "tools"}
		found := false
		for _, d := range defaults {
			if info, err := os.Stat(d); err == nil && info.IsDir() {
				report.ManifestDir = d
				count, err := countManifests(d)
				if err != nil {
					report.Issues = append(report.Issues, fmt.Sprintf("manifests: %v", err))
				} else {
					report.ManifestCount = count
				}
				found = true
				break
			}
		}
		if !found {
			report.Issues = append(report.Issues, "no manifests directory found (specify with --manifests)")
		}
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "assay env: %v\n", err)
			return ExitError
		}
	case "text":
		fmt.Fprintf(stdout, "Version:    %s\n", report.Version)
		fmt.Fprintf(stdout, "Checkers:   %d (%s)\n", report.CheckerCount, strings.Join(report.Checkers, ", "))
		if report.PolicyPath != "" {
			valid := "invalid"
			if report.PolicyValid {
				valid = "valid"
			}
			fmt.Fprintf(stdout, "Policy:     %s (%s)\n", report.PolicyPath, valid)
		} else {
			fmt.Fprintf(stdout, "Policy:     (none)\n")
		}
		if report.ManifestDir != "" {
			fmt.Fprintf(stdout, "Manifests:  %d in %s\n", report.ManifestCount, report.ManifestDir)
		} else {
			fmt.Fprintf(stdout, "Manifests:  (none)\n")
		}
		if len(report.Issues) > 0 {
			fmt.Fprintln(stdout, "\nIssues:")
			for _, issue := range report.Issues {
				fmt.Fprintf(stdout, "  - %s\n", issue)
			}
		} else {
			fmt.Fprintln(stdout, "\nNo issues found.")
		}
	default:
		fmt.Fprintf(stderr, "assay env: unknown format %q\n", *format)
		return ExitError
	}

	if len(report.Issues) > 0 {
		return ExitBlocked
	}
	return ExitPass
}

func countManifests(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if filepath.Ext(e.Name()) == ".json" {
			count++
		}
	}
	return count, nil
}
