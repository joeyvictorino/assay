package policy

import (
	"encoding/json"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestImportJSONRoundTrip(t *testing.T) {
	yaml, _ := GenerateTemplate("standard")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	p := eng.Policy()

	js, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	imported, err := ImportJSON(js)
	if err != nil {
		t.Fatal(err)
	}

	if imported.Version != p.Version {
		t.Errorf("version = %q, want %q", imported.Version, p.Version)
	}
	if imported.DefaultEffect != p.DefaultEffect {
		t.Errorf("default_effect = %q, want %q", imported.DefaultEffect, p.DefaultEffect)
	}
	if len(imported.Rules) != len(p.Rules) {
		t.Errorf("rules count = %d, want %d", len(imported.Rules), len(p.Rules))
	}
	if len(imported.Agents) != len(p.Agents) {
		t.Errorf("agents count = %d, want %d", len(imported.Agents), len(p.Agents))
	}
}

func TestImportJSONMissingVersion(t *testing.T) {
	js := `{"default_effect":"deny","agents":{"a":{"max_tier":"low","allowed_tools":["*"],"budget":{"max_usd":1,"max_latency_ms":30000}}}}`
	_, err := ImportJSON([]byte(js))
	if err == nil {
		t.Error("expected error for missing version")
	}
}

func TestImportJSONMissingDefaultEffect(t *testing.T) {
	js := `{"version":"1.0","agents":{"a":{"max_tier":"low","allowed_tools":["*"],"budget":{"max_usd":1,"max_latency_ms":30000}}}}`
	_, err := ImportJSON([]byte(js))
	if err == nil {
		t.Error("expected error for missing default_effect")
	}
}

func TestImportCSVBasic(t *testing.T) {
	csv := `effect,agent,tool,capabilities
allow,scanner,http_get,http.read
deny,scanner,shell_exec,code.exec;shell
allow,analyzer,http_get,http.read
`
	p, err := ImportCSV([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}

	if len(p.Rules) != 3 {
		t.Errorf("rules count = %d, want 3", len(p.Rules))
	}
	if len(p.Agents) != 2 {
		t.Errorf("agents count = %d, want 2 (scanner, analyzer)", len(p.Agents))
	}

	if p.Rules[0].Effect != model.EffectAllow {
		t.Errorf("first rule effect = %q, want allow", p.Rules[0].Effect)
	}
	if p.Rules[1].Effect != model.EffectDeny {
		t.Errorf("second rule effect = %q, want deny", p.Rules[1].Effect)
	}

	if len(p.Rules[1].Match.Capabilities) != 2 {
		t.Errorf("second rule caps = %d, want 2", len(p.Rules[1].Match.Capabilities))
	}
}

func TestImportCSVWithOptionalColumns(t *testing.T) {
	csv := `id,effect,agent,tool,capabilities,rationale
r1,allow,bot,read,,safe read
r2,deny,bot,write,write.file,too risky
`
	p, err := ImportCSV([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}

	if p.Rules[0].ID != "r1" {
		t.Errorf("first rule id = %q, want r1", p.Rules[0].ID)
	}
	if p.Rules[0].Rationale != "safe read" {
		t.Errorf("first rule rationale = %q, want 'safe read'", p.Rules[0].Rationale)
	}
	if p.Rules[1].ID != "r2" {
		t.Errorf("second rule id = %q, want r2", p.Rules[1].ID)
	}
}

func TestImportCSVMissingRequiredColumn(t *testing.T) {
	csv := `effect,tool,capabilities
allow,http_get,http.read
`
	_, err := ImportCSV([]byte(csv))
	if err == nil {
		t.Error("expected error for missing agent column")
	}
}

func TestImportCSVInvalidEffect(t *testing.T) {
	csv := `effect,agent,tool,capabilities
maybe,scanner,http_get,http.read
`
	_, err := ImportCSV([]byte(csv))
	if err == nil {
		t.Error("expected error for invalid effect")
	}
}

func TestImportCSVHeaderOnly(t *testing.T) {
	csv := `effect,agent,tool,capabilities
`
	_, err := ImportCSV([]byte(csv))
	if err == nil {
		t.Error("expected error for no data rows")
	}
}

func TestImportCSVAgentToolsCollected(t *testing.T) {
	csv := `effect,agent,tool,capabilities
allow,bot,read,
allow,bot,write,
allow,bot,delete,
`
	p, err := ImportCSV([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}

	boundary, ok := p.Agents["bot"]
	if !ok {
		t.Fatal("agent 'bot' not found")
	}
	if len(boundary.AllowedTools) != 3 {
		t.Errorf("bot tools = %d, want 3", len(boundary.AllowedTools))
	}
}
