package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/policy"
)

type scoreResult struct {
	Score     int    `json:"score"`
	Max       int    `json:"max"`
	Grade     string `json:"grade"`
	Manifests int    `json:"manifests"`
	Rules     int    `json:"rules"`
	Agents    int    `json:"agents"`
	Issues    int    `json:"issues"`
}

func gradeFromScore(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func policyScore(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy score", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	manifests := fs.String("manifests", "", "directory of *.json tool manifests (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" || *manifests == "" {
		fmt.Fprintln(stderr, "assay policy score: --policy and --manifests are required")
		return ExitError
	}

	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy score: %v\n", err)
		return ExitError
	}

	files, err := loadManifestDir(*manifests)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy score: %v\n", err)
		return ExitError
	}
	var ms []model.ToolManifest
	for _, f := range files {
		ms = append(ms, f.M)
	}

	issues := policy.Lint(eng, ms)
	stats := policy.Stats(eng.Policy())

	score := 100 - len(issues)*5
	if score < 0 {
		score = 0
	}

	res := scoreResult{
		Score:     score,
		Max:       100,
		Grade:     gradeFromScore(score),
		Manifests: len(ms),
		Rules:     stats.RuleCount,
		Agents:    stats.AgentCount,
		Issues:    len(issues),
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(stderr, "assay policy score: %v\n", err)
			return ExitError
		}
	case "text":
		fmt.Fprintf(stdout, "Posture Score: %d/%d (Grade: %s)\n", res.Score, res.Max, res.Grade)
		fmt.Fprintf(stdout, "Manifests:     %d\n", res.Manifests)
		fmt.Fprintf(stdout, "Rules:         %d\n", res.Rules)
		fmt.Fprintf(stdout, "Agents:        %d\n", res.Agents)
		fmt.Fprintf(stdout, "Issues:        %d\n", res.Issues)
	default:
		fmt.Fprintf(stderr, "assay policy score: unknown format %q\n", *format)
		return ExitError
	}

	if res.Score < 70 {
		return ExitBlocked
	}
	return ExitPass
}
