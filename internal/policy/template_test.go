package policy

import (
	"testing"
)

func TestGenerateTemplatePresets(t *testing.T) {
	for _, preset := range TemplatePresets() {
		t.Run(preset, func(t *testing.T) {
			yaml, err := GenerateTemplate(preset)
			if err != nil {
				t.Fatalf("GenerateTemplate(%q) error: %v", preset, err)
			}
			if yaml == "" {
				t.Fatal("empty template")
			}
			eng, err := Parse([]byte(yaml))
			if err != nil {
				t.Fatalf("template %q does not parse: %v", preset, err)
			}
			p := eng.Policy()
			if p.Version == "" {
				t.Error("template has no version")
			}
			if len(p.Agents) == 0 {
				t.Error("template has no agents")
			}
			if len(p.Rules) == 0 {
				t.Error("template has no rules")
			}
			issues := Validate(p)
			for _, i := range issues {
				if i.Severity == "error" {
					t.Errorf("template %q has validation error: %s: %s", preset, i.Field, i.Message)
				}
			}
		})
	}
}

func TestGenerateTemplateUnknown(t *testing.T) {
	_, err := GenerateTemplate("nonexistent")
	if err == nil {
		t.Error("expected error for unknown preset")
	}
}

func TestStrictHasWeights(t *testing.T) {
	yaml, _ := GenerateTemplate("strict")
	eng, _ := Parse([]byte(yaml))
	p := eng.Policy()
	if len(p.Weights) == 0 {
		t.Error("strict template should have weights")
	}
	if len(p.DangerousCombos) < 2 {
		t.Error("strict template should have multiple dangerous combinations")
	}
}
