package model

import (
	"testing"
	"time"
)

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		sev  Severity
		want int
	}{
		{SevInfo, 0},
		{SevLow, 1},
		{SevMedium, 2},
		{SevHigh, 3},
		{SevCritical, 4},
		{Severity("unknown"), 0},
		{Severity(""), 0},
	}
	for _, tt := range tests {
		if got := tt.sev.Rank(); got != tt.want {
			t.Errorf("Severity(%q).Rank() = %d, want %d", tt.sev, got, tt.want)
		}
	}
}

func TestSeverityRankOrdering(t *testing.T) {
	ordered := []Severity{SevInfo, SevLow, SevMedium, SevHigh, SevCritical}
	for i := 1; i < len(ordered); i++ {
		if ordered[i].Rank() <= ordered[i-1].Rank() {
			t.Errorf("%q.Rank() (%d) should be > %q.Rank() (%d)",
				ordered[i], ordered[i].Rank(), ordered[i-1], ordered[i-1].Rank())
		}
	}
}

func TestRiskTierRank(t *testing.T) {
	tests := []struct {
		tier RiskTier
		want int
	}{
		{TierLow, 1},
		{TierMedium, 2},
		{TierHigh, 3},
		{TierCritical, 4},
		{RiskTier("unknown"), 0},
		{RiskTier(""), 0},
	}
	for _, tt := range tests {
		if got := tt.tier.Rank(); got != tt.want {
			t.Errorf("RiskTier(%q).Rank() = %d, want %d", tt.tier, got, tt.want)
		}
	}
}

func TestRiskTierRankOrdering(t *testing.T) {
	ordered := []RiskTier{TierLow, TierMedium, TierHigh, TierCritical}
	for i := 1; i < len(ordered); i++ {
		if ordered[i].Rank() <= ordered[i-1].Rank() {
			t.Errorf("%q.Rank() (%d) should be > %q.Rank() (%d)",
				ordered[i], ordered[i].Rank(), ordered[i-1], ordered[i-1].Rank())
		}
	}
}

func TestAllClassesCount(t *testing.T) {
	if got := len(AllClasses); got != 20 {
		t.Errorf("len(AllClasses) = %d, want 20", got)
	}
}

func TestAllClassesNoDuplicates(t *testing.T) {
	seen := make(map[Class]bool)
	for _, c := range AllClasses {
		if seen[c] {
			t.Errorf("duplicate class %q in AllClasses", c)
		}
		seen[c] = true
	}
}

func TestAllClassesNonEmpty(t *testing.T) {
	for i, c := range AllClasses {
		if c == "" {
			t.Errorf("AllClasses[%d] is empty", i)
		}
	}
}

func TestFindingStateConstants(t *testing.T) {
	states := []FindingState{StateTheorized, StateValidated, StateRefuted, StateDeclined}
	seen := make(map[FindingState]bool)
	for _, s := range states {
		if s == "" {
			t.Error("empty FindingState constant")
		}
		if seen[s] {
			t.Errorf("duplicate FindingState %q", s)
		}
		seen[s] = true
	}
}

func TestEffectConstants(t *testing.T) {
	if EffectAllow == EffectDeny {
		t.Error("EffectAllow and EffectDeny must differ")
	}
	if EffectAllow == "" || EffectDeny == "" {
		t.Error("Effect constants must be non-empty")
	}
}

func TestFindingZeroValue(t *testing.T) {
	var f Finding
	if f.ID != "" {
		t.Error("zero Finding.ID should be empty")
	}
	if f.Severity.Rank() != 0 {
		t.Errorf("zero Finding.Severity.Rank() = %d, want 0", f.Severity.Rank())
	}
	if f.Class != "" {
		t.Error("zero Finding.Class should be empty")
	}
}

func TestFindingFields(t *testing.T) {
	now := time.Now()
	f := Finding{
		ID:       "f-001",
		DedupKey: "dk-001",
		RunID:    "run-1",
		Lab:      "lab-a",
		Class:    ClassSQLi,
		CWE:      "CWE-89",
		Location: Location{
			Method:       "GET",
			PathTemplate: "/api/users/:id",
			Param:        "id",
			Component:    "query",
		},
		Severity:    SevHigh,
		State:       StateValidated,
		Summary:     "SQL injection in user lookup",
		Model:       ModelRef{Provider: "anthropic", Model: "claude-3", Agent: "recon"},
		PromptHash:  "abc123",
		ToolCallIDs: []string{"tc-1", "tc-2"},
		Temperature: 0.7,
		MaxTokens:   4096,
		FirstSeen:   now,
		LastSeen:    now,
	}

	if f.Class != ClassSQLi {
		t.Errorf("Finding.Class = %q, want %q", f.Class, ClassSQLi)
	}
	if f.Severity != SevHigh {
		t.Errorf("Finding.Severity = %q, want %q", f.Severity, SevHigh)
	}
	if f.Severity.Rank() != 3 {
		t.Errorf("Finding.Severity.Rank() = %d, want 3", f.Severity.Rank())
	}
	if f.Location.Method != "GET" {
		t.Errorf("Finding.Location.Method = %q, want GET", f.Location.Method)
	}
	if f.Model.Provider != "anthropic" {
		t.Errorf("Finding.Model.Provider = %q, want anthropic", f.Model.Provider)
	}
	if len(f.ToolCallIDs) != 2 {
		t.Errorf("len(Finding.ToolCallIDs) = %d, want 2", len(f.ToolCallIDs))
	}
}

func TestToolManifestFields(t *testing.T) {
	m := ToolManifest{
		Name:        "http_request",
		Version:     "1.0",
		RiskTier:    TierHigh,
		SideEffects: true,
		TimeoutMS:   30000,
	}
	if m.RiskTier.Rank() != 3 {
		t.Errorf("ToolManifest.RiskTier.Rank() = %d, want 3", m.RiskTier.Rank())
	}
}

func TestBudgetZeroValue(t *testing.T) {
	var b Budget
	if b.MaxUSD != 0 || b.MaxLatencyMS != 0 || b.MaxTokens != 0 {
		t.Error("zero Budget should have all zeros")
	}
}

func TestUsageFields(t *testing.T) {
	u := Usage{
		InputTokens:      1000,
		OutputTokens:     500,
		CacheReadTokens:  200,
		CacheWriteTokens: 100,
		CostUSD:          0.025,
		LatencyMS:        1500,
	}
	if u.InputTokens+u.OutputTokens != 1500 {
		t.Error("token sum mismatch")
	}
}

func TestPRFFields(t *testing.T) {
	p := PRF{TP: 8, FP: 2, FN: 1, Precision: 0.8, Recall: 0.889, F1: 0.842}
	if p.TP+p.FP != 10 {
		t.Errorf("TP+FP = %d, want 10", p.TP+p.FP)
	}
}

func TestReconcileStatusZero(t *testing.T) {
	var r ReconcileStatus
	if r.Checked || r.Match {
		t.Error("zero ReconcileStatus should be unchecked and unmatched")
	}
}

func TestValidationResultPointer(t *testing.T) {
	f := Finding{ID: "f-1"}
	if f.Validation != nil {
		t.Error("zero Finding should have nil Validation")
	}
	f.Validation = &ValidationResult{
		State:   StateValidated,
		Checker: "sqli-checker",
	}
	if f.Validation.State != StateValidated {
		t.Errorf("Validation.State = %q, want %q", f.Validation.State, StateValidated)
	}
}

func TestRemediationPointer(t *testing.T) {
	f := Finding{ID: "f-1"}
	if f.Remediation != nil {
		t.Error("zero Finding should have nil Remediation")
	}
	f.Remediation = &Remediation{
		CodeDiff: "--- a/foo\n+++ b/foo",
		Verified: true,
	}
	if !f.Remediation.Verified {
		t.Error("Remediation.Verified should be true")
	}
}

func TestRunReportFields(t *testing.T) {
	rr := RunReport{
		RunID:    "run-1",
		Mode:     "full",
		Verdict:  "PASS",
		ExitCode: 0,
	}
	if rr.Verdict != "PASS" {
		t.Errorf("RunReport.Verdict = %q, want PASS", rr.Verdict)
	}
}

func TestReviewerKindConstants(t *testing.T) {
	if ReviewerSelf == ReviewerPeer {
		t.Error("ReviewerSelf and ReviewerPeer must differ")
	}
}

func TestDecisionFields(t *testing.T) {
	d := Decision{
		Effect:    EffectDeny,
		Reason:    "DENY_RULE:no-write-in-recon",
		Rationale: "recon agents cannot write",
	}
	if d.Effect != EffectDeny {
		t.Errorf("Decision.Effect = %q, want %q", d.Effect, EffectDeny)
	}
}

func TestPolicyDefaults(t *testing.T) {
	p := Policy{
		Version:       "1",
		DefaultEffect: EffectDeny,
	}
	if p.DefaultEffect != EffectDeny {
		t.Errorf("Policy.DefaultEffect = %q, want %q", p.DefaultEffect, EffectDeny)
	}
}

func TestLocationZeroValue(t *testing.T) {
	var l Location
	if l.Method != "" || l.PathTemplate != "" {
		t.Error("zero Location should have empty fields")
	}
}

func TestModelRefFields(t *testing.T) {
	ref := ModelRef{Provider: "openai", Model: "gpt-4", Agent: "probe"}
	if ref.Provider != "openai" || ref.Model != "gpt-4" || ref.Agent != "probe" {
		t.Errorf("ModelRef fields mismatch: %+v", ref)
	}
}

func TestAuditRecordFields(t *testing.T) {
	ar := AuditRecord{
		Seq:   1,
		RunID: "run-1",
		Kind:  "tool_call",
		Agent: "recon",
	}
	if ar.Kind != "tool_call" {
		t.Errorf("AuditRecord.Kind = %q, want tool_call", ar.Kind)
	}
}

func TestTaskFields(t *testing.T) {
	task := Task{
		ID:    "t-1",
		Wave:  0,
		Kind:  "recon",
		Agent: "recon-1",
		Lab:   "lab-a",
	}
	if task.Kind != "recon" {
		t.Errorf("Task.Kind = %q, want recon", task.Kind)
	}
}
