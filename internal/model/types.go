// Package model holds the shared data types every assay component exchanges.
//
// It contains structs and small enums only; no logic. Only the lead edits
// this package. Streams depend on it and never on each other's internals.
package model

import "time"

// FindingState is the evidentiary status of a finding.
type FindingState string

const (
	StateTheorized FindingState = "theorized" // reported by a model, not yet checked
	StateValidated FindingState = "validated" // deterministic checker confirmed it
	StateRefuted   FindingState = "refuted"   // deterministic checker disproved it
	StateDeclined  FindingState = "declined"  // model refused the task; no finding content
)

// Severity follows the usual five-step scale.
type Severity string

const (
	SevInfo     Severity = "info"
	SevLow      Severity = "low"
	SevMedium   Severity = "medium"
	SevHigh     Severity = "high"
	SevCritical Severity = "critical"
)

// Rank orders severities for comparisons. Unknown values rank lowest.
func (s Severity) Rank() int {
	switch s {
	case SevCritical:
		return 4
	case SevHigh:
		return 3
	case SevMedium:
		return 2
	case SevLow:
		return 1
	}
	return 0
}

// RiskTier classifies a tool by the harm it can do if misused.
type RiskTier string

const (
	TierLow      RiskTier = "low"
	TierMedium   RiskTier = "medium"
	TierHigh     RiskTier = "high"
	TierCritical RiskTier = "critical"
)

// Rank orders tiers for boundary checks.
func (t RiskTier) Rank() int {
	switch t {
	case TierCritical:
		return 4
	case TierHigh:
		return 3
	case TierMedium:
		return 2
	case TierLow:
		return 1
	}
	return 0
}

// Effect is a policy outcome. Deny always wins.
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Class is the closed vulnerability taxonomy a model may report. The
// harness normalizes everything else; models cannot invent classes.
type Class string

const (
	ClassSQLi            Class = "sqli"
	ClassXSSReflected    Class = "xss-reflected"
	ClassXSSStored       Class = "xss-stored"
	ClassIDOR            Class = "idor"
	ClassAuthMissing     Class = "auth-missing"
	ClassAuthBypass      Class = "auth-bypass"
	ClassDefaultCreds    Class = "default-creds"
	ClassOpenRedirect    Class = "open-redirect"
	ClassPathTraversal   Class = "path-traversal"
	ClassSSRF            Class = "ssrf"
	ClassInfoDisclosure  Class = "info-disclosure"
	ClassSecurityHeaders Class = "security-headers"
	ClassCORS            Class = "cors-misconfig"
	ClassMassAssignment  Class = "mass-assignment"
	ClassJWTWeak         Class = "jwt-weak"
	ClassRateLimit       Class = "rate-limit-missing"
	ClassVerboseError    Class = "verbose-error"
	ClassSensitiveFile   Class = "sensitive-file"
	ClassCSRF            Class = "csrf"
	ClassOther           Class = "other"
)

// AllClasses lists every valid Class in a stable order.
var AllClasses = []Class{
	ClassSQLi, ClassXSSReflected, ClassXSSStored, ClassIDOR, ClassAuthMissing,
	ClassAuthBypass, ClassDefaultCreds, ClassOpenRedirect, ClassPathTraversal,
	ClassSSRF, ClassInfoDisclosure, ClassSecurityHeaders, ClassCORS,
	ClassMassAssignment, ClassJWTWeak, ClassRateLimit, ClassVerboseError,
	ClassSensitiveFile, ClassCSRF, ClassOther,
}

// Location pins a finding to a request shape. PathTemplate is normalized
// (ids and uuids replaced) by internal/finding.
type Location struct {
	Method       string `json:"method"`
	PathTemplate string `json:"path_template"`
	Param        string `json:"param,omitempty"`
	Component    string `json:"component,omitempty"`
}

// ModelRef identifies which model, through which provider, acting as which
// agent produced something.
type ModelRef struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Agent    string `json:"agent"`
}

// AuditRef points at an audit-log record by run and sequence number.
type AuditRef struct {
	RunID string `json:"run_id"`
	Seq   uint64 `json:"seq"`
}

// Evidence is one piece of control-plane proof behind a finding. It never
// carries bodies; only digests, status codes and non-secret markers.
type Evidence struct {
	Kind           string `json:"kind"` // http-exchange | marker-reflected | header-missing | auth-bypass | tool-output
	ToolCallID     string `json:"tool_call_id,omitempty"`
	AuditSeq       uint64 `json:"audit_seq,omitempty"`
	RequestDigest  string `json:"request_digest,omitempty"`
	ResponseDigest string `json:"response_digest,omitempty"`
	StatusCode     int    `json:"status_code,omitempty"`
	Marker         string `json:"marker,omitempty"`
}

// ReconcileStatus records whether a finding's claimed tool calls matched
// the control plane.
type ReconcileStatus struct {
	Checked bool     `json:"checked"`
	Match   bool     `json:"match"`
	Reasons []string `json:"reasons,omitempty"`
}

// ValidationResult is the outcome of a deterministic exploitability check.
type ValidationResult struct {
	State     FindingState `json:"state"`
	Checker   string       `json:"checker"`
	Evidence  []Evidence   `json:"evidence,omitempty"`
	Note      string       `json:"note,omitempty"`
	CheckedAt time.Time    `json:"checked_at"`
}

// Remediation is the fix set generated for one finding.
type Remediation struct {
	CodeDiff     string `json:"code_diff,omitempty"`     // unified diff; synthetic lab only
	ConfigChange string `json:"config_change,omitempty"` // human-readable config step
	VirtualPatch string `json:"virtual_patch,omitempty"` // WAF rule text
	Verified     bool   `json:"verified"`                // rebuilt and re-checked
	Draft        bool   `json:"draft"`                   // model-drafted text, unverified
}

// Finding is the unit of output. ID and DedupKey are derived, never chosen
// by a model.
type Finding struct {
	ID           string            `json:"id"`
	DedupKey     string            `json:"dedup_key"`
	RunID        string            `json:"run_id"`
	Lab          string            `json:"lab"`
	Class        Class             `json:"class"`
	CWE          string            `json:"cwe,omitempty"`
	Location     Location          `json:"location"`
	Severity     Severity          `json:"severity"`
	State        FindingState      `json:"state"`
	Summary      string            `json:"summary"` // <=280 chars, redacted
	Evidence     []Evidence        `json:"evidence,omitempty"`
	Model        ModelRef          `json:"model"`
	PromptHash   string            `json:"prompt_hash,omitempty"`
	ToolCallIDs  []string          `json:"tool_call_ids,omitempty"`
	ControlPlane []AuditRef        `json:"control_plane,omitempty"`
	Reconcile    ReconcileStatus   `json:"reconcile"`
	Validation   *ValidationResult `json:"validation,omitempty"`
	Remediation  *Remediation      `json:"remediation,omitempty"`
	FirstSeen    time.Time         `json:"first_seen"`
	LastSeen     time.Time         `json:"last_seen"`
}

// ToolManifest is a signed tool definition. Signature covers the canonical
// JSON of the manifest with Signature removed.
type ToolManifest struct {
	Name          string         `json:"name"`
	Version       string         `json:"version"`
	Description   string         `json:"description"`
	InputSchema   map[string]any `json:"input_schema"`
	RiskTier      RiskTier       `json:"risk_tier"`
	Capabilities  []string       `json:"capabilities"` // e.g. http.read, http.write, auth.login, report, delegate
	SideEffects   bool           `json:"side_effects"`
	TimeoutMS     int64          `json:"timeout_ms"`
	AllowedAgents []string       `json:"allowed_agents,omitempty"`
	Signer        string         `json:"signer,omitempty"`    // key id
	SignedAt      string         `json:"signed_at,omitempty"` // RFC3339
	Signature     string         `json:"signature,omitempty"` // base64 Ed25519
}

// Budget caps spend and latency for a scope (run, model, task).
type Budget struct {
	MaxUSD       float64 `json:"max_usd"`
	MaxLatencyMS int64   `json:"max_latency_ms"`
	MaxTokens    int64   `json:"max_tokens,omitempty"`
}

// AgentBoundary is the execution boundary for one agent id.
type AgentBoundary struct {
	MaxTier            RiskTier `json:"max_tier"`
	AllowedTools       []string `json:"allowed_tools"`
	MaxDelegationDepth int      `json:"max_delegation_depth"`
	Budget             Budget   `json:"budget"`
}

// RuleMatch selects the requests a Rule applies to. Empty slices match all.
type RuleMatch struct {
	Tools        []string `json:"tools,omitempty"`
	Agents       []string `json:"agents,omitempty"`
	Labs         []string `json:"labs,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	MaxTier      RiskTier `json:"max_tier,omitempty"`
}

// Rule is one policy statement.
type Rule struct {
	ID        string    `json:"id"`
	Effect    Effect    `json:"effect"`
	Match     RuleMatch `json:"match"`
	Rationale string    `json:"rationale"`
}

// Policy is the deny-wins policy document.
type Policy struct {
	Version         string                   `json:"version"`
	DefaultEffect   Effect                   `json:"default_effect"`
	Rules           []Rule                   `json:"rules"`
	DangerousCombos [][]string               `json:"dangerous_combinations"`
	Agents          map[string]AgentBoundary `json:"agents"`
}

// PolicyRequest is what the engine evaluates.
type PolicyRequest struct {
	Agent           string       `json:"agent"`
	Tool            ToolManifest `json:"tool"`
	Lab             string       `json:"lab,omitempty"`
	Depth           int          `json:"depth"`
	Ancestors       []string     `json:"ancestors,omitempty"` // agent ids up the delegation chain
	SpentUSD        float64      `json:"spent_usd"`
	SignatureOK     bool         `json:"signature_ok"`
	SignatureReason string       `json:"signature_reason,omitempty"`
}

// Decision is a policy or gate outcome with a machine reason and a human
// rationale. Every Decision is audited.
type Decision struct {
	Effect       Effect   `json:"effect"`
	Reason       string   `json:"reason"` // code, e.g. DENY_RULE:no-write-in-recon
	Rationale    string   `json:"rationale"`
	MatchedRules []string `json:"matched_rules,omitempty"`
	PolicyHash   string   `json:"policy_hash,omitempty"`
}

// AuditRecord is the plaintext of one audit-log entry. Bodies never appear;
// only digests and bounded metadata.
type AuditRecord struct {
	Seq        uint64            `json:"seq"`
	Time       time.Time         `json:"time"`
	RunID      string            `json:"run_id"`
	TraceID    string            `json:"trace_id,omitempty"`
	TaskID     string            `json:"task_id,omitempty"`
	Agent      string            `json:"agent,omitempty"`
	Kind       string            `json:"kind"` // model_call | tool_call | policy_decision | gate_decision | router_decision | finding | delegation | score
	ToolCallID string            `json:"tool_call_id,omitempty"`
	Tool       string            `json:"tool,omitempty"`        // tool actually executed (control plane)
	ArgsDigest string            `json:"args_digest,omitempty"` // sha256(canon(args))
	Hashes     map[string]string `json:"hashes,omitempty"`      // prompt, response, result ...
	Meta       map[string]any    `json:"meta,omitempty"`        // model, tokens, latency_ms, cost_microusd, decision ...
	PrevHash   string            `json:"prev_hash"`
}

// ClaimedToolCall is a tool call as the model stated it (transcript side).
type ClaimedToolCall struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ArgsDigest string `json:"args_digest"`
}

// Usage is token and cost accounting for one model call.
type Usage struct {
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	LatencyMS        int64   `json:"latency_ms"`
}

// PairCell is one cell of the pairwise overlap matrix.
type PairCell struct {
	Both    int     `json:"both"`
	OnlyA   int     `json:"only_a"`
	OnlyB   int     `json:"only_b"`
	Jaccard float64 `json:"jaccard"`
}

// FileDigest records an input file hash (orbit-ir evidence manifest style).
type FileDigest struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// PRF is precision, recall and F1 against ground truth.
type PRF struct {
	TP        int     `json:"tp"`
	FP        int     `json:"fp"`
	FN        int     `json:"fn"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
}

// OverlapMatrix is the published cross-model overlap result.
type OverlapMatrix struct {
	Models       []string            `json:"models"`
	Keys         []string            `json:"keys"`
	Membership   map[string][]int    `json:"membership"` // key -> model indexes
	Pairwise     [][]PairCell        `json:"pairwise"`
	UniquePer    map[string][]string `json:"unique_per_model"`
	Union        int                 `json:"union"`
	Intersection int                 `json:"intersection"`
	Inputs       []FileDigest        `json:"inputs"`
}

// PathStep is one hop in an attack path (orbit-ir trajectory style).
type PathStep struct {
	Type string `json:"type"` // finding | precondition | fact | objective
	ID   string `json:"id"`
	Note string `json:"note,omitempty"`
}

// AttackPath chains findings to an objective.
type AttackPath struct {
	ID         string       `json:"id"`
	Lab        string       `json:"lab"`
	Objective  string       `json:"objective"` // account-takeover | data-access | privilege-escalation | code-exec
	Path       []PathStep   `json:"path"`
	Confidence FindingState `json:"confidence"` // min over path: validated > theorized
	SourceRefs []string     `json:"source_refs"`
}

// ModelResult summarizes one model's run.
type ModelResult struct {
	Ref       ModelRef             `json:"ref"`
	Findings  map[FindingState]int `json:"findings_by_state"`
	Usage     Usage                `json:"usage"`
	P50MS     int64                `json:"p50_ms"`
	P95MS     int64                `json:"p95_ms"`
	Refusals  int                  `json:"refusals"`
	Failovers int                  `json:"failovers"`
}

// ReflectionResult records one task's self-reflection: the model's own scores
// for its report, in order, and how many revisions followed. Critique text is
// never stored; only its hash reaches the audit log.
type ReflectionResult struct {
	Lab       string `json:"lab"`
	Model     string `json:"model"`
	Scored    bool   `json:"scored"`
	Scores    []int  `json:"scores,omitempty"`
	Revisions int    `json:"revisions"`
	Final     int    `json:"final_score,omitempty"`
	Passed    bool   `json:"passed"`
	Note      string `json:"note,omitempty"` // why reflection did not happen, when it did not
}

// RunReport is the committed summary of a run.
type RunReport struct {
	RunID        string             `json:"run_id"`
	GitSHA       string             `json:"git_sha"`
	CIRunURL     string             `json:"ci_run_url,omitempty"`
	Mode         string             `json:"mode"`             // full | degraded
	Config       string             `json:"config,omitempty"` // run config name
	Label        string             `json:"label,omitempty"`  // run config label, e.g. which kind of models ran
	Started      time.Time          `json:"started"`
	Finished     time.Time          `json:"finished"`
	Scope        Decision           `json:"scope"`
	Models       []ModelResult      `json:"models"`
	Labs         []string           `json:"labs"`
	Overlap      *OverlapMatrix     `json:"overlap,omitempty"`
	Precision    map[string]PRF     `json:"precision,omitempty"` // model -> PRF (ground-truth labs only)
	Chains       []AttackPath       `json:"chains,omitempty"`
	Reflection   []ReflectionResult `json:"reflection,omitempty"`
	Reconcile    map[string]int     `json:"reconcile_reason_counts,omitempty"`
	PolicyDenies map[string]int     `json:"policy_denies_by_reason,omitempty"`
	AuditHead    string             `json:"audit_head_hash"`
	AuditCount   uint64             `json:"audit_record_count"`
	BudgetCapUSD float64            `json:"budget_cap_usd"`
	SpentUSD     float64            `json:"spent_usd"`
	Truncated    bool               `json:"truncated"`
	Verdict      string             `json:"verdict"` // PASS | BLOCKED | ERROR
	ExitCode     int                `json:"exit_code"`
}

// Task is one unit of orchestrated work.
type Task struct {
	ID        string   `json:"id"`
	Wave      int      `json:"wave"`
	DependsOn []string `json:"depends_on,omitempty"`
	Kind      string   `json:"kind"` // recon | probe | validate | remediate | score
	Agent     string   `json:"agent"`
	Depth     int      `json:"depth"`
	Parent    string   `json:"parent,omitempty"`
	Lab       string   `json:"lab"`
}
