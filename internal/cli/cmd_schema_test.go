package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaDefault(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runSchema(nil, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"$schema"`) {
		t.Error("output should contain $schema")
	}
	if !strings.Contains(out, `"run_id"`) {
		t.Error("output should contain run_id property")
	}
}

func TestSchemaIsValidJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runSchema(nil, &stdout, &stderr)
	if code != ExitPass {
		t.Fatal("expected ExitPass")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if parsed["title"] != "Assay Run Report" {
		t.Errorf("unexpected title: %v", parsed["title"])
	}
}

func TestSchemaContainsAllTopLevelFields(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runSchema(nil, &stdout, &stderr)
	out := stdout.String()
	for _, field := range []string{
		"run_id", "git_sha", "mode", "started", "finished",
		"scope", "models", "labs", "verdict", "exit_code",
		"budget_cap_usd", "spent_usd", "truncated",
		"audit_head_hash", "audit_record_count",
	} {
		if !strings.Contains(out, `"`+field+`"`) {
			t.Errorf("schema should contain field %q", field)
		}
	}
}

func TestSchemaContainsDefs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runSchema(nil, &stdout, &stderr)
	out := stdout.String()
	for _, def := range []string{
		"model_ref", "usage", "model_result", "decision",
		"prf", "overlap_matrix", "attack_path", "reflection_result",
	} {
		if !strings.Contains(out, `"`+def+`"`) {
			t.Errorf("schema should contain $def %q", def)
		}
	}
}

func TestSchemaFindings(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runSchema([]string{"--type", "findings"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"Assay Findings"`) {
		t.Error("output should contain Assay Findings title")
	}
	if !strings.Contains(out, `"dedup_key"`) {
		t.Error("output should contain dedup_key")
	}
}

func TestSchemaFindingsIsValidJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runSchema([]string{"--type", "findings"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatal("expected ExitPass")
	}
	var parsed interface{}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("findings schema is not valid JSON: %v", err)
	}
}

func TestSchemaFindingsContainsDefs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runSchema([]string{"--type", "findings"}, &stdout, &stderr)
	out := stdout.String()
	for _, def := range []string{
		"finding", "location", "model_ref", "audit_ref",
		"evidence", "reconcile_status", "validation_result", "remediation",
	} {
		if !strings.Contains(out, `"`+def+`"`) {
			t.Errorf("findings schema should contain $def %q", def)
		}
	}
}

func TestSchemaUnknownType(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runSchema([]string{"--type", "bogus"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError for unknown type, got %d", code)
	}
}

func TestSchemaExplicitRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runSchema([]string{"--type", "run"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Assay Run Report") {
		t.Error("should contain title")
	}
}
