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
	Register("policy", "evaluate the policy for one tool request: eval | lint | test | coverage | simulate | diff | merge | validate | export | template | stats | history | import | graph | search | benchmark", runPolicy)
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
	case "validate":
		return policyValidate(args[1:], stdout, stderr)
	case "export":
		return policyExport(args[1:], stdout, stderr)
	case "template":
		return policyTemplate(args[1:], stdout, stderr)
	case "stats":
		return policyStats(args[1:], stdout, stderr)
	case "graph":
		return policyGraph(args[1:], stdout, stderr)
	case "search":
		return policySearch(args[1:], stdout, stderr)
	case "history":
		return policyHistory(args[1:], stdout, stderr)
	case "import":
		return policyImport(args[1:], stdout, stderr)
	case "benchmark":
		return policyBenchmark(args[1:], stdout, stderr)
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
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy validate --policy FILE")
	fmt.Fprintln(w, "  validates policy structure without needing manifests; exit 0 valid, 1 issues, 2 error")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy export --policy FILE [--format markdown|yaml|json]")
	fmt.Fprintln(w, "  exports the policy as a human-readable document")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy template --preset minimal|standard|strict [--out FILE]")
	fmt.Fprintln(w, "  generates a starter policy YAML from a preset")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy stats --policy FILE [--format text|json]")
	fmt.Fprintln(w, "  prints summary statistics about the policy")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy history --policies FILE[,FILE,...] [--format text|json]")
	fmt.Fprintln(w, "  shows a changelog across sequential policy versions (earliest first)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy import --input FILE --from json|csv [--out FILE] [--format yaml|json]")
	fmt.Fprintln(w, "  imports a policy from another format")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy graph --policy FILE [--format dot|mermaid]")
	fmt.Fprintln(w, "  visualizes agent-tool-rule relationships as a graph")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy search --policy FILE [--keyword K] [--effect allow|deny]")
	fmt.Fprintln(w, "  [--agent A] [--tool T] [--capability C] [--format text|json]")
	fmt.Fprintln(w, "  searches policy rules by keyword, effect, agent, tool, or capability")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  assay policy benchmark --policy FILE --manifests DIR [--iterations N] [--format text|json]")
	fmt.Fprintln(w, "  measures policy evaluation throughput")
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
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(merged); err != nil {
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

func policyValidate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy validate", stderr)
	polPath := fs.String("policy", "", "policy YAML file (required)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *polPath == "" {
		fmt.Fprintln(stderr, "assay policy validate: --policy is required")
		return ExitError
	}
	p, err := policy.ParseLenient(*polPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy validate: %v\n", err)
		return ExitError
	}
	issues := policy.Validate(p)

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(issues); err != nil {
		fmt.Fprintf(stderr, "assay policy validate: %v\n", err)
		return ExitError
	}

	if len(issues) == 0 {
		fmt.Fprintln(stderr, "assay policy validate: valid")
		return ExitPass
	}
	errors := 0
	for _, i := range issues {
		if i.Severity == "error" {
			errors++
		}
	}
	fmt.Fprintf(stderr, "assay policy validate: %d issue(s) (%d errors)\n", len(issues), errors)
	return ExitBlocked
}

func policyExport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy export", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	format := fs.String("format", "markdown", "output format: markdown (default), yaml, or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" {
		fmt.Fprintln(stderr, "assay policy export: --policy is required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy export: %v\n", err)
		return ExitError
	}
	p := eng.Policy()

	switch *format {
	case "markdown", "md":
		fmt.Fprint(stdout, policy.ExportMarkdown(p))
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(p); err != nil {
			fmt.Fprintf(stderr, "assay policy export: %v\n", err)
			return ExitError
		}
	case "yaml":
		out, err := marshalPolicyYAML(p)
		if err != nil {
			fmt.Fprintf(stderr, "assay policy export: %v\n", err)
			return ExitError
		}
		stdout.Write(out)
	default:
		fmt.Fprintf(stderr, "assay policy export: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}

func policyTemplate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy template", stderr)
	preset := fs.String("preset", "", "template preset: minimal, standard, or strict (required)")
	outPath := fs.String("out", "-", "output path (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *preset == "" {
		fmt.Fprintf(stderr, "assay policy template: --preset is required (choices: %s)\n",
			strings.Join(policy.TemplatePresets(), ", "))
		return ExitError
	}
	tmpl, err := policy.GenerateTemplate(*preset)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy template: %v\n", err)
		return ExitError
	}
	if *outPath == "-" {
		fmt.Fprint(stdout, tmpl)
	} else {
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
			fmt.Fprintf(stderr, "assay policy template: %v\n", err)
			return ExitError
		}
		if err := os.WriteFile(*outPath, []byte(tmpl), 0o644); err != nil {
			fmt.Fprintf(stderr, "assay policy template: %v\n", err)
			return ExitError
		}
		fmt.Fprintf(stderr, "assay policy template: wrote %s\n", *outPath)
	}
	return ExitPass
}

func policyStats(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy stats", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	format := fs.String("format", "text", "output format: text (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" {
		fmt.Fprintln(stderr, "assay policy stats: --policy is required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy stats: %v\n", err)
		return ExitError
	}
	s := policy.Stats(eng.Policy())

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(s); err != nil {
			fmt.Fprintf(stderr, "assay policy stats: %v\n", err)
			return ExitError
		}
	case "text":
		allowN := s.RulesByEffect["allow"]
		denyN := s.RulesByEffect["deny"]
		fmt.Fprintf(stdout, "Rules:        %d (%d allow, %d deny)\n", s.RuleCount, allowN, denyN)
		fmt.Fprintf(stdout, "Agents:       %d (%s)\n", s.AgentCount, strings.Join(s.AgentNames, ", "))
		fmt.Fprintf(stdout, "Budget:       $%.2f – $%.2f USD, %d – %d ms\n",
			s.BudgetRange.MinUSD, s.BudgetRange.MaxUSD,
			s.BudgetRange.MinLatencyMS, s.BudgetRange.MaxLatencyMS)
		fmt.Fprintf(stdout, "Dangerous:    %d combinations\n", s.DangerousComboCount)
		fmt.Fprintf(stdout, "Weights:      %d classes\n", s.WeightCount)
		if len(s.Capabilities) > 0 {
			fmt.Fprintf(stdout, "Capabilities: %s\n", strings.Join(s.Capabilities, ", "))
		}
		if len(s.Tools) > 0 {
			fmt.Fprintf(stdout, "Tools:        %s\n", strings.Join(s.Tools, ", "))
		}
	default:
		fmt.Fprintf(stderr, "assay policy stats: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}

func policyHistory(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy history", stderr)
	policiesFlag := fs.String("policies", "", "comma-separated list of policy YAML files, earliest first (required)")
	format := fs.String("format", "text", "output format: text (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policiesFlag == "" {
		fmt.Fprintln(stderr, "assay policy history: --policies is required")
		return ExitError
	}

	paths := strings.Split(*policiesFlag, ",")
	var engines []*policy.Engine
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		eng, err := policy.Load(p)
		if err != nil {
			fmt.Fprintf(stderr, "assay policy history: %v\n", err)
			return ExitError
		}
		engines = append(engines, eng)
	}

	if len(engines) == 0 {
		fmt.Fprintln(stderr, "assay policy history: no valid policy files provided")
		return ExitError
	}

	entries := policy.BuildHistory(engines)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			fmt.Fprintf(stderr, "assay policy history: %v\n", err)
			return ExitError
		}
	case "text":
		fmt.Fprint(stdout, policy.FormatHistoryText(entries))
	default:
		fmt.Fprintf(stderr, "assay policy history: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}

func policyImport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy import", stderr)
	inputPath := fs.String("input", "", "input file path (required)")
	fromFmt := fs.String("from", "", "input format: json or csv (required)")
	outPath := fs.String("out", "-", "output path (default stdout)")
	format := fs.String("format", "yaml", "output format: yaml (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *inputPath == "" || *fromFmt == "" {
		fmt.Fprintln(stderr, "assay policy import: --input and --from are required")
		return ExitError
	}

	data, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy import: %v\n", err)
		return ExitError
	}

	var p model.Policy
	switch *fromFmt {
	case "json":
		p, err = policy.ImportJSON(data)
	case "csv":
		p, err = policy.ImportCSV(data)
	default:
		fmt.Fprintf(stderr, "assay policy import: unknown source format %q (use json or csv)\n", *fromFmt)
		return ExitError
	}
	if err != nil {
		fmt.Fprintf(stderr, "assay policy import: %v\n", err)
		return ExitError
	}

	var output []byte
	switch *format {
	case "json":
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(p); err != nil {
			fmt.Fprintf(stderr, "assay policy import: %v\n", err)
			return ExitError
		}
		output = buf.Bytes()
	case "yaml":
		output, err = marshalPolicyYAML(p)
		if err != nil {
			fmt.Fprintf(stderr, "assay policy import: %v\n", err)
			return ExitError
		}
	default:
		fmt.Fprintf(stderr, "assay policy import: unknown output format %q\n", *format)
		return ExitError
	}

	if *outPath == "-" {
		stdout.Write(output)
	} else {
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
			fmt.Fprintf(stderr, "assay policy import: %v\n", err)
			return ExitError
		}
		if err := os.WriteFile(*outPath, output, 0o644); err != nil {
			fmt.Fprintf(stderr, "assay policy import: %v\n", err)
			return ExitError
		}
		fmt.Fprintf(stderr, "assay policy import: wrote %s\n", *outPath)
	}
	return ExitPass
}

func policyGraph(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy graph", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	format := fs.String("format", "dot", "output format: dot (default) or mermaid")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" {
		fmt.Fprintln(stderr, "assay policy graph: --policy is required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy graph: %v\n", err)
		return ExitError
	}
	p := eng.Policy()

	switch *format {
	case "dot":
		fmt.Fprint(stdout, policy.GraphDOT(p))
	case "mermaid":
		fmt.Fprint(stdout, policy.GraphMermaid(p))
	default:
		fmt.Fprintf(stderr, "assay policy graph: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}

func policySearch(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy search", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	keyword := fs.String("keyword", "", "search keyword (matches rule ID, rationale, agents, tools, capabilities)")
	effect := fs.String("effect", "", "filter by effect: allow or deny")
	agent := fs.String("agent", "", "filter by agent name")
	tool := fs.String("tool", "", "filter by tool name")
	capability := fs.String("capability", "", "filter by capability")
	format := fs.String("format", "text", "output format: text (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" {
		fmt.Fprintln(stderr, "assay policy search: --policy is required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy search: %v\n", err)
		return ExitError
	}
	q := policy.SearchQuery{
		Keyword:    *keyword,
		Effect:     *effect,
		Agent:      *agent,
		Tool:       *tool,
		Capability: *capability,
	}
	results := policy.Search(eng.Policy(), q)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			fmt.Fprintf(stderr, "assay policy search: %v\n", err)
			return ExitError
		}
	case "text":
		fmt.Fprint(stdout, policy.FormatSearchText(results))
	default:
		fmt.Fprintf(stderr, "assay policy search: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}

func policyBenchmark(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay policy benchmark", stderr)
	policyPath := fs.String("policy", "", "policy YAML file (required)")
	manifests := fs.String("manifests", "", "directory of *.json tool manifests (required)")
	iterations := fs.Int("iterations", 100, "number of evaluation rounds")
	format := fs.String("format", "text", "output format: text (default) or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *policyPath == "" || *manifests == "" {
		fmt.Fprintln(stderr, "assay policy benchmark: --policy and --manifests are required")
		return ExitError
	}
	eng, err := policy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy benchmark: %v\n", err)
		return ExitError
	}
	files, err := loadManifestDir(*manifests)
	if err != nil {
		fmt.Fprintf(stderr, "assay policy benchmark: %v\n", err)
		return ExitError
	}
	var ms []model.ToolManifest
	for _, f := range files {
		ms = append(ms, f.M)
	}
	r := policy.Benchmark(eng, ms, *iterations)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			fmt.Fprintf(stderr, "assay policy benchmark: %v\n", err)
			return ExitError
		}
	case "text":
		fmt.Fprint(stdout, policy.FormatBenchmarkText(r))
	default:
		fmt.Fprintf(stderr, "assay policy benchmark: unknown format %q\n", *format)
		return ExitError
	}
	fmt.Fprintf(stderr, "assay policy benchmark: %d evaluations in %d ms (%.2f eval/s)\n",
		r.TotalEvaluations, r.TotalDurationMS, r.EvalPerSecond)
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
	return yaml.Marshal(m)
}
