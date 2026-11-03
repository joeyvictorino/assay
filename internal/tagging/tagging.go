package tagging

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
	"gopkg.in/yaml.v3"
)

type TagRule struct {
	Tag      string   `yaml:"tag" json:"tag"`
	Classes  []string `yaml:"classes,omitempty" json:"classes,omitempty"`
	Keywords []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
}

type TagRulesFile struct {
	Rules []TagRule `yaml:"rules" json:"rules"`
}

type TaggedFinding struct {
	ID       string         `json:"id"`
	Class    model.Class    `json:"class"`
	Severity model.Severity `json:"severity"`
	Summary  string         `json:"summary"`
	Model    model.ModelRef `json:"model"`
	Lab      string         `json:"lab"`
	Tags     []string       `json:"tags"`
}

type Report struct {
	TotalFindings int               `json:"total_findings"`
	TotalTagged   int               `json:"total_tagged"`
	TotalUntagged int               `json:"total_untagged"`
	TagCounts     map[string]int    `json:"tag_counts"`
	Findings      []TaggedFinding   `json:"findings"`
}

func LoadRules(path string) ([]TagRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading tag rules: %w", err)
	}
	var rf TagRulesFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parsing tag rules YAML: %w", err)
	}
	return rf.Rules, nil
}

func matchRule(f model.Finding, rule TagRule) bool {
	classStr := strings.ToLower(string(f.Class))
	for _, c := range rule.Classes {
		if strings.ToLower(c) == classStr || strings.Contains(classStr, strings.ToLower(c)) {
			return true
		}
	}
	summaryLower := strings.ToLower(f.Summary)
	for _, kw := range rule.Keywords {
		if strings.Contains(summaryLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func ApplyTags(findings []model.Finding, rules []TagRule) []TaggedFinding {
	var result []TaggedFinding
	for _, f := range findings {
		seen := map[string]bool{}
		var tags []string
		for _, rule := range rules {
			if matchRule(f, rule) && !seen[rule.Tag] {
				tags = append(tags, rule.Tag)
				seen[rule.Tag] = true
			}
		}
		sort.Strings(tags)
		result = append(result, TaggedFinding{
			ID:       f.ID,
			Class:    f.Class,
			Severity: f.Severity,
			Summary:  f.Summary,
			Model:    f.Model,
			Lab:      f.Lab,
			Tags:     tags,
		})
	}
	return result
}

func FilterByTag(findings []TaggedFinding, tag string) []TaggedFinding {
	var filtered []TaggedFinding
	for _, f := range findings {
		for _, t := range f.Tags {
			if strings.EqualFold(t, tag) {
				filtered = append(filtered, f)
				break
			}
		}
	}
	return filtered
}

func Analyze(findings []model.Finding, rules []TagRule) Report {
	tagged := ApplyTags(findings, rules)
	tagCounts := map[string]int{}
	totalTagged := 0
	for _, f := range tagged {
		if len(f.Tags) > 0 {
			totalTagged++
		}
		for _, t := range f.Tags {
			tagCounts[t]++
		}
	}
	return Report{
		TotalFindings: len(findings),
		TotalTagged:   totalTagged,
		TotalUntagged: len(findings) - totalTagged,
		TagCounts:     tagCounts,
		Findings:      tagged,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Findings Tagging Report\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&b, "  Total findings: %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "  Tagged:         %d\n", r.TotalTagged)
	fmt.Fprintf(&b, "  Untagged:       %d\n\n", r.TotalUntagged)

	if len(r.TagCounts) > 0 {
		b.WriteString("  Tag distribution:\n")
		tags := make([]string, 0, len(r.TagCounts))
		for t := range r.TagCounts {
			tags = append(tags, t)
		}
		sort.Strings(tags)
		for _, t := range tags {
			fmt.Fprintf(&b, "    %-20s %d\n", t, r.TagCounts[t])
		}
		b.WriteString("\n")
	}

	for _, f := range r.Findings {
		tagStr := "(none)"
		if len(f.Tags) > 0 {
			tagStr = strings.Join(f.Tags, ", ")
		}
		fmt.Fprintf(&b, "  %s [%s] tags=[%s] %s\n", f.ID, f.Severity, tagStr, f.Summary)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
