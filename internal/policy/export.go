package policy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// ExportMarkdown renders a Policy as a human-readable Markdown document.
func ExportMarkdown(p model.Policy) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Policy: %s\n\n", p.Version)
	fmt.Fprintf(&b, "**Default effect:** `%s`\n\n", p.DefaultEffect)

	if len(p.Agents) > 0 {
		b.WriteString("## Agent Boundaries\n\n")
		b.WriteString("| Agent | Max Tier | Allowed Tools | Max Depth | Budget (USD) | Max Latency | Max Tokens |\n")
		b.WriteString("|-------|----------|---------------|-----------|--------------|-------------|------------|\n")

		names := make([]string, 0, len(p.Agents))
		for name := range p.Agents {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			ab := p.Agents[name]
			tools := strings.Join(ab.AllowedTools, ", ")
			if tools == "" {
				tools = "*"
			}
			tokens := "-"
			if ab.Budget.MaxTokens > 0 {
				tokens = fmt.Sprintf("%d", ab.Budget.MaxTokens)
			}
			latency := "-"
			if ab.Budget.MaxLatencyMS > 0 {
				latency = fmt.Sprintf("%dms", ab.Budget.MaxLatencyMS)
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %d | %.4f | %s | %s |\n",
				name, ab.MaxTier, tools, ab.MaxDelegationDepth, ab.Budget.MaxUSD, latency, tokens)
		}
		b.WriteString("\n")
	}

	if len(p.Rules) > 0 {
		b.WriteString("## Rules\n\n")
		for _, r := range p.Rules {
			fmt.Fprintf(&b, "### `%s` (%s)\n\n", r.ID, r.Effect)
			if r.Rationale != "" {
				fmt.Fprintf(&b, "%s\n\n", r.Rationale)
			}
			b.WriteString("**Match:**\n")
			if len(r.Match.Tools) > 0 {
				fmt.Fprintf(&b, "- Tools: `%s`\n", strings.Join(r.Match.Tools, "`, `"))
			}
			if len(r.Match.Agents) > 0 {
				fmt.Fprintf(&b, "- Agents: `%s`\n", strings.Join(r.Match.Agents, "`, `"))
			}
			if len(r.Match.Labs) > 0 {
				fmt.Fprintf(&b, "- Labs: `%s`\n", strings.Join(r.Match.Labs, "`, `"))
			}
			if len(r.Match.Capabilities) > 0 {
				fmt.Fprintf(&b, "- Capabilities: `%s`\n", strings.Join(r.Match.Capabilities, "`, `"))
			}
			if r.Match.MaxTier != "" {
				fmt.Fprintf(&b, "- Max tier: `%s`\n", r.Match.MaxTier)
			}
			b.WriteString("\n")
		}
	}

	if len(p.DangerousCombos) > 0 {
		b.WriteString("## Dangerous Combinations\n\n")
		for _, combo := range p.DangerousCombos {
			fmt.Fprintf(&b, "- %s\n", strings.Join(combo, " + "))
		}
		b.WriteString("\n")
	}

	if len(p.Weights) > 0 {
		b.WriteString("## Weights\n\n")
		b.WriteString("| Key | Weight |\n")
		b.WriteString("|-----|--------|\n")
		keys := make([]string, 0, len(p.Weights))
		for k := range p.Weights {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "| %s | %.4f |\n", k, p.Weights[k])
		}
		b.WriteString("\n")
	}

	return b.String()
}
