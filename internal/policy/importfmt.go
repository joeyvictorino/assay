package policy

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// ImportJSON parses a JSON policy document into model.Policy.
func ImportJSON(data []byte) (model.Policy, error) {
	var p model.Policy
	if err := json.Unmarshal(data, &p); err != nil {
		return model.Policy{}, fmt.Errorf("import json: %w", err)
	}
	if p.Version == "" {
		return model.Policy{}, fmt.Errorf("import json: version is required")
	}
	if p.DefaultEffect == "" {
		return model.Policy{}, fmt.Errorf("import json: default_effect is required")
	}
	return p, nil
}

// ImportCSV parses a CSV rules table into a Policy. The CSV must have a
// header row with at least: effect, agent, tool, capabilities.
// Optional columns: id, rationale.
// Each row becomes a Rule; agents are collected into agent boundaries with
// sensible defaults.
func ImportCSV(data []byte) (model.Policy, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	records, err := r.ReadAll()
	if err != nil {
		return model.Policy{}, fmt.Errorf("import csv: %w", err)
	}
	if len(records) < 2 {
		return model.Policy{}, fmt.Errorf("import csv: need header and at least one data row")
	}

	header := records[0]
	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	required := []string{"effect", "agent", "tool"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return model.Policy{}, fmt.Errorf("import csv: missing required column %q", col)
		}
	}

	p := model.Policy{
		Version:       "1.0",
		DefaultEffect: model.EffectDeny,
		Agents:        map[string]model.AgentBoundary{},
	}

	agentTools := map[string]map[string]bool{}

	for i, row := range records[1:] {
		lineNum := i + 2
		if len(row) < len(header) {
			return model.Policy{}, fmt.Errorf("import csv: row %d: expected %d columns, got %d", lineNum, len(header), len(row))
		}

		effect := model.Effect(strings.TrimSpace(strings.ToLower(row[colIdx["effect"]])))
		if effect != model.EffectAllow && effect != model.EffectDeny {
			return model.Policy{}, fmt.Errorf("import csv: row %d: effect must be allow or deny, got %q", lineNum, effect)
		}

		agent := strings.TrimSpace(row[colIdx["agent"]])
		tool := strings.TrimSpace(row[colIdx["tool"]])

		var caps []string
		if idx, ok := colIdx["capabilities"]; ok && strings.TrimSpace(row[idx]) != "" {
			for _, c := range strings.Split(row[idx], ";") {
				if c = strings.TrimSpace(c); c != "" {
					caps = append(caps, c)
				}
			}
		}

		id := fmt.Sprintf("imported-%d", i+1)
		if idx, ok := colIdx["id"]; ok && strings.TrimSpace(row[idx]) != "" {
			id = strings.TrimSpace(row[idx])
		}

		rationale := ""
		if idx, ok := colIdx["rationale"]; ok {
			rationale = strings.TrimSpace(row[idx])
		}

		rule := model.Rule{
			ID:        id,
			Effect:    effect,
			Rationale: rationale,
			Match: model.RuleMatch{
				Agents:       []string{agent},
				Tools:        []string{tool},
				Capabilities: caps,
			},
		}
		p.Rules = append(p.Rules, rule)

		if _, ok := agentTools[agent]; !ok {
			agentTools[agent] = map[string]bool{}
		}
		agentTools[agent][tool] = true
	}

	for agent, tools := range agentTools {
		toolList := make([]string, 0, len(tools))
		for t := range tools {
			toolList = append(toolList, t)
		}
		p.Agents[agent] = model.AgentBoundary{
			MaxTier:      model.TierMedium,
			AllowedTools: toolList,
			Budget: model.Budget{
				MaxUSD:       1.0,
				MaxLatencyMS: 30000,
			},
		}
	}

	return p, nil
}
