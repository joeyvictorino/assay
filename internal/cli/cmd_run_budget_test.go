package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/config"
	"github.com/joeyvictorino/assay/internal/model"
)

func fakeOnlyConfig(perRun, perModel, perTask float64) string {
	return fmt.Sprintf(`name: budget-test
default_model: fake-model
providers:
  fake: { type: fake }
models:
  - { provider: fake, model: fake-model }
routes:
  recon: [{ provider: fake, model: fake-model }]
  probe: [{ provider: fake, model: fake-model }]
  validate: [{ provider: fake, model: fake-model }]
  remediate: [{ provider: fake, model: fake-model }]
  score: [{ provider: fake, model: fake-model }]
budgets:
  per_run: { max_usd: %g }
  per_model: { max_usd: %g }
  per_task: { max_usd: %g, max_latency_ms: 120000 }
cost_per_mtok:
  fake-model: { input_usd: 1, output_usd: 5, cache_read_usd: 0.10 }
max_failovers: 1
agent: { max_turns: 8, max_tokens: 2048 }
`, perRun, perModel, perTask)
}

// One scripted assessment of one lab costs about $0.0014 at these prices.
// Each (lab, model) assessment has its own router, so before the caps were
// carried across routers a run over two labs spent twice a cap that was
// meant for the whole run (and for the whole model) and still passed.
func TestRunAndModelBudgetsHoldAcrossLabs(t *testing.T) {
	cases := []struct {
		name                      string
		perRun, perModel, perTask float64
	}{
		{"per-run cap", 0.002, 0.002, 0.002},
		{"per-model cap", 15, 0.002, 0.002},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPipelineFixture(t, "NOT-A-SECRET-LAB-MARKER")
			fakeProviderFn = fakeScript
			cfgPath := filepath.Join(f.root, "budget.yaml")
			must(t, os.WriteFile(cfgPath, []byte(fakeOnlyConfig(tc.perRun, tc.perModel, tc.perTask)), 0o644))
			f.opts.config = cfgPath

			var out, errb bytes.Buffer
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			code, err := runPipeline(ctx, f.opts, &out, &errb)
			if err != nil {
				t.Fatalf("pipeline: %v\n%s", err, errb.String())
			}
			if code != ExitBlocked {
				t.Fatalf("exit %d, want %d (BLOCKED: the cap stopped the second lab)\n%s", code, ExitBlocked, errb.String())
			}
			var rr model.RunReport
			readJSON(t, filepath.Join(f.opts.out, f.opts.runID, "run.json"), &rr)
			if !rr.Truncated || rr.Verdict != "BLOCKED" {
				t.Fatalf("truncated=%v verdict=%s", rr.Truncated, rr.Verdict)
			}
			// The first lab ran to completion; the second was stopped at the cap.
			// The cap is checked before each call, so allow one call of overshoot.
			if rr.SpentUSD < 0.001 || rr.SpentUSD > tc.perRun+0.0005 || rr.SpentUSD > 0.0025 {
				t.Fatalf("spent $%.5f, want one lab's cost up to the $%g cap plus at most one call", rr.SpentUSD, tc.perModel)
			}
			var findings []model.Finding
			readJSON(t, filepath.Join(f.opts.out, f.opts.runID, "findings.redacted.json"), &findings)
			if len(findings) == 0 {
				t.Fatal("the first lab's findings were lost")
			}
		})
	}
}

func TestRejectPlaceholderModels(t *testing.T) {
	ok := &config.Config{Models: []config.ModelConfig{{Provider: "anthropic", Model: "claude-opus-5-5"}}}
	if err := rejectPlaceholderModels(ok); err != nil {
		t.Fatalf("real model ids rejected: %v", err)
	}
	bad := &config.Config{Models: []config.ModelConfig{
		{Provider: "anthropic", Model: "claude-opus-5-5"},
		{Provider: "openai", Model: placeholderPrefix + "OPENAI_MODEL_ID"},
	}}
	err := rejectPlaceholderModels(bad)
	if err == nil || !strings.Contains(err.Error(), "openai") || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("placeholder accepted or unhelpful error: %v", err)
	}
}

// runs/frontier.yaml ships with an OpenAI model id that Joey must fill in.
// While the OpenAI key is unset the provider is dropped and the placeholder
// never matters; once the key is set the run must refuse to start.
func TestFrontierConfigPlaceholderOnlyBitesWithAKey(t *testing.T) {
	cfgPath := filepath.Join(repoRoot(t), "runs", "frontier.yaml")
	load := func() *config.Config {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "")
	cfg := load()
	if dropped := dropMissingProviders(cfg); len(dropped) != 1 || dropped[0] != "openai" {
		t.Fatalf("dropped %v, want [openai]", dropped)
	}
	if err := rejectPlaceholderModels(cfg); err != nil {
		t.Fatalf("anthropic-only frontier run refused: %v", err)
	}
	got := map[string]bool{}
	for _, m := range cfg.Models {
		got[m.Provider+"/"+m.Model] = true
	}
	for _, want := range []string{"anthropic/claude-opus-5-5", "anthropic/claude-sonnet-5-5", "anthropic/claude-haiku-4-5"} {
		if !got[want] {
			t.Errorf("frontier run is missing %s (have %v)", want, got)
		}
	}
	for kind, routes := range cfg.Routes {
		for _, r := range routes {
			if r.Provider == "fake" {
				t.Errorf("route %s still points at the fake provider", kind)
			}
		}
	}

	t.Setenv("OPENAI_API_KEY", "test-key")
	cfg = load()
	if dropped := dropMissingProviders(cfg); len(dropped) != 0 {
		t.Fatalf("dropped %v with both keys set", dropped)
	}
	if err := rejectPlaceholderModels(cfg); err == nil {
		t.Fatal("run would call the unfilled OpenAI placeholder model id")
	}
	if cfg.Budgets.PerRun.MaxUSD != 15 {
		t.Errorf("per-run cap is $%g, want $15", cfg.Budgets.PerRun.MaxUSD)
	}
}
