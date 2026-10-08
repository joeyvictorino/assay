// Package evals checks a committed run against regression bands.
//
// A case is a small YAML document naming the models (by glob) and lab it
// covers and the bands the run must stay inside: at least this many
// validated findings, at most this share theorized, at most this much
// spend, latency and reconciliation mismatch. Bands are set from what the
// last good run achieved with slack, not from an absolute quality bar, so
// a verdict says "worse than before", never "good enough". A case marked
// advisory documents a target not yet met and is reported but never fails
// the verdict. evals/thresholds.yaml holds the defaults a case inherits for
// every band it leaves unset.
package evals

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/joeyvictorino/assay/internal/model"
)

// ErrInvalidCase wraps every case or thresholds parse failure.
var ErrInvalidCase = errors.New("evals: invalid case")

// Case is one eval case. Pointer fields are bands; nil means "inherit the
// default or, when there is none, do not check".
type Case struct {
	Name                   string   `yaml:"name" json:"name"`
	Lab                    string   `yaml:"lab,omitempty" json:"lab,omitempty"`
	ModelGlob              string   `yaml:"model_glob,omitempty" json:"model_glob,omitempty"`
	MinValidated           *int     `yaml:"min_validated,omitempty" json:"min_validated,omitempty"`
	MaxTheorizedRatio      *float64 `yaml:"max_theorized_ratio,omitempty" json:"max_theorized_ratio,omitempty"`
	MaxCostUSD             *float64 `yaml:"max_cost_usd,omitempty" json:"max_cost_usd,omitempty"`
	MaxP95MS               *int64   `yaml:"max_p95_ms,omitempty" json:"max_p95_ms,omitempty"`
	MaxReconcileMismatches *int     `yaml:"max_reconcile_mismatches,omitempty" json:"max_reconcile_mismatches,omitempty"`
	RequireZeroRefusals    bool     `yaml:"require_zero_refusals,omitempty" json:"require_zero_refusals,omitempty"`
	Advisory               bool     `yaml:"advisory,omitempty" json:"advisory,omitempty"`
	// Note is free text explaining the band or the target.
	Note string `yaml:"note,omitempty" json:"note,omitempty"`
	// Source is the file the case was loaded from; set by Load.
	Source string `yaml:"-" json:"source,omitempty"`
}

// Thresholds is the evals/thresholds.yaml document.
type Thresholds struct {
	// Defaults are the bands a case inherits where it sets none. Name,
	// Lab, ModelGlob, RequireZeroRefusals and Advisory are ignored here.
	Defaults Case `yaml:"defaults" json:"defaults"`
}

// Check is one band comparison inside a case.
type Check struct {
	Name     string `json:"name"`
	Observed string `json:"observed"`
	Limit    string `json:"limit"`
	Passed   bool   `json:"passed"`
}

// CaseResult is the outcome of one case against one run.
type CaseResult struct {
	Name     string   `json:"name"`
	Advisory bool     `json:"advisory"`
	Passed   bool     `json:"passed"`
	Models   []string `json:"models"`
	Checks   []Check  `json:"checks"`
	Reason   string   `json:"reason,omitempty"` // why no check could run
}

// Verdict groups case results. Advisory holds advisory cases that missed
// their band; they never make OK false.
type Verdict struct {
	RunID    string       `json:"run_id"`
	Passed   []CaseResult `json:"passed"`
	Failed   []CaseResult `json:"failed"`
	Advisory []CaseResult `json:"advisory"`
}

// OK reports whether no non-advisory case failed.
func (v Verdict) OK() bool { return len(v.Failed) == 0 }

// Load reads every *.yaml and *.yml file in dir, one case per file, in
// file-name order. Names must be unique and non-empty.
func Load(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("evals: read %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ext := filepath.Ext(e.Name()); ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: no case files in %s", ErrInvalidCase, dir)
	}
	seen := map[string]string{}
	cases := make([]Case, 0, len(files))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("evals: read %s: %w", f, err)
		}
		c, err := ParseCase(raw)
		if err != nil {
			return nil, fmt.Errorf("evals: %s: %w", f, err)
		}
		if prev, dup := seen[c.Name]; dup {
			return nil, fmt.Errorf("%w: %s: duplicate case name %q (also in %s)", ErrInvalidCase, f, c.Name, prev)
		}
		seen[c.Name] = f
		c.Source = f
		cases = append(cases, c)
	}
	return cases, nil
}

// ParseCase parses one case document. Unknown fields are rejected.
func ParseCase(raw []byte) (Case, error) {
	var c Case
	if err := strictYAML(raw, &c); err != nil {
		return Case{}, err
	}
	if err := c.validate(); err != nil {
		return Case{}, err
	}
	return c, nil
}

func (c Case) validate() error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidCase, fmt.Sprintf(format, args...))
	}
	if strings.TrimSpace(c.Name) == "" {
		return bad("name is required")
	}
	if c.ModelGlob != "" {
		if _, err := path.Match(c.ModelGlob, ""); err != nil {
			return bad("case %q: model_glob %q: %v", c.Name, c.ModelGlob, err)
		}
	}
	if c.MinValidated != nil && *c.MinValidated < 0 {
		return bad("case %q: min_validated must be >= 0", c.Name)
	}
	if c.MaxTheorizedRatio != nil && (*c.MaxTheorizedRatio < 0 || *c.MaxTheorizedRatio > 1) {
		return bad("case %q: max_theorized_ratio must be within [0,1]", c.Name)
	}
	if c.MaxCostUSD != nil && *c.MaxCostUSD < 0 {
		return bad("case %q: max_cost_usd must be >= 0", c.Name)
	}
	if c.MaxP95MS != nil && *c.MaxP95MS < 0 {
		return bad("case %q: max_p95_ms must be >= 0", c.Name)
	}
	if c.MaxReconcileMismatches != nil && *c.MaxReconcileMismatches < 0 {
		return bad("case %q: max_reconcile_mismatches must be >= 0", c.Name)
	}
	return nil
}

// LoadThresholds reads the defaults document.
func LoadThresholds(file string) (Thresholds, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return Thresholds{}, fmt.Errorf("evals: read %s: %w", file, err)
	}
	t, err := ParseThresholds(raw)
	if err != nil {
		return Thresholds{}, fmt.Errorf("evals: %s: %w", file, err)
	}
	return t, nil
}

// ParseThresholds parses the defaults document. Unknown fields are rejected.
func ParseThresholds(raw []byte) (Thresholds, error) {
	var t Thresholds
	if err := strictYAML(raw, &t); err != nil {
		return Thresholds{}, err
	}
	d := t.Defaults
	d.Name = "defaults"
	if err := d.validate(); err != nil {
		return Thresholds{}, err
	}
	return t, nil
}

func strictYAML(raw []byte, v any) error {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fmt.Errorf("%w: empty document", ErrInvalidCase)
	}
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: yaml: %v", ErrInvalidCase, err)
	}
	return nil
}

// WithDefaults returns copies of cases with every unset band filled from
// t.Defaults.
func WithDefaults(cases []Case, t Thresholds) []Case {
	out := make([]Case, len(cases))
	for i, c := range cases {
		d := t.Defaults
		if c.MinValidated == nil {
			c.MinValidated = d.MinValidated
		}
		if c.MaxTheorizedRatio == nil {
			c.MaxTheorizedRatio = d.MaxTheorizedRatio
		}
		if c.MaxCostUSD == nil {
			c.MaxCostUSD = d.MaxCostUSD
		}
		if c.MaxP95MS == nil {
			c.MaxP95MS = d.MaxP95MS
		}
		if c.MaxReconcileMismatches == nil {
			c.MaxReconcileMismatches = d.MaxReconcileMismatches
		}
		out[i] = c
	}
	return out
}

// Evaluate runs every case against the run report and its findings.
// findings may be nil; then validated and theorized counts come from the
// per-model counts in the report, reconciliation mismatches from the
// report's reason counts, and a case with a lab filter cannot run.
func Evaluate(cases []Case, rr model.RunReport, findings []model.Finding) Verdict {
	v := Verdict{RunID: rr.RunID, Passed: []CaseResult{}, Failed: []CaseResult{}, Advisory: []CaseResult{}}
	for _, c := range cases {
		res := evaluateCase(c, rr, findings)
		switch {
		case res.Passed:
			v.Passed = append(v.Passed, res)
		case c.Advisory:
			v.Advisory = append(v.Advisory, res)
		default:
			v.Failed = append(v.Failed, res)
		}
	}
	return v
}

type tally struct {
	validated, theorized, mismatches, refusals int
	costUSD                                    float64
	p95MS                                      int64
}

func evaluateCase(c Case, rr model.RunReport, findings []model.Finding) CaseResult {
	res := CaseResult{Name: c.Name, Advisory: c.Advisory, Models: []string{}, Checks: []Check{}}
	glob := c.ModelGlob
	if glob == "" {
		glob = "*"
	}
	matched := map[string]bool{}
	var tl tally
	for _, m := range rr.Models {
		if ok, _ := path.Match(glob, m.Ref.Model); !ok {
			continue
		}
		matched[m.Ref.Model] = true
		res.Models = append(res.Models, m.Ref.Model)
		tl.refusals += m.Refusals
		tl.costUSD += m.Usage.CostUSD
		if m.P95MS > tl.p95MS {
			tl.p95MS = m.P95MS
		}
		if findings == nil {
			tl.validated += m.Findings[model.StateValidated]
			tl.theorized += m.Findings[model.StateTheorized]
		}
	}
	sort.Strings(res.Models)
	if len(res.Models) == 0 {
		res.Reason = fmt.Sprintf("no model in the run matches %q", glob)
		return res
	}
	if findings == nil {
		if c.Lab != "" {
			res.Reason = "lab filter needs the run's findings file"
			return res
		}
		for _, n := range rr.Reconcile {
			tl.mismatches += n
		}
	} else {
		for _, f := range findings {
			if !matched[f.Model.Model] || (c.Lab != "" && f.Lab != c.Lab) {
				continue
			}
			switch f.State {
			case model.StateValidated:
				tl.validated++
			case model.StateTheorized:
				tl.theorized++
			}
			if f.Reconcile.Checked && !f.Reconcile.Match {
				tl.mismatches++
			}
		}
	}

	add := func(name, observed, limit string, ok bool) {
		res.Checks = append(res.Checks, Check{Name: name, Observed: observed, Limit: limit, Passed: ok})
	}
	if c.MinValidated != nil {
		add("min_validated", fmt.Sprint(tl.validated), ">= "+fmt.Sprint(*c.MinValidated), tl.validated >= *c.MinValidated)
	}
	if c.MaxTheorizedRatio != nil {
		ratio := 0.0
		if total := tl.validated + tl.theorized; total > 0 {
			ratio = float64(tl.theorized) / float64(total)
		}
		add("max_theorized_ratio", fmt.Sprintf("%.3f (%d/%d)", ratio, tl.theorized, tl.validated+tl.theorized), fmt.Sprintf("<= %.3f", *c.MaxTheorizedRatio), ratio <= *c.MaxTheorizedRatio)
	}
	if c.MaxCostUSD != nil {
		add("max_cost_usd", fmt.Sprintf("%.6f", tl.costUSD), fmt.Sprintf("<= %.6f", *c.MaxCostUSD), tl.costUSD <= *c.MaxCostUSD)
	}
	if c.MaxP95MS != nil {
		add("max_p95_ms", fmt.Sprint(tl.p95MS), "<= "+fmt.Sprint(*c.MaxP95MS), tl.p95MS <= *c.MaxP95MS)
	}
	if c.MaxReconcileMismatches != nil {
		add("max_reconcile_mismatches", fmt.Sprint(tl.mismatches), "<= "+fmt.Sprint(*c.MaxReconcileMismatches), tl.mismatches <= *c.MaxReconcileMismatches)
	}
	if c.RequireZeroRefusals {
		add("require_zero_refusals", fmt.Sprint(tl.refusals), "== 0", tl.refusals == 0)
	}
	if len(res.Checks) == 0 {
		res.Reason = "case has no bands to check"
		return res
	}
	res.Passed = true
	for _, ch := range res.Checks {
		if !ch.Passed {
			res.Passed = false
		}
	}
	return res
}

// Markdown renders a verdict as an issue or log body.
func Markdown(v Verdict) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Eval verdict for run %s\n\n", v.RunID)
	status := "PASS"
	if !v.OK() {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "%s: %d passed, %d failed, %d advisory misses\n\n", status, len(v.Passed), len(v.Failed), len(v.Advisory))
	section := func(title string, rs []CaseResult) {
		if len(rs) == 0 {
			return
		}
		fmt.Fprintf(&b, "### %s\n\n", title)
		for _, r := range rs {
			fmt.Fprintf(&b, "- **%s** (models: %s)\n", r.Name, strings.Join(r.Models, ", "))
			if r.Reason != "" {
				fmt.Fprintf(&b, "  - %s\n", r.Reason)
			}
			for _, ch := range r.Checks {
				mark := "ok"
				if !ch.Passed {
					mark = "MISS"
				}
				fmt.Fprintf(&b, "  - %s: %s %s observed %s\n", ch.Name, mark, ch.Limit, ch.Observed)
			}
		}
		b.WriteString("\n")
	}
	section("Failed", v.Failed)
	section("Advisory (target not yet met)", v.Advisory)
	section("Passed", v.Passed)
	return b.String()
}
