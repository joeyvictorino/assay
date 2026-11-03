package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/joeyvictorino/assay/internal/model"
	"gopkg.in/yaml.v3"
)

// ValidationIssue is one problem found during policy schema validation.
type ValidationIssue struct {
	Field    string `json:"field"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error" or "warning"
}

// ParseLenient reads a policy YAML file and decodes it into a model.Policy
// without running the strict validation that Load enforces. This allows
// the validate command to report all issues rather than failing on the first.
func ParseLenient(path string) (model.Policy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.Policy{}, fmt.Errorf("read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return model.Policy{}, fmt.Errorf("%s: empty document", path)
	}
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		return model.Policy{}, fmt.Errorf("%s: yaml: %v", path, err)
	}
	js, err := json.Marshal(generic)
	if err != nil {
		return model.Policy{}, fmt.Errorf("%s: %v", path, err)
	}
	var p model.Policy
	if err := json.Unmarshal(js, &p); err != nil {
		return model.Policy{}, fmt.Errorf("%s: %v", path, err)
	}
	return p, nil
}

// Validate checks a policy for structural correctness without needing
// tool manifests. It returns issues found; an empty slice means valid.
func Validate(p model.Policy) []ValidationIssue {
	var issues []ValidationIssue

	if p.Version == "" {
		issues = append(issues, ValidationIssue{
			Field: "version", Message: "version is required", Severity: "error",
		})
	}

	if p.DefaultEffect != model.EffectAllow && p.DefaultEffect != model.EffectDeny {
		issues = append(issues, ValidationIssue{
			Field:    "default_effect",
			Message:  fmt.Sprintf("default_effect must be \"allow\" or \"deny\", got %q", p.DefaultEffect),
			Severity: "error",
		})
	}

	validTiers := map[model.RiskTier]bool{
		model.TierLow: true, model.TierMedium: true,
		model.TierHigh: true, model.TierCritical: true,
	}

	for name, ab := range p.Agents {
		prefix := fmt.Sprintf("agents.%s", name)
		if !validTiers[ab.MaxTier] {
			issues = append(issues, ValidationIssue{
				Field:    prefix + ".max_tier",
				Message:  fmt.Sprintf("max_tier must be low|medium|high|critical, got %q", ab.MaxTier),
				Severity: "error",
			})
		}
		if ab.MaxDelegationDepth < 0 {
			issues = append(issues, ValidationIssue{
				Field:    prefix + ".max_delegation_depth",
				Message:  "max_delegation_depth must be non-negative",
				Severity: "error",
			})
		}
		if ab.Budget.MaxUSD < 0 {
			issues = append(issues, ValidationIssue{
				Field:    prefix + ".budget.max_usd",
				Message:  "budget.max_usd must be non-negative",
				Severity: "error",
			})
		}
	}

	ruleIDs := map[string]int{}
	for i, r := range p.Rules {
		prefix := fmt.Sprintf("rules[%d]", i)
		if r.ID == "" {
			issues = append(issues, ValidationIssue{
				Field: prefix + ".id", Message: "rule id is required", Severity: "error",
			})
		} else {
			if prev, exists := ruleIDs[r.ID]; exists {
				issues = append(issues, ValidationIssue{
					Field:    prefix + ".id",
					Message:  fmt.Sprintf("duplicate rule id %q (first at rules[%d])", r.ID, prev),
					Severity: "error",
				})
			}
			ruleIDs[r.ID] = i
		}

		if r.Effect != model.EffectAllow && r.Effect != model.EffectDeny {
			issues = append(issues, ValidationIssue{
				Field:    prefix + ".effect",
				Message:  fmt.Sprintf("effect must be \"allow\" or \"deny\", got %q", r.Effect),
				Severity: "error",
			})
		}

		if len(r.Match.Tools) == 0 && len(r.Match.Agents) == 0 &&
			len(r.Match.Labs) == 0 && len(r.Match.Capabilities) == 0 &&
			r.Match.MaxTier == "" {
			issues = append(issues, ValidationIssue{
				Field:    prefix + ".match",
				Message:  "match block must have at least one criterion",
				Severity: "warning",
			})
		}
	}

	for i, combo := range p.DangerousCombos {
		if len(combo) < 2 {
			issues = append(issues, ValidationIssue{
				Field:    fmt.Sprintf("dangerous_combinations[%d]", i),
				Message:  "each combination must have at least 2 items",
				Severity: "error",
			})
		}
	}

	for k, v := range p.Weights {
		if v < 0 || math.IsInf(v, 0) || math.IsNaN(v) {
			issues = append(issues, ValidationIssue{
				Field:    fmt.Sprintf("weights.%s", k),
				Message:  fmt.Sprintf("weight must be a finite non-negative number, got %v", v),
				Severity: "error",
			})
		}
	}

	return issues
}
