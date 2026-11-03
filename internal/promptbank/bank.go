package promptbank

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Variable struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
}

type Template struct {
	Name               string     `json:"name" yaml:"name"`
	Description        string     `json:"description" yaml:"description"`
	Category           string     `json:"category" yaml:"category"`
	SystemPrompt       string     `json:"system_prompt" yaml:"system_prompt"`
	UserPromptTemplate string     `json:"user_prompt_template" yaml:"user_prompt_template"`
	Variables          []Variable `json:"variables,omitempty" yaml:"variables,omitempty"`
}

type Bank struct {
	dir string
}

func New(dir string) *Bank {
	return &Bank{dir: dir}
}

func (b *Bank) Dir() string { return b.dir }

func (b *Bank) List() ([]Template, error) {
	entries, err := os.ReadDir(b.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var templates []Template
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		t, err := b.load(e.Name())
		if err != nil {
			continue
		}
		templates = append(templates, t)
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})
	return templates, nil
}

func (b *Bank) Get(name string) (Template, error) {
	return b.load(name + ".json")
}

func (b *Bank) Add(t Template) error {
	if t.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if err := os.MkdirAll(b.dir, 0o755); err != nil {
		return fmt.Errorf("create prompt bank directory: %w", err)
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(b.dir, t.Name+".json")
	return os.WriteFile(p, append(data, '\n'), 0o644)
}

func (b *Bank) Export(format string) (string, error) {
	templates, err := b.List()
	if err != nil {
		return "", err
	}
	switch format {
	case "json":
		data, err := json.MarshalIndent(templates, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data) + "\n", nil
	case "yaml":
		return formatYAML(templates), nil
	default:
		return "", fmt.Errorf("unknown format %q (json | yaml)", format)
	}
}

func (b *Bank) load(filename string) (Template, error) {
	p := filepath.Join(b.dir, filename)
	data, err := os.ReadFile(p)
	if err != nil {
		return Template{}, fmt.Errorf("load template %s: %w", filename, err)
	}
	var t Template
	if err := json.Unmarshal(data, &t); err != nil {
		return Template{}, fmt.Errorf("parse template %s: %w", filename, err)
	}
	return t, nil
}

func formatYAML(templates []Template) string {
	var sb strings.Builder
	for i, t := range templates {
		if i > 0 {
			sb.WriteString("---\n")
		}
		fmt.Fprintf(&sb, "name: %s\n", yamlQuote(t.Name))
		fmt.Fprintf(&sb, "description: %s\n", yamlQuote(t.Description))
		fmt.Fprintf(&sb, "category: %s\n", yamlQuote(t.Category))
		fmt.Fprintf(&sb, "system_prompt: %s\n", yamlQuote(t.SystemPrompt))
		fmt.Fprintf(&sb, "user_prompt_template: %s\n", yamlQuote(t.UserPromptTemplate))
		if len(t.Variables) > 0 {
			sb.WriteString("variables:\n")
			for _, v := range t.Variables {
				fmt.Fprintf(&sb, "  - name: %s\n", yamlQuote(v.Name))
				fmt.Fprintf(&sb, "    description: %s\n", yamlQuote(v.Description))
				if v.Default != "" {
					fmt.Fprintf(&sb, "    default: %s\n", yamlQuote(v.Default))
				}
			}
		}
	}
	return sb.String()
}

func yamlQuote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":\n\"'{}[]|>&*!%#@`,?") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
