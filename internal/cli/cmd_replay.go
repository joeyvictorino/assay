package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/policy"
)

func init() {
	Register("replay", "re-evaluate saved decisions against a different policy", runReplay)
}

// ReplayDecision is one agent+tool decision from a previous run.
type ReplayDecision struct {
	Agent  string `json:"agent"`
	Tool   string `json:"tool"`
	Effect string `json:"effect"`
	Reason string `json:"reason,omitempty"`
}

// ReplayInput is the format read from the --run file.
type ReplayInput struct {
	PolicyHash string           `json:"policy_hash,omitempty"`
	Decisions  []ReplayDecision `json:"decisions"`
}

// ReplayResult compares one original decision with the new policy's outcome.
type ReplayResult struct {
	Agent         string `json:"agent"`
	Tool          string `json:"tool"`
	OriginalEffect string `json:"original_effect"`
	NewEffect     string `json:"new_effect"`
	NewReason     string `json:"new_reason"`
	Changed       bool   `json:"changed"`
}

// ReplayReport is the full output.
type ReplayReport struct {
	OriginalPolicyHash string         `json:"original_policy_hash,omitempty"`
	NewPolicyHash      string         `json:"new_policy_hash"`
	TotalDecisions     int            `json:"total_decisions"`
	ChangedCount       int            `json:"changed_count"`
	UnchangedCount     int            `json:"unchanged_count"`
	Results            []ReplayResult `json:"results"`
}

func runReplay(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay replay", stderr)
	runPath := fs.String("run", "", "JSON file with previous decisions (required)")
	policyPath := fs.String("policy", "", "new policy YAML file to evaluate against (required)")
	manifests := fs.String("manifests", "", "directory of *.json tool manifests (required)")
	format := fs.String("format", "text", "output format: text (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runPath == "" || *policyPath == "" || *manifests == "" {
		fmt.Fprintln(stderr, "assay replay: --run, --policy, and --manifests are required")
		return ExitError
	}

	raw, err := os.ReadFile(*runPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay replay: %v\n", err)
		return ExitError
	}
	var input ReplayInput
	if err := json.Unmarshal(raw, &input); err != nil {
		fmt.Fprintf(stderr, "assay replay: %s: %v\n", *runPath, err)
		return ExitError
	}
	if len(input.Decisions) == 0 {
		fmt.Fprintln(stderr, "assay replay: no decisions in input file")
		return ExitError
	}

	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay replay: %v\n", err)
		return ExitError
	}

	files, err := loadManifestDir(*manifests)
	if err != nil {
		fmt.Fprintf(stderr, "assay replay: %v\n", err)
		return ExitError
	}
	byName := map[string]model.ToolManifest{}
	for _, f := range files {
		byName[f.M.Name] = f.M
	}

	var results []ReplayResult
	changed, unchanged := 0, 0

	for _, d := range input.Decisions {
		tool, ok := byName[d.Tool]
		if !ok {
			tool = model.ToolManifest{
				Name:     d.Tool,
				Version:  "unknown",
				RiskTier: model.TierLow,
			}
		}
		req := model.PolicyRequest{
			Agent:           d.Agent,
			Tool:            tool,
			SignatureOK:     true,
			SignatureReason: "REPLAY",
		}
		newDec := eng.Evaluate(context.Background(), req)
		isChanged := string(newDec.Effect) != d.Effect
		if isChanged {
			changed++
		} else {
			unchanged++
		}
		results = append(results, ReplayResult{
			Agent:          d.Agent,
			Tool:           d.Tool,
			OriginalEffect: d.Effect,
			NewEffect:      string(newDec.Effect),
			NewReason:      newDec.Reason,
			Changed:        isChanged,
		})
	}

	report := ReplayReport{
		OriginalPolicyHash: input.PolicyHash,
		NewPolicyHash:      eng.Hash(),
		TotalDecisions:     len(input.Decisions),
		ChangedCount:       changed,
		UnchangedCount:     unchanged,
		Results:            results,
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "assay replay: %v\n", err)
			return ExitError
		}
	case "text":
		fmt.Fprint(stdout, formatReplayText(report))
	default:
		fmt.Fprintf(stderr, "assay replay: unknown format %q\n", *format)
		return ExitError
	}

	if changed > 0 {
		fmt.Fprintf(stderr, "assay replay: %d/%d decisions changed\n", changed, len(input.Decisions))
		return ExitBlocked
	}
	fmt.Fprintf(stderr, "assay replay: all %d decisions unchanged\n", len(input.Decisions))
	return ExitPass
}

func formatReplayText(r ReplayReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Replay: %d decisions (%d changed, %d unchanged)\n\n", r.TotalDecisions, r.ChangedCount, r.UnchangedCount)

	changedResults := make([]ReplayResult, 0)
	for _, res := range r.Results {
		if res.Changed {
			changedResults = append(changedResults, res)
		}
	}
	sort.Slice(changedResults, func(i, j int) bool {
		if changedResults[i].Agent != changedResults[j].Agent {
			return changedResults[i].Agent < changedResults[j].Agent
		}
		return changedResults[i].Tool < changedResults[j].Tool
	})

	if len(changedResults) == 0 {
		fmt.Fprintln(&b, "No decisions changed.")
		return b.String()
	}

	fmt.Fprintln(&b, "Changed decisions:")
	for _, res := range changedResults {
		fmt.Fprintf(&b, "  %s / %s: %s -> %s (%s)\n", res.Agent, res.Tool, res.OriginalEffect, res.NewEffect, res.NewReason)
	}
	return b.String()
}
