// Package tools holds the tool manifests the models see and the executor
// that runs them. Every outbound HTTP request goes through Env.HTTP, which
// the lead wires to the gated client; this package never opens sockets.
package tools

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"

	"github.com/joeyvictorino/assay/internal/model"
)

//go:embed manifests/*.json
var manifestFS embed.FS

// Names lists every bundled tool in a stable order.
var Names = []string{"delegate", "http_get", "http_post", "inspect_headers", "login_as", "report_finding", "score_task"}

// Manifests loads every bundled manifest, sorted by name. The manifests are
// unsigned here; the signer (internal/toolsig) adds Signer/SignedAt/Signature.
func Manifests() ([]model.ToolManifest, error) {
	entries, err := fs.ReadDir(manifestFS, "manifests")
	if err != nil {
		return nil, fmt.Errorf("tools: read manifests: %w", err)
	}
	var out []model.ToolManifest
	for _, e := range entries {
		b, err := manifestFS.ReadFile("manifests/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("tools: read %s: %w", e.Name(), err)
		}
		var m model.ToolManifest
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("tools: parse %s: %w", e.Name(), err)
		}
		if err := ValidateManifest(m); err != nil {
			return nil, fmt.Errorf("tools: %s: %w", e.Name(), err)
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Manifest returns one bundled manifest by name.
func Manifest(name string) (model.ToolManifest, error) {
	all, err := Manifests()
	if err != nil {
		return model.ToolManifest{}, err
	}
	for _, m := range all {
		if m.Name == name {
			return m, nil
		}
	}
	return model.ToolManifest{}, fmt.Errorf("tools: unknown tool %q", name)
}

// ValidateManifest checks the structural rules every manifest must meet:
// a name, version, description, risk tier, at least one capability, a
// positive timeout, and a strict object schema (additionalProperties false
// and a required list covering every property).
func ValidateManifest(m model.ToolManifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest without name")
	}
	if m.Version == "" {
		return fmt.Errorf("%s: missing version", m.Name)
	}
	if m.Description == "" {
		return fmt.Errorf("%s: missing description", m.Name)
	}
	if m.RiskTier.Rank() == 0 {
		return fmt.Errorf("%s: invalid risk_tier %q", m.Name, m.RiskTier)
	}
	if len(m.Capabilities) == 0 {
		return fmt.Errorf("%s: no capabilities", m.Name)
	}
	if m.TimeoutMS <= 0 {
		return fmt.Errorf("%s: timeout_ms must be positive", m.Name)
	}
	s := m.InputSchema
	if s == nil {
		return fmt.Errorf("%s: missing input_schema", m.Name)
	}
	if s["type"] != "object" {
		return fmt.Errorf("%s: input_schema.type must be object", m.Name)
	}
	if ap, ok := s["additionalProperties"].(bool); !ok || ap {
		return fmt.Errorf("%s: input_schema.additionalProperties must be false", m.Name)
	}
	props, _ := s["properties"].(map[string]any)
	reqRaw, ok := s["required"].([]any)
	if !ok {
		return fmt.Errorf("%s: input_schema.required must be an array", m.Name)
	}
	required := map[string]bool{}
	for _, r := range reqRaw {
		name, ok := r.(string)
		if !ok {
			return fmt.Errorf("%s: required entries must be strings", m.Name)
		}
		if _, exists := props[name]; !exists {
			return fmt.Errorf("%s: required %q is not a property", m.Name, name)
		}
		required[name] = true
	}
	for name := range props {
		if !required[name] {
			return fmt.Errorf("%s: property %q must be required (strict schema)", m.Name, name)
		}
	}
	return nil
}
