package chain

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func f(id string, class model.Class, state model.FindingState) model.Finding {
	return model.Finding{ID: id, Lab: "synthetic-ops", Class: class, State: state,
		Location:     model.Location{Method: "GET", PathTemplate: "/" + string(class)},
		ControlPlane: []model.AuditRef{{RunID: "run-1", Seq: 7}}, RunID: "run-1",
		Validation: &model.ValidationResult{Evidence: []model.Evidence{{AuditSeq: 9}}}}
}

func pathIDs(p model.AttackPath) []string {
	var out []string
	for _, s := range p.Path {
		out = append(out, s.Type+":"+s.ID)
	}
	return out
}

func find(paths []model.AttackPath, objective string) *model.AttackPath {
	for i := range paths {
		if paths[i].Objective == objective {
			return &paths[i]
		}
	}
	return nil
}

func TestDefaultCredsSessionIDORDataAccess(t *testing.T) {
	paths := Build([]model.Finding{
		f("F-IDOR", model.ClassIDOR, model.StateValidated),
		f("F-DC", model.ClassDefaultCreds, model.StateValidated),
	})
	p := find(paths, FactDataAccess)
	if p == nil {
		t.Fatalf("no data-access path: %+v", paths)
	}
	want := []string{"finding:F-DC", "fact:session", "finding:F-IDOR", "objective:data-access"}
	if got := pathIDs(*p); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("path %v, want %v", got, want)
	}
	if p.Confidence != model.StateValidated || p.Lab != "synthetic-ops" || !strings.HasPrefix(p.ID, "path-") {
		t.Fatalf("%+v", p)
	}
	joined := strings.Join(p.SourceRefs, ",")
	for _, want := range []string{"finding:F-DC", "finding:F-IDOR", "audit:run-1:7", "audit:run-1:9"} {
		if !strings.Contains(joined, want) {
			t.Errorf("source refs %v lack %s", p.SourceRefs, want)
		}
	}
	// default-creds alone also reaches privilege-escalation and account-takeover.
	if find(paths, FactPrivilegeEscalate) == nil || find(paths, FactAccountTakeover) == nil {
		t.Fatalf("objectives: %+v", paths)
	}
}

func TestSQLiPlusAuthBypassYieldsSession(t *testing.T) {
	rules := []Rule{
		{Pre: []string{"sqli", "auth-bypass"}, Post: FactSession, Note: "combined"},
		{Pre: []string{FactSession, "idor"}, Post: FactDataAccess, Note: "read"},
	}
	fs := []model.Finding{
		f("F-SQLI", model.ClassSQLi, model.StateValidated),
		f("F-AB", model.ClassAuthBypass, model.StateTheorized),
		f("F-IDOR", model.ClassIDOR, model.StateValidated),
	}
	paths := BuildWithRules(fs, rules)
	p := find(paths, FactDataAccess)
	if p == nil {
		t.Fatal("no path")
	}
	// Findings appear in the rule's Pre order.
	want := "finding:F-SQLI finding:F-AB fact:session finding:F-IDOR objective:data-access"
	if got := strings.Join(pathIDs(*p), " "); got != want {
		t.Fatalf("got %s", got)
	}
	if p.Confidence != model.StateTheorized {
		t.Fatalf("confidence must be the weakest step: %s", p.Confidence)
	}
	// Without auth-bypass the combined rule must not fire.
	if BuildWithRules(fs[:1], rules) != nil {
		t.Fatal("sqli alone must not reach data-access under these rules")
	}
	// With the default table, sqli + auth-bypass reach session and beyond.
	if find(Build(fs[:2]), FactDataAccess) == nil {
		t.Fatal("default rules: sqli must reach data-access")
	}
}

func TestNoFindingsNoPaths(t *testing.T) {
	if got := Build(nil); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	if got := Build([]model.Finding{f("F-R", model.ClassSQLi, model.StateRefuted), f("F-D", model.ClassIDOR, model.StateDeclined)}); len(got) != 0 {
		t.Fatalf("refuted/declined must not chain: %+v", got)
	}
	// A finding that leads nowhere produces no path.
	if got := Build([]model.Finding{f("F-H", model.ClassSecurityHeaders, model.StateValidated)}); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestTheorizedOnlyPathHasTheorizedConfidence(t *testing.T) {
	paths := Build([]model.Finding{
		f("F-DC", model.ClassDefaultCreds, model.StateTheorized),
		f("F-IDOR", model.ClassIDOR, model.StateTheorized),
	})
	if len(paths) == 0 {
		t.Fatal("theorized findings must still chain")
	}
	for _, p := range paths {
		if p.Confidence != model.StateTheorized {
			t.Errorf("%s: %s", p.Objective, p.Confidence)
		}
	}
}

func TestMixedConfidenceIsMin(t *testing.T) {
	paths := Build([]model.Finding{
		f("F-DC", model.ClassDefaultCreds, model.StateValidated),
		f("F-IDOR", model.ClassIDOR, model.StateTheorized),
	})
	if p := find(paths, FactDataAccess); p == nil || p.Confidence != model.StateTheorized {
		t.Fatalf("%+v", p)
	}
	if p := find(paths, FactPrivilegeEscalate); p == nil || p.Confidence != model.StateValidated {
		t.Fatalf("%+v", p)
	}
}

func TestBestDerivationWins(t *testing.T) {
	// Two sqli findings: the validated one must back the class fact.
	paths := Build([]model.Finding{
		f("F-S2", model.ClassSQLi, model.StateTheorized),
		f("F-S1", model.ClassSQLi, model.StateValidated),
	})
	p := find(paths, FactDataAccess)
	if p == nil || p.Confidence != model.StateValidated || p.Path[0].ID != "F-S1" {
		t.Fatalf("%+v", p)
	}
}

func TestDeterministicAcrossOrderAndLabs(t *testing.T) {
	a := f("F-DC", model.ClassDefaultCreds, model.StateValidated)
	b := f("F-IDOR", model.ClassIDOR, model.StateValidated)
	c := f("F-OTHER", model.ClassAuthMissing, model.StateValidated)
	c.Lab = "dvwa"
	p1 := Build([]model.Finding{a, b, c})
	p2 := Build([]model.Finding{c, b, a})
	if len(p1) != len(p2) || len(p1) == 0 {
		t.Fatalf("%d vs %d", len(p1), len(p2))
	}
	for i := range p1 {
		if p1[i].ID != p2[i].ID || p1[i].Lab != p2[i].Lab || strings.Join(pathIDs(p1[i]), ",") != strings.Join(pathIDs(p2[i]), ",") {
			t.Fatalf("order-dependent output at %d: %+v vs %+v", i, p1[i], p2[i])
		}
	}
	if p1[0].Lab != "dvwa" {
		t.Fatalf("labs must be sorted: %s", p1[0].Lab)
	}
	if Mermaid(p1) != Mermaid(p2) {
		t.Fatal("mermaid must be deterministic")
	}
}

func TestMermaid(t *testing.T) {
	paths := Build([]model.Finding{
		f("F-DC", model.ClassDefaultCreds, model.StateValidated),
		f("F-IDOR", model.ClassIDOR, model.StateValidated),
	})
	m := Mermaid(paths)
	if !strings.HasPrefix(m, "flowchart LR\n") {
		t.Fatalf("header: %q", m)
	}
	for _, want := range []string{
		`F_F_DC["F-DC: default-creds GET /default-creds"]`,
		`F_F_IDOR["F-IDOR: idor GET /idor"]`,
		`X_session([session])`,
		`O_data_access(((data-access)))`,
		`O_privilege_escalation(((privilege-escalation)))`,
		`F_F_DC --> X_session`,
		`X_session --> O_data_access`,
		`F_F_IDOR --> O_data_access`,
	} {
		if !strings.Contains(m, want) {
			t.Errorf("mermaid lacks %q:\n%s", want, m)
		}
	}
	if strings.Count(m, "F_F_DC --> X_session") != 1 {
		t.Fatal("edges must be deduplicated")
	}
	if Mermaid(nil) != "flowchart LR\n" {
		t.Fatal("empty")
	}
}

func TestRulesTableSane(t *testing.T) {
	known := map[string]bool{}
	for _, c := range model.AllClasses {
		known[string(c)] = true
	}
	for _, fct := range []string{FactSession, FactAdminSession, FactDataAccess, FactAccountTakeover, FactPrivilegeEscalate, FactCodeExec, FactConfigRead, FactCredentialExposure} {
		known[fct] = true
	}
	for i, r := range DefaultRules {
		if len(r.Pre) == 0 || r.Post == "" || r.Note == "" {
			t.Errorf("rule %d malformed: %+v", i, r)
		}
		if !known[r.Post] {
			t.Errorf("rule %d: unknown post %q", i, r.Post)
		}
		for _, p := range r.Pre {
			if !known[p] {
				t.Errorf("rule %d: unknown pre %q", i, p)
			}
			if p == r.Post {
				t.Errorf("rule %d derives its own precondition", i)
			}
		}
	}
}
