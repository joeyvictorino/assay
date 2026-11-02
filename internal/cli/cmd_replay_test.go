package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/policy"
)

func TestReplayDetectsChanges(t *testing.T) {
	dir := t.TempDir()

	strictYAML, _ := policy.GenerateTemplate("strict")
	strictPath := filepath.Join(dir, "strict.yaml")
	if err := os.WriteFile(strictPath, []byte(strictYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	strictEng, err := policy.Parse([]byte(strictYAML))
	if err != nil {
		t.Fatal(err)
	}
	strictP := strictEng.Policy()

	toolName := "read-tool"
	for _, name := range sortedNames(strictP.Agents) {
		a := strictP.Agents[name]
		for _, tn := range a.AllowedTools {
			if tn != "*" {
				toolName = tn
				break
			}
		}
	}

	manifestDir := filepath.Join(dir, "manifests")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := model.ToolManifest{
		Name:         toolName,
		Version:      "1.0.0",
		RiskTier:     model.TierLow,
		Capabilities: []string{"http.read"},
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(manifestDir, toolName+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	agentName := sortedNames(strictP.Agents)[0]
	input := ReplayInput{
		PolicyHash: "abc123",
		Decisions: []ReplayDecision{
			{Agent: agentName, Tool: toolName, Effect: "allow"},
			{Agent: "nonexistent-agent", Tool: toolName, Effect: "allow"},
		},
	}
	inputBytes, _ := json.Marshal(input)
	runPath := filepath.Join(dir, "decisions.json")
	if err := os.WriteFile(runPath, inputBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	runReplay([]string{
		"--run", runPath,
		"--policy", strictPath,
		"--manifests", manifestDir,
		"--format", "json",
	}, &stdout, &stderr)

	var report ReplayReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse output: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	if report.TotalDecisions != 2 {
		t.Errorf("total decisions = %d, want 2", report.TotalDecisions)
	}
	if report.OriginalPolicyHash != "abc123" {
		t.Errorf("original hash = %q, want abc123", report.OriginalPolicyHash)
	}
	if report.NewPolicyHash == "" {
		t.Error("new policy hash should not be empty")
	}
}

func TestReplayTextFormat(t *testing.T) {
	dir := t.TempDir()

	minYAML, _ := policy.GenerateTemplate("minimal")
	policyFile := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(policyFile, []byte(minYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	minEng, _ := policy.Parse([]byte(minYAML))
	minP := minEng.Policy()
	agentName := sortedNames(minP.Agents)[0]

	manifestDir := filepath.Join(dir, "manifests")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := model.ToolManifest{Name: "test-tool", Version: "1.0.0", RiskTier: model.TierLow}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(manifestDir, "test-tool.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	input := ReplayInput{
		Decisions: []ReplayDecision{
			{Agent: agentName, Tool: "test-tool", Effect: "deny"},
		},
	}
	inputBytes, _ := json.Marshal(input)
	runPath := filepath.Join(dir, "decisions.json")
	if err := os.WriteFile(runPath, inputBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	runReplay([]string{
		"--run", runPath,
		"--policy", policyFile,
		"--manifests", manifestDir,
		"--format", "text",
	}, &stdout, &stderr)

	output := stdout.String()
	if !strings.Contains(output, "Replay:") {
		t.Errorf("text output should contain 'Replay:', got: %s", output)
	}
}

func TestReplayNoChanges(t *testing.T) {
	dir := t.TempDir()

	strictYAML, _ := policy.GenerateTemplate("strict")
	policyFile := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(policyFile, []byte(strictYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	eng, _ := policy.Parse([]byte(strictYAML))
	p := eng.Policy()
	agentName := sortedNames(p.Agents)[0]

	toolName := "read-tool"
	for _, name := range sortedNames(p.Agents) {
		a := p.Agents[name]
		for _, tn := range a.AllowedTools {
			if tn != "*" {
				toolName = tn
				break
			}
		}
	}

	manifestDir := filepath.Join(dir, "manifests")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := model.ToolManifest{Name: toolName, Version: "1.0.0", RiskTier: model.TierLow, Capabilities: []string{"http.read"}}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(manifestDir, toolName+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	evalReq := model.PolicyRequest{
		Agent:           agentName,
		Tool:            m,
		SignatureOK:     true,
		SignatureReason: "TEST",
	}
	d := eng.Evaluate(nil, evalReq)

	input := ReplayInput{
		Decisions: []ReplayDecision{
			{Agent: agentName, Tool: toolName, Effect: string(d.Effect)},
		},
	}
	inputBytes, _ := json.Marshal(input)
	runPath := filepath.Join(dir, "decisions.json")
	if err := os.WriteFile(runPath, inputBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runReplay([]string{
		"--run", runPath,
		"--policy", policyFile,
		"--manifests", manifestDir,
		"--format", "json",
	}, &stdout, &stderr)

	if code != ExitPass {
		t.Errorf("expected ExitPass (0) for no changes, got %d; stderr: %s", code, stderr.String())
	}

	var report ReplayReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if report.ChangedCount != 0 {
		t.Errorf("changed count = %d, want 0", report.ChangedCount)
	}
}

func TestReplayMissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runReplay([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func sortedNames(agents map[string]model.AgentBoundary) []string {
	names := make([]string, 0, len(agents))
	for n := range agents {
		names = append(names, n)
	}
	return names
}
