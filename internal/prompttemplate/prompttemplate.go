package prompttemplate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Template struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Pattern     string   `json:"pattern"`
	Params      []string `json:"params,omitempty"`
	Description string   `json:"description"`
}

type Library struct {
	Templates []Template `json:"templates"`
}

type RenderResult struct {
	Template      string   `json:"template"`
	Rendered      string   `json:"rendered"`
	MissingParams []string `json:"missing_params,omitempty"`
}

type CategorySummary struct {
	Category string     `json:"category"`
	Count    int        `json:"count"`
	Names    []string   `json:"names"`
}

type Summary struct {
	TotalTemplates int               `json:"total_templates"`
	Categories     []CategorySummary `json:"categories"`
	TotalParams    int               `json:"total_params"`
}

type Report struct {
	Library *Library `json:"library"`
	Summary Summary  `json:"summary"`
}

func NewLibrary() *Library {
	return &Library{}
}

func Parse(content string) (*Library, error) {
	lib := NewLibrary()
	lines := strings.Split(content, "\n")
	var current *Template

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "---") {
			if current != nil {
				lib.Templates = append(lib.Templates, *current)
			}
			current = &Template{}
			continue
		}

		if current == nil {
			current = &Template{}
		}

		if strings.HasPrefix(trimmed, "name:") {
			current.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
		} else if strings.HasPrefix(trimmed, "category:") {
			current.Category = strings.TrimSpace(strings.TrimPrefix(trimmed, "category:"))
		} else if strings.HasPrefix(trimmed, "description:") {
			current.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
		} else if strings.HasPrefix(trimmed, "params:") {
			paramStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "params:"))
			for _, p := range strings.Split(paramStr, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					current.Params = append(current.Params, p)
				}
			}
		} else if strings.HasPrefix(trimmed, "pattern:") {
			current.Pattern = strings.TrimSpace(strings.TrimPrefix(trimmed, "pattern:"))
		}
	}

	if current != nil && current.Name != "" {
		lib.Templates = append(lib.Templates, *current)
	}

	return lib, nil
}

func Render(tmpl Template, params map[string]string) RenderResult {
	result := RenderResult{Template: tmpl.Name}
	rendered := tmpl.Pattern

	for _, p := range tmpl.Params {
		placeholder := "{{" + p + "}}"
		if val, ok := params[p]; ok {
			rendered = strings.ReplaceAll(rendered, placeholder, val)
		} else {
			result.MissingParams = append(result.MissingParams, p)
		}
	}

	result.Rendered = rendered
	return result
}

func Categorize(lib *Library) map[string][]Template {
	cats := make(map[string][]Template)
	for _, t := range lib.Templates {
		cat := t.Category
		if cat == "" {
			cat = "uncategorized"
		}
		cats[cat] = append(cats[cat], t)
	}
	return cats
}

func Analyze(lib *Library) Report {
	cats := Categorize(lib)
	var catSummaries []CategorySummary
	totalParams := 0

	keys := make([]string, 0, len(cats))
	for k := range cats {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, cat := range keys {
		tmpls := cats[cat]
		names := make([]string, len(tmpls))
		for i, t := range tmpls {
			names[i] = t.Name
			totalParams += len(t.Params)
		}
		catSummaries = append(catSummaries, CategorySummary{
			Category: cat,
			Count:    len(tmpls),
			Names:    names,
		})
	}

	return Report{
		Library: lib,
		Summary: Summary{
			TotalTemplates: len(lib.Templates),
			Categories:     catSummaries,
			TotalParams:    totalParams,
		},
	}
}

func FormatText(r Report) string {
	var sb strings.Builder
	sb.WriteString("Prompt Template Library\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&sb, "  Templates:   %d\n", r.Summary.TotalTemplates)
	fmt.Fprintf(&sb, "  Categories:  %d\n", len(r.Summary.Categories))
	fmt.Fprintf(&sb, "  Parameters:  %d\n\n", r.Summary.TotalParams)

	for _, cat := range r.Summary.Categories {
		fmt.Fprintf(&sb, "  [%s] (%d templates)\n", cat.Category, cat.Count)
		for _, name := range cat.Names {
			fmt.Fprintf(&sb, "    - %s\n", name)
		}
	}

	if r.Library != nil {
		sb.WriteString("\nTemplates:\n")
		for _, t := range r.Library.Templates {
			fmt.Fprintf(&sb, "  %s [%s]\n", t.Name, t.Category)
			if t.Description != "" {
				fmt.Fprintf(&sb, "    %s\n", t.Description)
			}
			if len(t.Params) > 0 {
				fmt.Fprintf(&sb, "    Params: %s\n", strings.Join(t.Params, ", "))
			}
			fmt.Fprintf(&sb, "    Pattern: %s\n", t.Pattern)
		}
	}

	return sb.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
