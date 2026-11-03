package report

import (
	"encoding/json"
	"testing"
)

func TestRunJSONSchemaIsValidJSON(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(RunJSONSchema), &parsed); err != nil {
		t.Fatalf("RunJSONSchema is not valid JSON: %v", err)
	}
	if parsed["title"] != "Assay Run Report" {
		t.Errorf("unexpected title: %v", parsed["title"])
	}
}

func TestRunJSONSchemaHasRequiredFields(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(RunJSONSchema), &parsed); err != nil {
		t.Fatal(err)
	}
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be an object")
	}
	for _, key := range []string{"run_id", "verdict", "models", "labs", "exit_code"} {
		if _, exists := props[key]; !exists {
			t.Errorf("missing property: %s", key)
		}
	}
}

func TestRunJSONSchemaHasDefs(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(RunJSONSchema), &parsed); err != nil {
		t.Fatal(err)
	}
	defs, ok := parsed["$defs"].(map[string]interface{})
	if !ok {
		t.Fatal("$defs should be an object")
	}
	for _, def := range []string{"model_result", "usage", "decision", "attack_path", "reflection_result", "overlap_matrix", "prf"} {
		if _, exists := defs[def]; !exists {
			t.Errorf("missing $def: %s", def)
		}
	}
}

func TestRunJSONSchemaReflectionFields(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(RunJSONSchema), &parsed); err != nil {
		t.Fatal(err)
	}
	defs := parsed["$defs"].(map[string]interface{})
	refl := defs["reflection_result"].(map[string]interface{})
	props := refl["properties"].(map[string]interface{})
	for _, key := range []string{
		"reviewer", "reviewer_kind", "prompt_variant",
		"validated", "refuted", "theorized",
	} {
		if _, exists := props[key]; !exists {
			t.Errorf("ReflectionResult missing property: %s", key)
		}
	}
}
