package provider

import (
	"errors"
	"fmt"

	"github.com/joeyvictorino/assay/internal/model"
)

// ErrMissingCost is returned when a model has no cost table entry.
var ErrMissingCost = errors.New("provider: missing cost entry")

// Cost is the price of one model per million tokens, in USD. Values come
// from runs/*.yaml; nothing is hard-coded.
type Cost struct {
	InputUSD     float64 `json:"input_usd" yaml:"input_usd"`
	OutputUSD    float64 `json:"output_usd" yaml:"output_usd"`
	CacheReadUSD float64 `json:"cache_read_usd" yaml:"cache_read_usd"`
}

// Costs maps a model id to its per-MTok price.
type Costs map[string]Cost

// Lookup returns the cost entry for model or ErrMissingCost.
func (c Costs) Lookup(modelID string) (Cost, error) {
	cost, ok := c[modelID]
	if !ok {
		return Cost{}, fmt.Errorf("%w: %q", ErrMissingCost, modelID)
	}
	return cost, nil
}

const mtok = 1_000_000.0

// ComputeCost prices one call. Cache-read tokens are billed at the cache
// read rate; cache-write tokens are billed at the input rate (the provider
// premium, if any, is folded into the configured input rate). Input tokens
// reported by providers exclude cached tokens, so the three are summed.
func ComputeCost(u model.Usage, c Cost) float64 {
	in := float64(u.InputTokens+u.CacheWriteTokens) * c.InputUSD / mtok
	out := float64(u.OutputTokens) * c.OutputUSD / mtok
	cache := float64(u.CacheReadTokens) * c.CacheReadUSD / mtok
	return in + out + cache
}

// MicroUSD converts a USD amount to integer micro-dollars for audit metadata.
func MicroUSD(usd float64) int64 {
	return int64(usd*1_000_000 + 0.5)
}
