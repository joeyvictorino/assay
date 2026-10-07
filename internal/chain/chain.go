// Package chain derives attack paths from findings with a deterministic
// rules table. Facts are abstract capabilities ("session", "data-access");
// a rule fires when every precondition (a finding class or a fact) is
// available. Paths run from findings to objectives; their confidence is the
// weakest state on the path (validated > theorized).
package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// Rule says: when every Pre (class name or fact) holds, Post holds.
type Rule struct {
	Pre  []string
	Post string
	Note string
}

// Facts.
const (
	FactSession            = "session"
	FactAdminSession       = "admin-session"
	FactDataAccess         = "data-access"
	FactAccountTakeover    = "account-takeover"
	FactPrivilegeEscalate  = "privilege-escalation"
	FactCodeExec           = "code-exec"
	FactConfigRead         = "config-read"
	FactCredentialExposure = "credential-exposure"
)

// Objectives are the facts a path may terminate in.
var Objectives = []string{FactAccountTakeover, FactDataAccess, FactPrivilegeEscalate, FactCodeExec}

// DefaultRules is the deterministic rules table. Order matters only for
// tie-breaking between derivations of equal confidence.
var DefaultRules = []Rule{
	{Pre: []string{"default-creds"}, Post: FactSession, Note: "default credential logs in"},
	{Pre: []string{"default-creds"}, Post: FactAdminSession, Note: "default credential is an administrator"},
	{Pre: []string{"auth-bypass"}, Post: FactSession, Note: "authentication bypass yields a session"},
	{Pre: []string{"jwt-weak"}, Post: FactSession, Note: "forged token accepted"},
	{Pre: []string{"jwt-weak"}, Post: FactAdminSession, Note: "forged token with elevated claims accepted"},
	{Pre: []string{"sqli", "auth-bypass"}, Post: FactSession, Note: "injection in the login path bypasses authentication"},
	{Pre: []string{"sqli"}, Post: FactDataAccess, Note: "injection reads database rows"},
	{Pre: []string{"sqli"}, Post: FactCredentialExposure, Note: "injection reads stored credentials"},
	{Pre: []string{"xss-stored"}, Post: FactSession, Note: "stored script steals a victim session"},
	{Pre: []string{"xss-reflected"}, Post: FactSession, Note: "reflected script steals a victim session"},
	{Pre: []string{"info-disclosure"}, Post: FactConfigRead, Note: "configuration disclosed"},
	{Pre: []string{"sensitive-file"}, Post: FactConfigRead, Note: "sensitive file read"},
	{Pre: []string{"path-traversal"}, Post: FactConfigRead, Note: "traversal reads configuration files"},
	{Pre: []string{"ssrf"}, Post: FactConfigRead, Note: "server-side request reaches internal metadata or config"},
	{Pre: []string{FactConfigRead}, Post: FactCredentialExposure, Note: "configuration contains credentials"},
	{Pre: []string{FactCredentialExposure}, Post: FactSession, Note: "exposed credential logs in"},
	{Pre: []string{FactCredentialExposure}, Post: FactAccountTakeover, Note: "exposed credential takes over the account"},
	{Pre: []string{FactSession, "idor"}, Post: FactDataAccess, Note: "authenticated caller reads other users' records"},
	{Pre: []string{"auth-missing"}, Post: FactDataAccess, Note: "unauthenticated endpoint returns protected data"},
	{Pre: []string{FactSession, "csrf"}, Post: FactAccountTakeover, Note: "forged request changes the victim's credential"},
	{Pre: []string{FactSession, "mass-assignment"}, Post: FactPrivilegeEscalate, Note: "writable role attribute"},
	{Pre: []string{FactAdminSession}, Post: FactPrivilegeEscalate, Note: "administrator session obtained"},
	{Pre: []string{FactAdminSession}, Post: FactAccountTakeover, Note: "administrator session controls every account"},
	{Pre: []string{FactAdminSession, "path-traversal"}, Post: FactCodeExec, Note: "administrator file access plus traversal reaches executable paths"},
	{Pre: []string{FactAdminSession, "ssrf"}, Post: FactCodeExec, Note: "administrator-triggered internal request reaches management endpoints"},
}

// derivation records how a fact was reached.
type derivation struct {
	conf model.FindingState
	rule *Rule          // nil for a finding-backed class fact
	from []string       // pre names
	f    *model.Finding // set when the fact is a finding class
}

// rank orders confidence: validated beats theorized; anything else is 0.
func rank(s model.FindingState) int {
	switch s {
	case model.StateValidated:
		return 2
	case model.StateTheorized:
		return 1
	}
	return 0
}

func minState(a, b model.FindingState) model.FindingState {
	if rank(a) <= rank(b) {
		return a
	}
	return b
}

// Build derives one attack path per (lab, objective) that is reachable from
// validated or theorized findings. Output is sorted by lab then objective.
func Build(findings []model.Finding) []model.AttackPath {
	return BuildWithRules(findings, DefaultRules)
}

// BuildWithRules is Build with a caller-supplied rules table.
func BuildWithRules(findings []model.Finding, rules []Rule) []model.AttackPath {
	byLab := map[string][]model.Finding{}
	for _, f := range findings {
		if rank(f.State) == 0 {
			continue // refuted and declined findings never chain
		}
		byLab[f.Lab] = append(byLab[f.Lab], f)
	}
	labs := make([]string, 0, len(byLab))
	for l := range byLab {
		labs = append(labs, l)
	}
	sort.Strings(labs)

	var out []model.AttackPath
	for _, lab := range labs {
		fs := byLab[lab]
		sort.Slice(fs, func(i, j int) bool { return fs[i].ID < fs[j].ID })
		facts := derive(fs, rules)
		for _, obj := range Objectives {
			d, ok := facts[obj]
			if !ok {
				continue
			}
			out = append(out, buildPath(lab, obj, d, facts))
		}
	}
	return out
}

// derive runs the rules to a fixpoint, keeping the best derivation per fact.
func derive(fs []model.Finding, rules []Rule) map[string]*derivation {
	facts := map[string]*derivation{}
	// Seed class facts from findings: best state wins, then lowest ID.
	for i := range fs {
		f := &fs[i]
		cls := string(f.Class)
		if cur, ok := facts[cls]; !ok || rank(f.State) > rank(cur.conf) || (rank(f.State) == rank(cur.conf) && f.ID < cur.f.ID) {
			facts[cls] = &derivation{conf: f.State, f: f}
		}
	}
	for changed := true; changed; {
		changed = false
		for i := range rules {
			r := &rules[i]
			conf := model.StateValidated
			ok := true
			for _, p := range r.Pre {
				d, has := facts[p]
				if !has {
					ok = false
					break
				}
				conf = minState(conf, d.conf)
			}
			if !ok {
				continue
			}
			// Guard against a rule deriving one of its own preconditions.
			if containsString(r.Pre, r.Post) {
				continue
			}
			if cur, has := facts[r.Post]; !has || rank(conf) > rank(cur.conf) {
				facts[r.Post] = &derivation{conf: conf, rule: r, from: r.Pre}
				changed = true
			}
		}
	}
	return facts
}

// buildPath flattens the derivation tree of an objective into a linear path
// (findings first in dependency order, then facts, then the objective).
func buildPath(lab, objective string, d *derivation, facts map[string]*derivation) model.AttackPath {
	var steps []model.PathStep
	seen := map[string]bool{}
	var findingIDs []string
	var refs []string
	conf := model.StateValidated

	var walk func(name string, d *derivation)
	walk = func(name string, d *derivation) {
		if seen[name] {
			return
		}
		seen[name] = true
		if d.f != nil {
			conf = minState(conf, d.f.State)
			steps = append(steps, model.PathStep{Type: "finding", ID: d.f.ID, Note: string(d.f.Class) + " " + d.f.Location.Method + " " + d.f.Location.PathTemplate})
			findingIDs = append(findingIDs, d.f.ID)
			refs = append(refs, "finding:"+d.f.ID)
			for _, cp := range d.f.ControlPlane {
				refs = append(refs, fmt.Sprintf("audit:%s:%d", cp.RunID, cp.Seq))
			}
			if d.f.Validation != nil {
				for _, ev := range d.f.Validation.Evidence {
					if ev.AuditSeq != 0 {
						refs = append(refs, fmt.Sprintf("audit:%s:%d", d.f.RunID, ev.AuditSeq))
					}
				}
			}
			return
		}
		for _, p := range d.from {
			walk(p, facts[p])
		}
		typ := "fact"
		if name == objective {
			typ = "objective"
		}
		steps = append(steps, model.PathStep{Type: typ, ID: name, Note: d.rule.Note})
	}
	walk(objective, d)

	sort.Strings(findingIDs)
	refs = dedupeSorted(refs)
	sum := sha256.Sum256([]byte(lab + "|" + objective + "|" + strings.Join(findingIDs, ",")))
	return model.AttackPath{
		ID:         "path-" + hex.EncodeToString(sum[:6]),
		Lab:        lab,
		Objective:  objective,
		Path:       steps,
		Confidence: conf,
		SourceRefs: refs,
	}
}

func dedupeSorted(xs []string) []string {
	sort.Strings(xs)
	out := xs[:0]
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

var nodeIDRe = regexp.MustCompile(`[^A-Za-z0-9_]`)

func nodeID(prefix, s string) string {
	return prefix + nodeIDRe.ReplaceAllString(s, "_")
}

// Mermaid renders paths as a left-to-right flowchart. Findings are
// rectangles keyed by finding id, facts are stadium nodes and objectives
// are double circles. Output is deterministic.
func Mermaid(paths []model.AttackPath) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	nodes := map[string]string{}
	edges := map[string]bool{}
	var edgeList []string
	sorted := append([]model.AttackPath(nil), paths...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Lab != sorted[j].Lab {
			return sorted[i].Lab < sorted[j].Lab
		}
		return sorted[i].Objective < sorted[j].Objective
	})
	for _, p := range sorted {
		var prevFindings []string
		var lastFact string
		for _, s := range p.Path {
			var id, decl string
			switch s.Type {
			case "finding":
				id = nodeID("F_", s.ID)
				decl = fmt.Sprintf(`%s["%s"]`, id, escape(s.ID+": "+s.Note))
				prevFindings = append(prevFindings, id)
			case "objective":
				id = nodeID("O_", s.ID)
				decl = fmt.Sprintf(`%s(((%s)))`, id, escape(s.ID))
			default:
				id = nodeID("X_", s.ID)
				decl = fmt.Sprintf(`%s([%s])`, id, escape(s.ID))
			}
			nodes[id] = decl
			if s.Type != "finding" {
				for _, from := range prevFindings {
					addEdge(edges, &edgeList, from, id, "")
				}
				if lastFact != "" {
					addEdge(edges, &edgeList, lastFact, id, "")
				}
				prevFindings = nil
				lastFact = id
			}
		}
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		b.WriteString("    " + nodes[id] + "\n")
	}
	sort.Strings(edgeList)
	for _, e := range edgeList {
		b.WriteString("    " + e + "\n")
	}
	return b.String()
}

func addEdge(seen map[string]bool, list *[]string, from, to, label string) {
	e := from + " --> " + to
	if label != "" {
		e = from + " -->|" + escape(label) + "| " + to
	}
	if !seen[e] {
		seen[e] = true
		*list = append(*list, e)
	}
}

func escape(s string) string {
	s = strings.ReplaceAll(s, `"`, "'")
	s = strings.ReplaceAll(s, "[", "(")
	s = strings.ReplaceAll(s, "]", ")")
	s = strings.ReplaceAll(s, "{", "(")
	s = strings.ReplaceAll(s, "}", ")")
	return s
}
