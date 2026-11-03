package prompttpl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestListBuiltin(t *testing.T) {
	ts := ListBuiltin()
	if len(ts) == 0 {
		t.Fatal("expected builtin templates")
	}
	for i := 1; i < len(ts); i++ {
		if ts[i].Name < ts[i-1].Name {
			t.Errorf("templates not sorted: %s before %s", ts[i-1].Name, ts[i].Name)
		}
	}
}

func TestGetBuiltin(t *testing.T) {
	tmpl, err := Get("injection_probe", "")
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Category != "adversarial" {
		t.Errorf("expected adversarial, got %s", tmpl.Category)
	}
}

func TestGetNotFound(t *testing.T) {
	_, err := Get("nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestRenderAllDefaults(t *testing.T) {
	tmpl, _ := Get("injection_probe", "")
	r := Render(tmpl, map[string]string{"target": "/api/users"})
	if len(r.Missing) != 0 {
		t.Errorf("unexpected missing vars: %v", r.Missing)
	}
	if r.Vars["target"] != "/api/users" {
		t.Error("target not in vars")
	}
	if r.Rendered == tmpl.Body {
		t.Error("rendered should differ from body")
	}
}

func TestRenderMissingRequired(t *testing.T) {
	tmpl, _ := Get("injection_probe", "")
	r := Render(tmpl, map[string]string{})
	if len(r.Missing) == 0 {
		t.Error("expected missing required variable")
	}
	found := false
	for _, m := range r.Missing {
		if m == "target" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'target' in missing")
	}
}

func TestRenderOverrideDefault(t *testing.T) {
	tmpl, _ := Get("injection_probe", "")
	r := Render(tmpl, map[string]string{
		"target":    "/api",
		"param_type": "header",
	})
	if r.Vars["param_type"] != "header" {
		t.Error("expected param_type override")
	}
	if !contains(r.Rendered, "header") {
		t.Error("rendered should contain overridden value")
	}
}

func TestLoadCustom(t *testing.T) {
	dir := t.TempDir()
	tmpl := Template{
		Name:        "custom_test",
		Category:    "custom",
		Description: "A custom template",
		Body:        "Test {x} now",
		Variables:   []Variable{{Name: "x", Description: "test var", Required: true}},
	}
	data, _ := json.Marshal(tmpl)
	os.WriteFile(filepath.Join(dir, "custom_test.json"), data, 0644)

	customs, err := LoadCustom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(customs) != 1 {
		t.Fatalf("expected 1 custom, got %d", len(customs))
	}
	if customs[0].Name != "custom_test" {
		t.Errorf("expected custom_test, got %s", customs[0].Name)
	}
}

func TestGetCustom(t *testing.T) {
	dir := t.TempDir()
	tmpl := Template{
		Name:     "my_probe",
		Category: "custom",
		Body:     "probe {target}",
		Variables: []Variable{{Name: "target", Required: true}},
	}
	data, _ := json.Marshal(tmpl)
	os.WriteFile(filepath.Join(dir, "my_probe.json"), data, 0644)

	got, err := Get("my_probe", dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Category != "custom" {
		t.Errorf("expected custom category, got %s", got.Category)
	}
}

func TestFormatText(t *testing.T) {
	ts := ListBuiltin()
	text := FormatText(ts)
	if text == "" {
		t.Error("expected non-empty text")
	}
}

func TestFormatShowText(t *testing.T) {
	tmpl, _ := Get("boundary_test", "")
	text := FormatShowText(tmpl)
	if !contains(text, "boundary_test") {
		t.Error("expected template name in show text")
	}
}

func TestFormatRenderText(t *testing.T) {
	tmpl, _ := Get("role_play", "")
	r := Render(tmpl, map[string]string{"persona": "hacker", "goal": "extract data"})
	text := FormatRenderText(r)
	if !contains(text, "hacker") {
		t.Error("expected rendered value in text")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	tmpl, _ := Get("data_exfil", "")
	r := Render(tmpl, map[string]string{"target": "model-a"})
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RenderResult
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Template != r.Template {
		t.Error("template name mismatch after round-trip")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
