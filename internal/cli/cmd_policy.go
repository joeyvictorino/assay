package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/policy"
	"github.com/joeyvictorino/assay/internal/toolsig"
)

func init() {
	Register("policy", "evaluate the policy for one tool request: eval | lint | test | coverage", runPolicy)
}

func runPolicy(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		policyUsage(stderr)
		return ExitError
	}
	switch args[0] {
	case "eval":
		return policyEval(args[1:], stdout, stderr)
	case "lint":
		return policyLint(args[1:], stdout, stderr)
	case "test":
		return policyTest(args[1:], stdout, stderr)
	case "coverage":
		return policyCoverage(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		policyUsage(stdout)
		return ExitPass
	}
	fmt.Fprintf(stderr, "assay policy: unknown action %q\n", args[0])
	policyUsage(stderr)
	return ExitError
}

func policyUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  assay policy eval --policy FILE --tool NAME --manifests DIR --agent A")
	fmt.Fprintln(w, "                    [--trust DIR | --assume-signed] [--lab L] [--depth N]")
	fmt.Fprintln(w, "                    [--ancestors a,b] [--spent USD]")
	fmt.Fprintln(w, "  prints the Decision as JSON; exit 0 allow, 1 deny, 2 error")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy lint --policy FILE --manifests DIR")
	fmt.Fprintln(w, "  checks for unreachable rules, unknown tool/agent refs; exit 0 clean, 1 issues, 2 error")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy test --policy FILE --manifests DIR --scenarios FILE")
	fmt.Fprintln(w, "  batch-tests scenarios against the policy; exit 0 all pass, 1 failures, 2 error")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy coverage --policy FILE --manifests DIR")
	fmt.Fprintln(w, "  reports rule coverage over the agent×tool matrix; exit 0 full, 1 gaps, 2 error")
}

func policyEval(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy eval", stderr)
	policyPath := fs.String("policy", "", "policy YAML file")
	toolName := fs.String("tool", "", "tool name (manifest \"name\" field)")
	manifests := fs.String("manifests", "", "directory of *.json manifests")
	agent := fs.String("agent", "", "requesting agent id")
	lab := fs.String("lab", "", "lab id")
	depth := fs.Int("depth", 0, "delegation depth")
	ancestors := fs.String("ancestors", "", "comma-separated agent ids up the delegation chain")
	spent := fs.Float64("spent", 0, "USD already spent by the agent in this run")
	trust := fs.String("trust", "", "trust store directory; the manifest signature is verified against it")
	assume := fs.Bool("assume-signed", false, "skip signature verification and treat the manifest as signed (policy testing only)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" || *toolName == "" || *manifests == "" || *agent == "" {
		fmt.Fprintln(stderr, "assay policy eval: --policy, --tool, --manifests and --agent are required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy eval: %v\n", err)
		return ExitError
	}
	files, err := loadManifestDir(*manifests)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy eval: %v\n", err)
		return ExitError
	}
	var found *model.ToolManifest
	for i := range files {
		if files[i].M.Name == *toolName {
			found = &files[i].M
			break
		}
	}
	if found == nil {
		fmt.Fprintf(stderr, "assay policy eval: no manifest named %q in %s\n", *toolName, *manifests)
		return ExitError
	}

	req := model.PolicyRequest{
		Agent:    *agent,
		Tool:     *found,
		Lab:      *lab,
		Depth:    *depth,
		SpentUSD: *spent,
	}
	if *ancestors != "" {
		for _, a := range strings.Split(*ancestors, ",") {
			if a = strings.TrimSpace(a); a != "" {
				req.Ancestors = append(req.Ancestors, a)
			}
		}
	}
	switch {
	case *trust != "":
		ts, err := toolsig.LoadTrustStore(*trust)
		if err != nil {
			fmt.Fprintf(stderr, "assay policy eval: %v\n", err)
			return ExitError
		}
		_, reason, verr := ts.Verify(*found)
		req.SignatureOK = verr == nil
		req.SignatureReason = reason
	case *assume:
		req.SignatureOK = true
		req.SignatureReason = "ASSUMED_SIGNED"
	default:
		req.SignatureOK = false
		req.SignatureReason = policy.ReasonSignatureUnverified
	}

	d := eng.Evaluate(context.Background(), req)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		fmt.Fprintf(stderr, "assay policy eval: %v\n", err)
		return ExitError
	}
	if d.Effect == model.EffectAllow {
		return ExitPass
	}
	return ExitBlocked
}

func policyLint(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy lint", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	manifests := fs.String("manifests", "", "directory of *.json tool manifests (required)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" || *manifests == "" {
		fmt.Fprintln(stderr, "assay policy lint: --policy and --manifests are required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy lint: %v\n", err)
		return ExitError
	}
	files, err := loadManifestDir(*manifests)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy lint: %v\n", err)
		return ExitError
	}
	var ms []model.ToolManifest
	for _, f := range files {
		ms = append(ms, f.M)
	}
	issues := policy.Lint(eng, ms)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(issues); err != nil {
		fmt.Fprintf(stderr, "assay policy lint: %v\n", err)
		return ExitError
	}
	if len(issues) > 0 {
		fmt.Fprintf(stderr, "assay policy lint: %d issue(s) found\n", len(issues))
		return ExitBlocked
	}
	fmt.Fprintln(stderr, "assay policy lint: clean")
	return ExitPass
}

type testScenario struct {
	Agent  string `json:"agent"`
	Tool   string `json:"tool"`
	Expect string `json:"expect"` // "allow" or "deny"
	Label  string `json:"label"`
}

type testResult struct {
	Label  string `json:"label"`
	Agent  string `json:"agent"`
	Tool   string `json:"tool"`
	Expect string `json:"expect"`
	Got    string `json:"got"`
	Reason string `json:"reason"`
	Pass   bool   `json:"pass"`
}

func policyTest(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy test", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	manifests := fs.String("manifests", "", "directory of *.json tool manifests (required)")
	scenarios := fs.String("scenarios", "", "JSON file with test scenarios (required)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" || *manifests == "" || *scenarios == "" {
		fmt.Fprintln(stderr, "assay policy test: --policy, --manifests and --scenarios are required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy test: %v\n", err)
		return ExitError
	}
	files, err := loadManifestDir(*manifests)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy test: %v\n", err)
		return ExitError
	}
	byName := map[string]model.ToolManifest{}
	for _, f := range files {
		byName[f.M.Name] = f.M
	}

	raw, err := os.ReadFile(*scenarios)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy test: %v\n", err)
		return ExitError
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var sc []testScenario
	if err := dec.Decode(&sc); err != nil {
		fmt.Fprintf(stderr, "assay policy test: %s: %v\n", *scenarios, err)
		return ExitError
	}
	if len(sc) == 0 {
		fmt.Fprintln(stderr, "assay policy test: scenarios file is empty")
		return ExitError
	}

	var results []testResult
	failures := 0
	for i, s := range sc {
		if s.Expect != "allow" && s.Expect != "deny" {
			fmt.Fprintf(stderr, "assay policy test: scenario %d: expect must be \"allow\" or \"deny\", got %q\n", i, s.Expect)
			return ExitError
		}
		m, ok := byName[s.Tool]
		if !ok {
			fmt.Fprintf(stderr, "assay policy test: scenario %d: no manifest named %q\n", i, s.Tool)
			return ExitError
		}
		req := model.PolicyRequest{
			Agent:           s.Agent,
			Tool:            m,
			SignatureOK:     true,
			SignatureReason: "POLICY_TEST",
		}
		d := eng.Evaluate(context.Background(), req)
		pass := string(d.Effect) == s.Expect
		label := s.Label
		if label == "" {
			label = fmt.Sprintf("scenario[%d]", i)
		}
		if !pass {
			failures++
		}
		results = append(results, testResult{
			Label:  label,
			Agent:  s.Agent,
			Tool:   s.Tool,
			Expect: s.Expect,
			Got:    string(d.Effect),
			Reason: d.Reason,
			Pass:   pass,
		})
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		fmt.Fprintf(stderr, "assay policy test: %v\n", err)
		return ExitError
	}
	if failures > 0 {
		fmt.Fprintf(stderr, "assay policy test: %d/%d scenario(s) failed\n", failures, len(sc))
		return ExitBlocked
	}
	fmt.Fprintf(stderr, "assay policy test: %d/%d scenario(s) passed\n", len(sc), len(sc))
	return ExitPass
}

func policyCoverage(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy coverage", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	manifests := fs.String("manifests", "", "directory of *.json tool manifests (required)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" || *manifests == "" {
		fmt.Fprintln(stderr, "assay policy coverage: --policy and --manifests are required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy coverage: %v\n", err)
		return ExitError
	}
	files, err := loadManifestDir(*manifests)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy coverage: %v\n", err)
		return ExitError
	}
	var ms []model.ToolManifest
	for _, f := range files {
		ms = append(ms, f.M)
	}
	report := policy.Coverage(eng, ms)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(stderr, "assay policy coverage: %v\n", err)
		return ExitError
	}
	if len(report.Uncovered) > 0 {
		fmt.Fprintf(stderr, "assay policy coverage: %.1f%% (%d/%d pairs covered, %d uncovered)\n",
			report.Percentage, report.CoveredPairs, report.TotalPairs, len(report.Uncovered))
		return ExitBlocked
	}
	fmt.Fprintf(stderr, "assay policy coverage: 100%% (%d/%d pairs covered)\n",
		report.CoveredPairs, report.TotalPairs)
	return ExitPass
}
