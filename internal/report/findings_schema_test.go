package report

import (
	"encoding/json"
	"testing"
)

func TestFindingsJSONSchemaIsValidJSON(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(FindingsJSONSchema), &parsed); err != nil {
		t.Fatalf("FindingsJSONSchema is not valid JSON: %v", err)
	}
	if parsed["title"] != "Assay Findings" {
		t.Errorf("unexpected title: %v", parsed["title"])
	}
}

func TestFindingsJSONSchemaIsArray(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(FindingsJSONSchema), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["type"] != "array" {
		t.Errorf("expected type array, got %v", parsed["type"])
	}
}

func TestFindingsJSONSchemaHasDefs(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(FindingsJSONSchema), &parsed); err != nil {
		t.Fatal(err)
	}
	defs, ok := parsed["$defs"].(map[string]interface{})
	if !ok {
		t.Fatal("$defs should be an object")
	}
	for _, def := range []string{
		"finding", "location", "model_ref", "audit_ref",
		"evidence", "reconcile_status", "validation_result", "remediation",
	} {
		if _, exists := defs[def]; !exists {
			t.Errorf("missing $def: %s", def)
		}
	}
}

func TestFindingsJSONSchemaClassEnum(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(FindingsJSONSchema), &parsed); err != nil {
		t.Fatal(err)
	}
	defs := parsed["$defs"].(map[string]interface{})
	finding := defs["finding"].(map[string]interface{})
	props := finding["properties"].(map[string]interface{})
	class := props["class"].(map[string]interface{})
	enums, ok := class["enum"].([]interface{})
	if !ok {
		t.Fatal("class.enum should be an array")
	}
	expected := []string{
		"sqli", "xss-reflected", "xss-stored", "idor", "auth-missing",
		"auth-bypass", "default-creds", "open-redirect", "path-traversal",
		"ssrf", "info-disclosure", "security-headers", "cors-misconfig",
		"mass-assignment", "jwt-weak", "rate-limit-missing", "verbose-error",
		"sensitive-file", "csrf", "other",
	}
	if len(enums) != len(expected) {
		t.Fatalf("expected %d class enums, got %d", len(expected), len(enums))
	}
	for i, v := range expected {
		if enums[i] != v {
			t.Errorf("class enum[%d]: expected %q, got %v", i, v, enums[i])
		}
	}
}

func TestFindingsJSONSchemaRequiredFields(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(FindingsJSONSchema), &parsed); err != nil {
		t.Fatal(err)
	}
	defs := parsed["$defs"].(map[string]interface{})
	finding := defs["finding"].(map[string]interface{})
	required, ok := finding["required"].([]interface{})
	if !ok {
		t.Fatal("finding.required should be an array")
	}
	want := map[string]bool{
		"id": true, "dedup_key": true, "run_id": true, "lab": true,
		"class": true, "location": true, "severity": true, "state": true,
		"summary": true, "model": true, "reconcile": true,
		"first_seen": true, "last_seen": true,
	}
	got := make(map[string]bool)
	for _, v := range required {
		got[v.(string)] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing required field: %s", k)
		}
	}
}
