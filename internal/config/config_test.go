package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider/fake"
)

const good = `
name: test
default_model: claude-sonnet-5-5
providers:
  anthropic: {type: anthropic, api_key_env: ANTHROPIC_API_KEY}
  llama: {type: openaicompat, base_url: http://127.0.0.1:8081/v1}
  fake: {type: fake}
models:
  - {provider: anthropic, model: claude-sonnet-5-5}
  - {provider: llama, model: local-8b, max_latency_ms: 600000}
  - {provider: fake, model: fake-model}
routes:
  recon:     [{provider: anthropic, model: claude-sonnet-5-5}, {provider: llama, model: local-8b}]
  probe:     [{provider: anthropic, model: claude-sonnet-5-5}]
  validate:  [{provider: anthropic, model: claude-sonnet-5-5}]
  remediate: [{provider: anthropic, model: claude-sonnet-5-5}]
  score:     [{provider: fake, model: fake-model}]
budgets:
  per_run:   {max_usd: 15}
  per_model: {max_usd: 6}
  per_task:  {max_usd: 1.5, max_latency_ms: 120000}
cost_per_mtok:
  claude-sonnet-5-5: {input_usd: 2, output_usd: 10, cache_read_usd: 0.20}
  local-8b: {input_usd: 0, output_usd: 0, cache_read_usd: 0}
  fake-model: {input_usd: 1, output_usd: 5, cache_read_usd: 0.1}
max_failovers: 1
agent: {max_turns: 8, max_tokens: 2048}
`

func TestParseGood(t *testing.T) {
	c, err := Parse([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "test" || c.DefaultModel != "claude-sonnet-5-5" || len(c.Providers) != 3 {
		t.Fatalf("config = %+v", c)
	}
	rc := c.RouterConfig("run-x")
	if rc.RunID != "run-x" || rc.MaxFailovers != 1 {
		t.Fatalf("router config = %+v", rc)
	}
	if len(rc.Routes["recon"]) != 2 || rc.Routes["recon"][1].Model != "local-8b" || rc.Routes["recon"][0].Agent != "recon" {
		t.Fatalf("routes = %+v", rc.Routes)
	}
	if rc.Budgets.PerRun.MaxUSD != 15 || rc.Budgets.PerModel.MaxUSD != 6 || rc.Budgets.PerTask.MaxUSD != 1.5 || rc.Budgets.PerTask.MaxLatencyMS != 120000 {
		t.Fatalf("budgets = %+v", rc.Budgets)
	}
	if rc.ModelLatencyMS["local-8b"] != 600000 {
		t.Fatalf("latency overrides = %v", rc.ModelLatencyMS)
	}
	if rc.Costs["claude-sonnet-5-5"].OutputUSD != 10 || rc.Costs["claude-sonnet-5-5"].CacheReadUSD != 0.20 {
		t.Fatalf("costs = %v", rc.Costs)
	}
	refs := c.ModelRefs()
	if len(refs) != 3 || refs[0].Provider != "anthropic" || refs[1].Provider != "fake" || refs[2].Provider != "llama" {
		t.Fatalf("model refs = %+v", refs)
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name string
		edit func(string) string
		want string
	}{
		{"unknown provider type", func(s string) string { return strings.Replace(s, "type: fake}", "type: bard}", 1) }, "unknown type"},
		{"route unknown provider", func(s string) string {
			return strings.Replace(s, "{provider: fake, model: fake-model}]", "{provider: ghost, model: fake-model}]", 1)
		}, "unknown provider"},
		{"route missing cost", func(s string) string {
			return strings.Replace(s, "  fake-model: {input_usd: 1, output_usd: 5, cache_read_usd: 0.1}\n", "", 1)
		}, "missing cost_per_mtok"},
		{"missing task kind", func(s string) string {
			return strings.Replace(s, "  score:     [{provider: fake, model: fake-model}]\n", "", 1)
		}, `task kind "score"`},
		{"unknown task kind", func(s string) string {
			return strings.Replace(s, "  score:     [{provider: fake, model: fake-model}]\n", "  score:     [{provider: fake, model: fake-model}]\n  exploit:   [{provider: fake, model: fake-model}]\n", 1)
		}, "unknown task kind"},
		{"openaicompat without base_url", func(s string) string { return strings.Replace(s, ", base_url: http://127.0.0.1:8081/v1", "", 1) }, "requires base_url"},
		{"date suffix", func(s string) string { return strings.ReplaceAll(s, "claude-sonnet-5-5", "claude-sonnet-5-5-20260901") }, "date suffix"},
		{"zero budget", func(s string) string {
			return strings.Replace(s, "per_run:   {max_usd: 15}", "per_run:   {max_usd: 0}", 1)
		}, "must be positive"},
		{"inverted budgets", func(s string) string {
			return strings.Replace(s, "per_model: {max_usd: 6}", "per_model: {max_usd: 60}", 1)
		}, "per_task <= per_model <= per_run"},
		{"no latency", func(s string) string { return strings.Replace(s, ", max_latency_ms: 120000", "", 1) }, "max_latency_ms"},
		{"undeclared route model", func(s string) string { return strings.Replace(s, "  - {provider: fake, model: fake-model}\n", "", 1) }, "not declared under models"},
		{"default model unpriced", func(s string) string {
			return strings.Replace(s, "default_model: claude-sonnet-5-5", "default_model: claude-opus-5-5", 1)
		}, "default_model"},
		{"unknown field", func(s string) string { return s + "surprise: 1\n" }, "yaml"},
		{"negative failovers", func(s string) string { return strings.Replace(s, "max_failovers: 1", "max_failovers: -2", 1) }, "max_failovers"},
		{"negative cost", func(s string) string { return strings.Replace(s, "input_usd: 1,", "input_usd: -1,", 1) }, "negative price"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.edit(good)))
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestLoadRepoConfigs(t *testing.T) {
	for _, name := range []string{"ci.yaml", "local.yaml"} {
		t.Run(name, func(t *testing.T) {
			c, err := Load(filepath.Join("..", "..", "runs", name))
			if err != nil {
				t.Fatal(err)
			}
			if c.Budgets.PerRun.MaxUSD != 15 || c.Budgets.PerModel.MaxUSD != 6 || c.Budgets.PerTask.MaxUSD != 1.5 {
				t.Fatalf("budgets = %+v", c.Budgets)
			}
			for _, kind := range TaskKinds {
				if len(c.Routes[kind]) == 0 {
					t.Fatalf("no route for %s", kind)
				}
			}
			sonnet, ok := c.CostPerMTok["claude-sonnet-5-5"]
			if !ok || sonnet.InputUSD != 2 || sonnet.OutputUSD != 10 || sonnet.CacheReadUSD != 0.20 {
				t.Fatalf("sonnet cost = %+v", sonnet)
			}
			if c.DefaultModel != "claude-sonnet-5-5" {
				t.Fatalf("default model = %q", c.DefaultModel)
			}
		})
	}
	if _, err := Load("runs/does-not-exist.yaml"); err == nil {
		t.Fatal("missing file accepted")
	}
}

func TestLocalConfigLatency(t *testing.T) {
	c, err := Load(filepath.Join("..", "..", "runs", "local.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	rc := c.RouterConfig("r")
	var local bool
	for name, p := range c.Providers {
		if p.Type == "openaicompat" && strings.Contains(p.BaseURL, "127.0.0.1") {
			local = true
			for _, m := range c.Models {
				if m.Provider == name && rc.ModelLatencyMS[m.Model] != 600000 {
					t.Fatalf("local model %s latency = %d", m.Model, rc.ModelLatencyMS[m.Model])
				}
			}
		}
	}
	if !local {
		t.Fatal("local.yaml has no llama-server provider")
	}
}

func TestBuildRegistry(t *testing.T) {
	c, err := Parse([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	env := func(k string) string {
		if k == "ANTHROPIC_API_KEY" {
			return "sk-test"
		}
		return ""
	}
	reg, err := c.BuildRegistry(nil, env)
	if err != nil {
		t.Fatal(err)
	}
	if names := reg.Names(); len(names) != 3 || names[0] != "anthropic" || names[1] != "fake" || names[2] != "llama" {
		t.Fatalf("names = %v", names)
	}
	p, _ := reg.Get("fake")
	if p.Name() != "fake" {
		t.Fatalf("fake name = %s", p.Name())
	}
	// Missing key is an error, not a silent unauthenticated client.
	if _, err := c.BuildRegistry(nil, func(string) string { return "" }); err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("err = %v", err)
	}
	// Overrides win and bypass env requirements.
	scripted := fake.New([]fake.Step{fake.Text("x")}, fake.Options{Name: "scripted"})
	reg, err = c.BuildRegistry(map[string]model.Provider{"anthropic": scripted}, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := reg.Get("anthropic"); p.Name() != "scripted" {
		t.Fatalf("override ignored: %s", p.Name())
	}
}
