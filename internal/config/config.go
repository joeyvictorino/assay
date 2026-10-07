// Package config loads and validates runs/*.yaml: providers, per-task-kind
// routes, budgets and the cost table. Nothing about prices or model ids is
// hard-coded in the harness; it all comes from here.
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider"
	anthropicprov "github.com/joeyvictorino/assay/internal/provider/anthropic"
	"github.com/joeyvictorino/assay/internal/provider/fake"
	"github.com/joeyvictorino/assay/internal/provider/openaicompat"
	"github.com/joeyvictorino/assay/internal/router"
)

// TaskKinds are the orchestrated task kinds every config must route.
var TaskKinds = []string{"recon", "probe", "validate", "remediate", "score"}

// ProviderTypes are the backends this package can construct.
var ProviderTypes = []string{"anthropic", "openaicompat", "fake"}

// ErrInvalid wraps every validation failure.
var ErrInvalid = errors.New("config: invalid")

var dateSuffix = regexp.MustCompile(`-\d{8}$`)

// ProviderConfig describes one backend.
type ProviderConfig struct {
	Type      string `yaml:"type"`
	BaseURL   string `yaml:"base_url,omitempty"`
	APIKeyEnv string `yaml:"api_key_env,omitempty"`
	TimeoutMS int64  `yaml:"timeout_ms,omitempty"`
}

// ModelConfig declares a model the routes may reference.
type ModelConfig struct {
	Provider     string `yaml:"provider"`
	Model        string `yaml:"model"`
	MaxLatencyMS int64  `yaml:"max_latency_ms,omitempty"`
}

// RouteRef is one ordered candidate for a task kind.
type RouteRef struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// Budget mirrors model.Budget with yaml tags.
type Budget struct {
	MaxUSD       float64 `yaml:"max_usd"`
	MaxLatencyMS int64   `yaml:"max_latency_ms,omitempty"`
	MaxTokens    int64   `yaml:"max_tokens,omitempty"`
}

func (b Budget) toModel() model.Budget {
	return model.Budget{MaxUSD: b.MaxUSD, MaxLatencyMS: b.MaxLatencyMS, MaxTokens: b.MaxTokens}
}

// Budgets groups the three scopes.
type Budgets struct {
	PerRun   Budget `yaml:"per_run"`
	PerModel Budget `yaml:"per_model"`
	PerTask  Budget `yaml:"per_task"`
}

// AgentConfig bounds the agent loop.
type AgentConfig struct {
	MaxTurns  int `yaml:"max_turns"`
	MaxTokens int `yaml:"max_tokens"`
}

// Config is the whole run configuration.
type Config struct {
	Name         string                    `yaml:"name"`
	DefaultModel string                    `yaml:"default_model"`
	Providers    map[string]ProviderConfig `yaml:"providers"`
	Models       []ModelConfig             `yaml:"models"`
	Routes       map[string][]RouteRef     `yaml:"routes"`
	Budgets      Budgets                   `yaml:"budgets"`
	CostPerMTok  map[string]provider.Cost  `yaml:"cost_per_mtok"`
	MaxFailovers int                       `yaml:"max_failovers"`
	Agent        AgentConfig               `yaml:"agent"`
}

// Load reads and validates a YAML file.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	return Parse(b)
}

// Parse decodes and validates YAML bytes.
func Parse(b []byte) (*Config, error) {
	var c Config
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%w: yaml: %v", ErrInvalid, err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// Validate checks referential integrity: every route names a declared
// provider of a known type and a model with a cost entry; every task kind
// has at least one candidate; budgets are positive.
func (c *Config) Validate() error {
	if len(c.Providers) == 0 {
		return invalid("no providers")
	}
	for name, p := range c.Providers {
		switch p.Type {
		case "anthropic", "fake":
		case "openaicompat":
			if p.BaseURL == "" {
				return invalid("provider %q: openaicompat requires base_url", name)
			}
		default:
			return invalid("provider %q: unknown type %q (want one of %s)", name, p.Type, strings.Join(ProviderTypes, ", "))
		}
	}
	if len(c.CostPerMTok) == 0 {
		return invalid("cost_per_mtok is empty")
	}
	for _, cost := range c.CostPerMTok {
		if cost.InputUSD < 0 || cost.OutputUSD < 0 || cost.CacheReadUSD < 0 {
			return invalid("cost_per_mtok: negative price")
		}
	}
	declared := map[string]ModelConfig{}
	for _, m := range c.Models {
		if m.Provider == "" || m.Model == "" {
			return invalid("models: provider and model are required")
		}
		if _, ok := c.Providers[m.Provider]; !ok {
			return invalid("models: unknown provider %q for model %q", m.Provider, m.Model)
		}
		if _, ok := c.CostPerMTok[m.Model]; !ok {
			return invalid("models: missing cost_per_mtok entry for %q", m.Model)
		}
		if c.Providers[m.Provider].Type == "anthropic" && dateSuffix.MatchString(m.Model) {
			return invalid("models: anthropic model id %q must not carry a date suffix", m.Model)
		}
		declared[m.Provider+"/"+m.Model] = m
	}
	if len(c.Routes) == 0 {
		return invalid("no routes")
	}
	for _, kind := range TaskKinds {
		if len(c.Routes[kind]) == 0 {
			return invalid("routes: task kind %q has no candidates", kind)
		}
	}
	for kind, refs := range c.Routes {
		if !contains(TaskKinds, kind) {
			return invalid("routes: unknown task kind %q", kind)
		}
		for _, r := range refs {
			p, ok := c.Providers[r.Provider]
			if !ok {
				return invalid("routes[%s]: unknown provider %q", kind, r.Provider)
			}
			if _, ok := c.CostPerMTok[r.Model]; !ok {
				return invalid("routes[%s]: missing cost_per_mtok entry for %q", kind, r.Model)
			}
			if p.Type == "anthropic" && dateSuffix.MatchString(r.Model) {
				return invalid("routes[%s]: anthropic model id %q must not carry a date suffix", kind, r.Model)
			}
			if len(c.Models) > 0 {
				if _, ok := declared[r.Provider+"/"+r.Model]; !ok {
					return invalid("routes[%s]: %s/%s is not declared under models", kind, r.Provider, r.Model)
				}
			}
		}
	}
	if c.DefaultModel != "" {
		if _, ok := c.CostPerMTok[c.DefaultModel]; !ok {
			return invalid("default_model %q has no cost_per_mtok entry", c.DefaultModel)
		}
	}
	b := c.Budgets
	if b.PerRun.MaxUSD <= 0 || b.PerModel.MaxUSD <= 0 || b.PerTask.MaxUSD <= 0 {
		return invalid("budgets: per_run, per_model and per_task max_usd must be positive")
	}
	if b.PerModel.MaxUSD > b.PerRun.MaxUSD || b.PerTask.MaxUSD > b.PerModel.MaxUSD {
		return invalid("budgets: expected per_task <= per_model <= per_run")
	}
	if b.PerTask.MaxLatencyMS <= 0 {
		return invalid("budgets: per_task.max_latency_ms must be positive")
	}
	if c.MaxFailovers < -1 {
		return invalid("max_failovers must be >= -1")
	}
	if c.Agent.MaxTurns < 0 || c.Agent.MaxTokens < 0 {
		return invalid("agent: max_turns and max_tokens must not be negative")
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// RouterConfig converts the config into the router's shape.
func (c *Config) RouterConfig(runID string) router.Config {
	routes := make(map[string][]model.ModelRef, len(c.Routes))
	for kind, refs := range c.Routes {
		for _, r := range refs {
			routes[kind] = append(routes[kind], model.ModelRef{Provider: r.Provider, Model: r.Model, Agent: kind})
		}
	}
	latency := map[string]int64{}
	for _, m := range c.Models {
		if m.MaxLatencyMS > 0 {
			latency[m.Model] = m.MaxLatencyMS
		}
	}
	costs := make(provider.Costs, len(c.CostPerMTok))
	for k, v := range c.CostPerMTok {
		costs[k] = v
	}
	return router.Config{
		Routes: routes,
		Budgets: router.Budgets{
			PerRun:   c.Budgets.PerRun.toModel(),
			PerModel: c.Budgets.PerModel.toModel(),
			PerTask:  c.Budgets.PerTask.toModel(),
		},
		Costs:          costs,
		ModelLatencyMS: latency,
		MaxFailovers:   c.MaxFailovers,
		RunID:          runID,
	}
}

// ModelRefs lists every provider/model pair referenced by routes, sorted.
func (c *Config) ModelRefs() []model.ModelRef {
	seen := map[string]model.ModelRef{}
	for _, refs := range c.Routes {
		for _, r := range refs {
			seen[r.Provider+"/"+r.Model] = model.ModelRef{Provider: r.Provider, Model: r.Model}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]model.ModelRef, 0, len(keys))
	for _, k := range keys {
		out = append(out, seen[k])
	}
	return out
}

// BuildRegistry constructs providers. overrides replaces any named provider
// (tests and CI inject scripted fakes); a "fake" type without an override
// gets a benign script that reports nothing. Environment variables are
// read through getenv (nil means os.Getenv).
func (c *Config) BuildRegistry(overrides map[string]model.Provider, getenv func(string) string) (*provider.Registry, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	reg := provider.NewRegistry()
	for name, pc := range c.Providers {
		if p, ok := overrides[name]; ok {
			reg.Register(name, p)
			continue
		}
		timeout := time.Duration(pc.TimeoutMS) * time.Millisecond
		switch pc.Type {
		case "anthropic":
			key := ""
			if pc.APIKeyEnv != "" {
				key = getenv(pc.APIKeyEnv)
				if key == "" {
					return nil, fmt.Errorf("config: provider %q: environment variable %s is empty", name, pc.APIKeyEnv)
				}
			}
			reg.Register(name, anthropicprov.New(anthropicprov.Config{Name: name, APIKey: key, BaseURL: pc.BaseURL, Timeout: timeout}))
		case "openaicompat":
			key := ""
			if pc.APIKeyEnv != "" {
				key = getenv(pc.APIKeyEnv)
				if key == "" {
					return nil, fmt.Errorf("config: provider %q: environment variable %s is empty", name, pc.APIKeyEnv)
				}
			}
			reg.Register(name, openaicompat.New(openaicompat.Config{Name: name, BaseURL: pc.BaseURL, APIKey: key, Timeout: timeout}))
		case "fake":
			reg.Register(name, fake.New([]fake.Step{fake.Text("No findings.")}, fake.Options{Name: name}))
		default:
			return nil, fmt.Errorf("config: provider %q: unknown type %q", name, pc.Type)
		}
	}
	return reg, nil
}
