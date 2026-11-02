package policy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// GraphDOT renders the policy as a Graphviz DOT digraph. Agents are boxes,
// tools are ellipses, rules are edges colored green (allow) or red (deny).
// Dangerous combinations are highlighted with dashed orange edges.
func GraphDOT(p model.Policy) string {
	var b strings.Builder
	b.WriteString("digraph policy {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [fontname=\"Helvetica\"];\n\n")

	agents := sortedAgents(p)
	tools := sortedTools(p)

	for _, name := range agents {
		b.WriteString(fmt.Sprintf("  %q [shape=box];\n", "agent:"+name))
	}
	b.WriteString("\n")
	for _, name := range tools {
		b.WriteString(fmt.Sprintf("  %q [shape=ellipse];\n", "tool:"+name))
	}
	b.WriteString("\n")

	for _, r := range p.Rules {
		color := "green"
		if r.Effect == model.EffectDeny {
			color = "red"
		}
		ruleAgents := r.Match.Agents
		if len(ruleAgents) == 0 {
			ruleAgents = agents
		}
		ruleTools := r.Match.Tools
		if len(ruleTools) == 0 {
			ruleTools = tools
		}
		for _, a := range ruleAgents {
			for _, t := range ruleTools {
				label := string(r.Effect)
				if r.ID != "" {
					label = r.ID
				}
				b.WriteString(fmt.Sprintf("  %q -> %q [label=%q, color=%s];\n",
					"agent:"+a, "tool:"+t, label, color))
			}
		}
	}

	for _, combo := range p.DangerousCombos {
		if len(combo) >= 2 {
			for i := 0; i < len(combo)-1; i++ {
				b.WriteString(fmt.Sprintf("  %q -> %q [style=dashed, color=orange, label=\"dangerous\"];\n",
					"tool:"+combo[i], "tool:"+combo[i+1]))
			}
		}
	}

	b.WriteString("}\n")
	return b.String()
}

// GraphMermaid renders the policy as a Mermaid flowchart. Agents are rectangles,
// tools are rounded rectangles, rules are edges labeled with allow/deny.
func GraphMermaid(p model.Policy) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	agents := sortedAgents(p)
	tools := sortedTools(p)

	for _, name := range agents {
		id := mermaidID("agent", name)
		b.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", id, name))
	}
	for _, name := range tools {
		id := mermaidID("tool", name)
		b.WriteString(fmt.Sprintf("  %s(\"%s\")\n", id, name))
	}

	for _, r := range p.Rules {
		style := "allow"
		if r.Effect == model.EffectDeny {
			style = "deny"
		}
		ruleAgents := r.Match.Agents
		if len(ruleAgents) == 0 {
			ruleAgents = agents
		}
		ruleTools := r.Match.Tools
		if len(ruleTools) == 0 {
			ruleTools = tools
		}
		label := string(r.Effect)
		if r.ID != "" {
			label = r.ID
		}
		for _, a := range ruleAgents {
			for _, t := range ruleTools {
				aID := mermaidID("agent", a)
				tID := mermaidID("tool", t)
				_ = style
				b.WriteString(fmt.Sprintf("  %s -->|%s| %s\n", aID, label, tID))
			}
		}
	}

	for _, combo := range p.DangerousCombos {
		if len(combo) >= 2 {
			for i := 0; i < len(combo)-1; i++ {
				aID := mermaidID("tool", combo[i])
				bID := mermaidID("tool", combo[i+1])
				b.WriteString(fmt.Sprintf("  %s -.->|dangerous| %s\n", aID, bID))
			}
		}
	}

	return b.String()
}

func sortedAgents(p model.Policy) []string {
	agents := make([]string, 0, len(p.Agents))
	for name := range p.Agents {
		agents = append(agents, name)
	}
	sort.Strings(agents)
	return agents
}

func sortedTools(p model.Policy) []string {
	toolSet := map[string]bool{}
	for _, name := range sortedAgents(p) {
		for _, t := range p.Agents[name].AllowedTools {
			toolSet[t] = true
		}
	}
	for _, r := range p.Rules {
		for _, t := range r.Match.Tools {
			toolSet[t] = true
		}
	}
	tools := make([]string, 0, len(toolSet))
	for t := range toolSet {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	return tools
}

func mermaidID(prefix, name string) string {
	safe := strings.NewReplacer(
		"-", "_",
		".", "_",
		" ", "_",
	).Replace(name)
	return prefix + "_" + safe
}
