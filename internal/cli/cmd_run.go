package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/audit"
	"github.com/joeyvictorino/assay/internal/chain"
	"github.com/joeyvictorino/assay/internal/config"
	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/overlap"
	"github.com/joeyvictorino/assay/internal/policy"
	"github.com/joeyvictorino/assay/internal/reconcile"
	"github.com/joeyvictorino/assay/internal/redact"
	"github.com/joeyvictorino/assay/internal/remediate"
	"github.com/joeyvictorino/assay/internal/report"
	"github.com/joeyvictorino/assay/internal/router"
	"github.com/joeyvictorino/assay/internal/scope"
	"github.com/joeyvictorino/assay/internal/tools"
	"github.com/joeyvictorino/assay/internal/toolsig"
	"github.com/joeyvictorino/assay/internal/validate"
)

// Lead-owned. `assay run` is the end-to-end pipeline: every configured
// model assesses every healthy lab independently, findings are checked by
// deterministic validators, chained, given remediations, reconciled against
// the audit log, and the per-model overlap is computed. Everything lands in
// results/<run-id>/ with no transcript on disk.
func init() { Register("run", "run the validation pipeline and write results/", runRun) }

// Test seams: the manifests to verify and run with, and the scripted
// provider a "fake" model uses. Production uses the defaults.
var (
	manifestsFn    = tools.Manifests
	fakeProviderFn = fakeScript
)

type runOpts struct {
	config, mode, out, scopePath, trust, policyPath, auditKeyEnv string
	ephemeralKey, allowMissing                                   bool
	labNames, labURLs                                            []string
	labDir, ciURL, runID                                         string
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func runRun(args []string, stdout, stderr io.Writer) int {
	var o runOpts
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.config, "config", "runs/ci.yaml", "run configuration")
	fs.StringVar(&o.mode, "mode", "full", "full or degraded (degraded: no target validation unless a lab URL is given)")
	fs.StringVar(&o.out, "out", "results", "results root")
	fs.StringVar(&o.scopePath, "scope", "scope/ci.yaml", "signed scope document")
	fs.StringVar(&o.trust, "trust", "trust/keys", "trust store directory")
	fs.StringVar(&o.policyPath, "policy", "policy/default.yaml", "policy document")
	fs.StringVar(&o.auditKeyEnv, "audit-key-env", "ASSAY_AUDIT_KEY", "env var holding the base64 audit master key")
	fs.BoolVar(&o.ephemeralKey, "ephemeral-audit-key", false, "generate an in-memory audit key (log stays chain-verifiable, not decryptable later)")
	fs.BoolVar(&o.allowMissing, "allow-missing-providers", false, "drop providers whose API key env var is empty and label the run degraded")
	labsFlag := stringList{}
	urlsFlag := stringList{}
	fs.Var(&labsFlag, "lab", "lab to assess (repeatable; default: synthetic-ops,juice-shop,dvwa)")
	fs.Var(&urlsFlag, "lab-url", "name=url override (repeatable)")
	fs.StringVar(&o.labDir, "lab-dir", "labs", "checkout path of labs/")
	fs.StringVar(&o.ciURL, "ci-url", "", "CI run URL recorded in results (default from GITHUB_* env)")
	fs.StringVar(&o.runID, "run-id", "", "run id (default: timestamp plus random suffix)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	o.labNames = labsFlag
	if len(o.labNames) == 0 {
		o.labNames = []string{"synthetic-ops", "juice-shop", "dvwa"}
	}
	o.labURLs = urlsFlag
	if o.runID == "" {
		var b [4]byte
		_, _ = rand.Read(b[:])
		o.runID = time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
	}
	code, err := runPipeline(context.Background(), o, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "assay run: %v\n", err)
	}
	return code
}

type perModel struct {
	ref      model.ModelRef
	findings []model.Finding
	usage    model.Usage
	refusals int
	failover int
	lat      []int64
	spent    float64
}

func runPipeline(ctx context.Context, o runOpts, stdout, stderr io.Writer) (int, error) {
	started := time.Now().UTC()
	cfg, err := config.Load(o.config)
	if err != nil {
		return ExitError, err
	}
	mode := o.mode
	if o.allowMissing {
		dropped := dropMissingProviders(cfg)
		if len(dropped) > 0 {
			mode = "degraded"
			fmt.Fprintf(stderr, "run: providers without credentials dropped: %s\n", strings.Join(dropped, ", "))
		}
	}
	if err := rejectPlaceholderModels(cfg); err != nil {
		return ExitError, err
	}
	// The assessed set is the config's models list (one independent agent
	// per model per lab); routes only matter for failover inside a route.
	refs := make([]model.ModelRef, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		refs = append(refs, model.ModelRef{Provider: m.Provider, Model: m.Model})
	}
	if len(refs) == 0 {
		return ExitError, errors.New("no models left to run")
	}

	// Trust, manifests, policy: all fail closed.
	ts, err := toolsig.LoadTrustStore(o.trust)
	if err != nil {
		return ExitError, err
	}
	manifests, err := manifestsFn()
	if err != nil {
		return ExitError, err
	}
	for _, m := range manifests {
		if _, reason, err := ts.Verify(m); err != nil {
			return ExitError, fmt.Errorf("manifest %s: %s: %w", m.Name, reason, err)
		}
	}
	eng, err := policy.Load(o.policyPath)
	if err != nil {
		return ExitError, err
	}

	// Audit log.
	var master []byte
	if o.ephemeralKey {
		master = audit.GenerateMaster()
	} else if master, err = audit.MasterFromEnv(o.auditKeyEnv); err != nil {
		return ExitError, err
	}
	if err := os.MkdirAll(".audit", 0o700); err != nil {
		return ExitError, err
	}
	auditPath := filepath.Join(".audit", o.runID+".log")
	aw, err := audit.OpenWriter(auditPath, o.runID, master)
	if err != nil {
		return ExitError, err
	}

	// Scope gate.
	gate, err := scope.Load(o.scopePath, started, ts.VerifyDocument)
	if err != nil {
		_ = aw.Close()
		return ExitError, err
	}
	labURLs, err := parseLabURLs(o.labURLs)
	if err != nil {
		_ = aw.Close()
		return ExitError, err
	}
	newClient := func(agentName string) *httpx.Client {
		return &httpx.Client{Gate: gate, Auditor: aw, RunID: o.runID, Agent: agentName, Timeout: 30 * time.Second}
	}
	probeTools := make([]model.ToolManifest, 0, len(manifests))
	for _, m := range manifests {
		if allowsAgent(m, "probe") {
			probeTools = append(probeTools, m)
		}
	}
	var scoreTools []model.ToolManifest
	for _, m := range manifests {
		if allowsAgent(m, "score") && len(m.AllowedAgents) > 0 {
			scoreTools = append(scoreTools, m)
		}
	}
	var reflections []model.ReflectionResult

	byModel := map[string]*perModel{}
	for _, r := range refs {
		byModel[r.Model] = &perModel{ref: r}
	}
	var all []model.Finding
	var labsRun []string
	chains := []model.AttackPath{}
	truncated := false
	precision := map[string]model.PRF{}
	for _, name := range o.labNames {
		lab, err := newLab(name, labURLs[name], o.labDir, newClient("lab-setup"))
		if err != nil {
			_ = aw.Close()
			return ExitError, err
		}
		if err := labHealthy(ctx, lab); err != nil {
			fmt.Fprintf(stderr, "run: lab %s skipped: %v\n", name, err)
			continue
		}
		labsRun = append(labsRun, name)
		labFindings := map[string][]model.Finding{}
		for _, ref := range refs {
			ref := ref
			ref.Agent = "probe"
			overrides := map[string]model.Provider{}
			if cfg.Providers[ref.Provider].Type == "fake" {
				overrides[ref.Provider] = fakeProviderFn(ref.Provider, lab.BaseURL())
			}
			pm := byModel[ref.Model]
			// A router only sees its own spend, so give each one what is left
			// of the run and model budgets; a task that cannot start is
			// recorded as an incomplete run (BLOCKED), never skipped silently.
			runLeft, modelLeft := budgetLeft(cfg, byModel, pm)
			if runLeft <= 0 || modelLeft <= 0 {
				fmt.Fprintf(stderr, "run: %s on %s not started: %s budget used up\n", ref.Model, name, exhaustedBudget(runLeft))
				truncated = true
				labFindings[ref.Model] = nil
				continue
			}
			reg, err := cfg.BuildRegistry(overrides, os.Getenv)
			if err != nil {
				_ = aw.Close()
				return ExitError, err
			}
			rc := cfg.RouterConfig(o.runID)
			rc.Routes = map[string][]model.ModelRef{"probe": {ref}, "score": {ref}}
			rc.MaxFailovers = -1 // one model per route: attribution over availability
			rc.Budgets.PerRun.MaxUSD = runLeft
			rc.Budgets.PerModel.MaxUSD = modelLeft
			rt := router.New(rc, reg, aw)
			client := newClient("probe")
			t0 := time.Now()
			res := runProbeTask(ctx, probeInputs{
				cfg: cfg, rt: rt, lab: lab, labName: name, ref: ref, client: client, gate: gate,
				ts: ts, eng: eng, aw: aw, probeTools: probeTools, scoreTools: scoreTools,
				runID: o.runID, stderr: stderr,
			})
			pm.lat = append(pm.lat, time.Since(t0).Milliseconds())
			pm.spent += rt.Spent()
			if res.Err != nil {
				fmt.Fprintf(stderr, "run: %s on %s: %v\n", ref.Model, name, res.Err)
			}
			if res.Truncated {
				truncated = true
			}
			pm.refusals += res.Refusals
			pm.failover += res.Failovers
			pm.usage = addUsage(pm.usage, res.Usage)
			reflections = append(reflections, res.Reflection)
			labFindings[ref.Model] = res.Findings
		}
		// Shared deterministic validation over the union, then project the
		// state back onto every model's copy by key.
		union := unionByKey(labFindings)
		venv := validate.Env{HTTP: newClient("validate"), Lab: lab}
		if m, ok := lab.(interface{ Marker() string }); ok {
			venv.Marker = m.Marker()
		}
		// The lab answered its health check and is inside the signed scope,
		// so deterministic validation always runs; "degraded" only describes
		// which model providers were available.
		union = validate.Validate(ctx, union, venv)
		lookup := gtLookup(lab)
		for i := range union {
			r := remediate.Generate(union[i], lookup(union[i]))
			union[i].Remediation = &r
		}
		state := map[string]model.Finding{}
		for _, f := range union {
			state[f.DedupKey] = f
		}
		for m, fs := range labFindings {
			for i := range fs {
				if s, ok := state[fs[i].DedupKey]; ok {
					fs[i].State, fs[i].Validation, fs[i].Remediation = s.State, s.Validation, s.Remediation
				}
				fs[i].Summary = redact.Sanitize(fs[i].Summary)
			}
			byModel[m].findings = append(byModel[m].findings, fs...)
		}
		all = append(all, union...)
		chains = append(chains, chain.Build(union)...)
		if gt, exhaustive := groundTruthFor(lab); exhaustive {
			for m, fs := range labFindings {
				precision[m+"@"+name] = overlap.PRF(fs, gt)
			}
		}
	}
	if err := aw.Close(); err != nil {
		return ExitError, err
	}

	// Reconcile claimed tool calls against the control plane.
	var exp bytes.Buffer
	asum, err := audit.Export(auditPath, master, &exp)
	if err != nil {
		return ExitError, fmt.Errorf("audit export: %w", err)
	}
	records, err := parseRecords(exp.Bytes())
	if err != nil {
		return ExitError, err
	}
	rec := reconcile.Reconcile(records)
	all = reconcile.Downgrade(all, rec)
	for _, pm := range byModel {
		pm.findings = reconcile.Downgrade(pm.findings, rec)
	}

	// Overlap and the run report.
	perModelFindings := map[string][]model.Finding{}
	var results []model.ModelResult
	var spent float64
	for _, r := range refs {
		pm := byModel[r.Model]
		perModelFindings[r.Model] = pm.findings
		spent += pm.spent
		counts := map[model.FindingState]int{}
		for _, f := range pm.findings {
			counts[f.State]++
		}
		pm.usage.CostUSD = pm.spent
		results = append(results, model.ModelResult{Ref: pm.ref, Findings: counts, Usage: pm.usage,
			P50MS: pct(pm.lat, 50), P95MS: pct(pm.lat, 95), Refusals: pm.refusals, Failovers: pm.failover})
	}
	om := overlap.Compute(perModelFindings, nil)
	oreport := overlap.Report{Overlap: om, Precision: precision}
	head, count := asum.HeadHash, asum.Records
	verdict, code := "PASS", ExitPass
	if truncated {
		verdict, code = "BLOCKED", ExitBlocked
	}
	rr := model.RunReport{
		RunID: o.runID, GitSHA: os.Getenv("GITHUB_SHA"), CIRunURL: ciURL(o.ciURL), Mode: mode,
		Config: cfg.Name, Label: cfg.Label,
		Started: started, Finished: time.Now().UTC(),
		Scope:  model.Decision{Effect: model.EffectAllow, Reason: "SCOPE_VERIFIED", Rationale: "scope " + gate.Fingerprint(), PolicyHash: gate.Fingerprint()},
		Models: results, Labs: labsRun, Overlap: &om, Precision: precision, Chains: chains,
		Reflection: reflections, Reconcile: rec.ReasonCounts, AuditHead: head, AuditCount: count,
		BudgetCapUSD: cfg.Budgets.PerRun.MaxUSD, SpentUSD: spent, Truncated: truncated,
		Verdict: verdict, ExitCode: code,
	}
	dir := filepath.Join(o.out, o.runID)
	if err := writeResults(dir, rr, all, oreport, rec, chains, exp.Bytes(), byModel, refs); err != nil {
		return ExitError, err
	}
	if err := os.WriteFile(filepath.Join(o.out, "LATEST"), []byte(o.runID+"\n"), 0o644); err != nil {
		return ExitError, err
	}
	fmt.Fprintf(stdout, "run %s: %s; labs=%v models=%d findings=%d spent=$%.4f audit=%d records head=%s\n",
		o.runID, verdict, labsRun, len(refs), len(all), spent, count, short(head))
	fmt.Fprintf(stdout, "results: %s\n", dir)
	return code, nil
}

const systemPrompt = `You are assessing an intentionally vulnerable training application that the operator is authorized to test. Use only the tools provided; every request is checked against a signed scope. Report each distinct weakness once through report_finding, citing the tool call ids that evidence it. Do not speculate without evidence. Stop when you have covered the obvious surface.`

func objective(lab labs.Lab) string {
	return fmt.Sprintf("Assess the application at %s (lab %q). Enumerate its visible endpoints with the HTTP tools, then report findings with report_finding.", lab.BaseURL(), lab.Name())
}

func allowsAgent(m model.ToolManifest, agent string) bool {
	if len(m.AllowedAgents) == 0 {
		return true
	}
	for _, a := range m.AllowedAgents {
		if a == agent || a == "*" {
			return true
		}
	}
	return false
}

// budgetLeft returns what remains of the run-wide and the model-wide USD
// caps. Each (lab, model) assessment builds its own router, which counts only
// its own spend, so the caps are enforced here across routers: the router
// checks before every model call, and one call may overshoot what is left.
func budgetLeft(cfg *config.Config, byModel map[string]*perModel, pm *perModel) (run, modelLeft float64) {
	var spent float64
	for _, m := range byModel {
		spent += m.spent
	}
	return cfg.Budgets.PerRun.MaxUSD - spent, cfg.Budgets.PerModel.MaxUSD - pm.spent
}

func exhaustedBudget(runLeft float64) string {
	if runLeft <= 0 {
		return "per-run"
	}
	return "per-model"
}

// placeholderPrefix marks a model id in a run config that a person must
// replace before the model can be called (runs/frontier.yaml).
const placeholderPrefix = "REPLACE_WITH_"

// rejectPlaceholderModels refuses to start while a model that will actually
// be called still carries a placeholder id. Models of dropped providers are
// already gone from cfg.Models, so an unfilled placeholder is harmless until
// its provider's key is set.
func rejectPlaceholderModels(cfg *config.Config) error {
	for _, m := range cfg.Models {
		if strings.HasPrefix(m.Model, placeholderPrefix) {
			return fmt.Errorf("model id %q for provider %q is a placeholder: edit the run config (see docs/frontier-run.md) or unset that provider's API key", m.Model, m.Provider)
		}
	}
	return nil
}

// dropMissingProviders removes providers whose key env is empty, along
// with their models and route entries, and returns the dropped names.
func dropMissingProviders(cfg *config.Config) []string {
	var dropped []string
	for name, pc := range cfg.Providers {
		if pc.APIKeyEnv != "" && os.Getenv(pc.APIKeyEnv) == "" {
			dropped = append(dropped, name)
			delete(cfg.Providers, name)
		}
	}
	if len(dropped) == 0 {
		return nil
	}
	isDropped := func(p string) bool {
		for _, d := range dropped {
			if d == p {
				return true
			}
		}
		return false
	}
	kept := cfg.Models[:0]
	for _, m := range cfg.Models {
		if !isDropped(m.Provider) {
			kept = append(kept, m)
		}
	}
	cfg.Models = kept
	for kind, routes := range cfg.Routes {
		k := routes[:0]
		for _, r := range routes {
			if !isDropped(r.Provider) {
				k = append(k, r)
			}
		}
		cfg.Routes[kind] = k
	}
	sort.Strings(dropped)
	return dropped
}

func unionByKey(per map[string][]model.Finding) []model.Finding {
	seen := map[string]int{}
	var out []model.Finding
	names := make([]string, 0, len(per))
	for n := range per {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		for _, f := range per[n] {
			if i, ok := seen[f.DedupKey]; ok {
				out[i].Evidence = append(out[i].Evidence, f.Evidence...)
				out[i].ToolCallIDs = append(out[i].ToolCallIDs, f.ToolCallIDs...)
				continue
			}
			seen[f.DedupKey] = len(out)
			out = append(out, f)
		}
	}
	return out
}

func addUsage(a, b model.Usage) model.Usage {
	a.InputTokens += b.InputTokens
	a.OutputTokens += b.OutputTokens
	a.CacheReadTokens += b.CacheReadTokens
	a.CacheWriteTokens += b.CacheWriteTokens
	a.CostUSD += b.CostUSD
	a.LatencyMS += b.LatencyMS
	return a
}

func pct(v []int64, p int) int64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]int64(nil), v...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	i := (len(s)-1)*p/100 + 0
	return s[i]
}

func parseRecords(b []byte) ([]model.AuditRecord, error) {
	var out []model.AuditRecord
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r model.AuditRecord
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("audit export: %w", err)
		}
		out = append(out, r)
	}
	return out, sc.Err()
}

func ciURL(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if os.Getenv("GITHUB_RUN_ID") == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/actions/runs/%s/attempts/%s", os.Getenv("GITHUB_SERVER_URL"), os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID"), os.Getenv("GITHUB_RUN_ATTEMPT"))
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func writeResults(dir string, rr model.RunReport, all []model.Finding, or overlap.Report, rec reconcile.Report, chains []model.AttackPath, auditExport []byte, byModel map[string]*perModel, refs []model.ModelRef) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	put := func(name string, v any) error {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0o644)
	}
	if err := put("run.json", rr); err != nil {
		return err
	}
	if err := put("findings.redacted.json", all); err != nil {
		return err
	}
	ob, err := overlap.EncodeReport(or)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "overlap.json"), ob, 0o644); err != nil {
		return err
	}
	for _, r := range refs {
		pm := byModel[r.Model]
		if err := put("findings."+safeName(r.Model)+".json", pm.findings); err != nil {
			return err
		}
	}
	costs := map[string]any{}
	for _, r := range refs {
		pm := byModel[r.Model]
		costs[r.Model] = map[string]any{"usd": pm.spent, "input_tokens": pm.usage.InputTokens, "output_tokens": pm.usage.OutputTokens,
			"cache_read_tokens": pm.usage.CacheReadTokens, "p50_ms": pct(pm.lat, 50), "p95_ms": pct(pm.lat, 95)}
	}
	if err := put("costs.json", costs); err != nil {
		return err
	}
	rb, err := reconcile.Encode(rec)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "reconcile.json"), rb, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "chains.mmd"), []byte(chain.Mermaid(chains)), 0o644); err != nil {
		return err
	}
	// The export is bodies-free by construction (digests and bounded
	// metadata only) and is what the reconciliation was computed from.
	if err := os.WriteFile(filepath.Join(dir, "audit.export.jsonl"), auditExport, 0o644); err != nil {
		return err
	}
	ci := map[string]string{"run_url": rr.CIRunURL, "run_id": os.Getenv("GITHUB_RUN_ID"), "attempt": os.Getenv("GITHUB_RUN_ATTEMPT"), "sha": rr.GitSHA, "workflow": os.Getenv("GITHUB_WORKFLOW")}
	if err := put("ci.json", ci); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "report.md"), []byte(report.Markdown(rr, all, &rec)), 0o644)
}

func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, s)
}
