package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/policy"
	"github.com/joeyvictorino/assay/internal/toolsig"
	"gopkg.in/yaml.v3"
)

func init() {
	Register("policy", "evaluate the policy for one tool request: eval | lint | test | coverage | simulate | diff | merge", runPolicy)
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
	case "simulate":
		return policySimulate(args[1:], stdout, stderr)
	case "diff":
		return policyDiff(args[1:], stdout, stderr)
	case "merge":
		return policyMerge(args[1:], stdout, stderr)
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
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy simulate --policy FILE --manifests DIR")
	fmt.Fprintln(w, "  [--agent A] [--tool T]  evaluate every agent×tool pair and print the decision matrix")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy diff --before FILE --after FILE [--format json|text]")
	fmt.Fprintln(w, "  compares two policy files; exit 0 identical, 1 different, 2 error")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy merge --policies FILE[,FILE,...] [--out FILE] [--format yaml|json]")
	fmt.Fprintln(w, "  merges multiple policy files; later files override earlier ones")
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

type simCell struct {
	Agent     string `json:"agent"`
	Tool      string `json:"tool"`
	Effect    string `json:"effect"`
	Reason    string `json:"reason"`
	Rationale string `json:"rationale"`
}

type simReport struct {
	Agents  []string  `json:"agents"`
	Tools   []string  `json:"tools"`
	Allowed int       `json:"allowed"`
	Denied  int       `json:"denied"`
	Cells   []simCell `json:"cells"`
}

func policySimulate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy simulate", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	manifests := fs.String("manifests", "", "directory of *.json tool manifests (required)")
	agentFilter := fs.String("agent", "", "filter to a specific agent (optional)")
	toolFilter := fs.String("tool", "", "filter to a specific tool (optional)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" || *manifests == "" {
		fmt.Fprintln(stderr, "assay policy simulate: --policy and --manifests are required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy simulate: %v\n", err)
		return ExitError
	}
	files, err := loadManifestDir(*manifests)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy simulate: %v\n", err)
		return ExitError
	}

	p := eng.Policy()

	agents := make([]string, 0, len(p.Agents))
	for name := range p.Agents {
		if *agentFilter != "" && name != *agentFilter {
			continue
		}
		agents = append(agents, name)
	}
	sort.Strings(agents)

	var tools []model.ToolManifest
	for _, f := range files {
		if *toolFilter != "" && f.M.Name != *toolFilter {
			continue
		}
		tools = append(tools, f.M)
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })

	if len(agents) == 0 {
		fmt.Fprintln(stderr, "assay policy simulate: no agents match the filter")
		return ExitError
	}
	if len(tools) == 0 {
		fmt.Fprintln(stderr, "assay policy simulate: no tools match the filter")
		return ExitError
	}

	toolNames := make([]string, len(tools))
	for i, t := range tools {
		toolNames[i] = t.Name
	}

	var cells []simCell
	allowed, denied := 0, 0
	for _, agent := range agents {
		for _, tool := range tools {
			req := model.PolicyRequest{
				Agent:           agent,
				Tool:            tool,
				SignatureOK:     true,
				SignatureReason: "SIMULATION",
			}
			d := eng.Evaluate(context.Background(), req)
			cells = append(cells, simCell{
				Agent:     agent,
				Tool:      tool.Name,
				Effect:    string(d.Effect),
				Reason:    d.Reason,
				Rationale: d.Rationale,
			})
			if d.Effect == model.EffectAllow {
				allowed++
			} else {
				denied++
			}
		}
	}

	report := simReport{
		Agents:  agents,
		Tools:   toolNames,
		Allowed: allowed,
		Denied:  denied,
		Cells:   cells,
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(stderr, "assay policy simulate: %v\n", err)
		return ExitError
	}
	total := allowed + denied
	fmt.Fprintf(stderr, "assay policy simulate: %d pairs evaluated (%d allowed, %d denied)\n",
		total, allowed, denied)
	return ExitPass
}

func policyDiff(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy diff", stderr)
	beforePath := fs.String("before", "", "earlier policy YAML file (required)")
	afterPath := fs.String("after", "", "later policy YAML file (required)")
	format := fs.String("format", "text", "output format: text (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *beforePath == "" || *afterPath == "" {
		fmt.Fprintln(stderr, "assay policy diff: --before and --after are required")
		return ExitError
	}
	before, err := policy.Load(*beforePath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy diff: %v\n", err)
		return ExitError
	}
	after, err := policy.Load(*afterPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy diff: %v\n", err)
		return ExitError
	}
	report := policy.Diff(before, after)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "assay policy diff: %v\n", err)
			return ExitError
		}
	case "text":
		fmt.Fprint(stdout, policy.FormatDiffText(report))
	default:
		fmt.Fprintf(stderr, "assay policy diff: unknown format %q\n", *format)
		return ExitError
	}

	if report.Identical {
		fmt.Fprintln(stderr, "assay policy diff: identical")
		return ExitPass
	}
	fmt.Fprintln(stderr, "assay policy diff: differences found")
	return ExitBlocked
}

func policyMerge(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy merge", stderr)
	policiesFlag := fs.String("policies", "", "comma-separated list of policy YAML files (required)")
	outPath := fs.String("out", "-", "output path (default stdout)")
	format := fs.String("format", "yaml", "output format: yaml (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policiesFlag == "" {
		fmt.Fprintln(stderr, "assay policy merge: --policies is required")
		return ExitError
	}

	paths := strings.Split(*policiesFlag, ",")
	var policies []model.Policy
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		eng, err := policy.Load(p)
		if err != nil {
			fmt.Fprintf(stderr, "assay policy merge: %v\n", err)
			return ExitError
		}
		policies = append(policies, eng.Policy())
	}

	if len(policies) == 0 {
		fmt.Fprintln(stderr, "assay policy merge: no valid policy files provided")
		return ExitError
	}

	merged := policy.Merge(policies)

	var output []byte
	var err error
	switch *format {
	case "json":
		enc := json.NewEncoder(&bytes.Buffer{})
		enc.SetIndent("", "  ")
		var buf bytes.Buffer
		enc2 := json.NewEncoder(&buf)
		enc2.SetIndent("", "  ")
		if err := enc2.Encode(merged); err != nil {
			fmt.Fprintf(stderr, "assay policy merge: %v\n", err)
			return ExitError
		}
		output = buf.Bytes()
	case "yaml":
		output, err = marshalPolicyYAML(merged)
		if err != nil {
			fmt.Fprintf(stderr, "assay policy merge: %v\n", err)
			return ExitError
		}
	default:
		fmt.Fprintf(stderr, "assay policy merge: unknown format %q\n", *format)
		return ExitError
	}

	if *outPath == "-" {
		stdout.Write(output)
	} else {
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
			fmt.Fprintf(stderr, "assay policy merge: %v\n", err)
			return ExitError
		}
		if err := os.WriteFile(*outPath, output, 0o644); err != nil {
			fmt.Fprintf(stderr, "assay policy merge: %v\n", err)
			return ExitError
		}
		fmt.Fprintf(stderr, "assay policy merge: wrote %s\n", *outPath)
	}
	return ExitPass
}

func marshalPolicyYAML(p model.Policy) ([]byte, error) {
	js, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(js, &m); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
