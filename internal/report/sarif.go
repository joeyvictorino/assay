package report

import (
	"encoding/json"
	"fmt"

	"github.com/joeyvictorino/assay/internal/model"
)

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine,omitempty"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifHelp struct {
	Text string `json:"text"`
}

type sarifShortDesc struct {
	Text string `json:"text"`
}

type sarifRule struct {
	ID               string         `json:"id"`
	ShortDescription sarifShortDesc `json:"shortDescription"`
	Help             sarifHelp      `json:"help"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

// FindingsToSARIF converts assay findings into a SARIF v2.1.0 log suitable
// for upload to GitHub Code Scanning via github/codeql-action/upload-sarif.
// Only validated and theorized findings are included; refuted and declined
// findings are omitted.
func FindingsToSARIF(findings []model.Finding, toolVersion string) (string, error) {
	ruleSet := map[string]bool{}
	var rules []sarifRule
	var results []sarifResult

	for _, f := range findings {
		if f.State == model.StateRefuted || f.State == model.StateDeclined {
			continue
		}

		ruleID := sarifRuleID(f)
		if !ruleSet[ruleID] {
			ruleSet[ruleID] = true
			rules = append(rules, sarifRule{
				ID:               ruleID,
				ShortDescription: sarifShortDesc{Text: sarifRuleTitle(f)},
				Help:             sarifHelp{Text: sarifRuleHelp(f)},
			})
		}

		uri := sarifURI(f)
		loc := sarifLocation{
			PhysicalLocation: sarifPhysicalLocation{
				ArtifactLocation: sarifArtifactLocation{URI: uri},
			},
		}

		results = append(results, sarifResult{
			RuleID:    ruleID,
			Level:     severityToSARIF(f.Severity),
			Message:   sarifMessage{Text: sarifMessage_(f)},
			Locations: []sarifLocation{loc},
		})
	}

	if toolVersion == "" {
		toolVersion = "dev"
	}

	log := sarifLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "assay",
					Version:        toolVersion,
					InformationURI: "https://github.com/joeyvictorino/assay",
					Rules:          rules,
				},
			},
			Results: results,
		}},
	}

	b, err := json.MarshalIndent(log, "", "  ")
	return string(b), err
}

func severityToSARIF(s model.Severity) string {
	switch s {
	case model.SevCritical, model.SevHigh:
		return "error"
	case model.SevMedium:
		return "warning"
	case model.SevLow, model.SevInfo:
		return "note"
	default:
		return "note"
	}
}

func sarifRuleID(f model.Finding) string {
	return fmt.Sprintf("assay/%s", f.Class)
}

func sarifRuleTitle(f model.Finding) string {
	return fmt.Sprintf("%s finding (%s)", f.Class, f.State)
}

func sarifRuleHelp(f model.Finding) string {
	cwe := ""
	if f.CWE != "" {
		cwe = fmt.Sprintf(" (CWE: %s)", f.CWE)
	}
	return fmt.Sprintf("Vulnerability class: %s%s. State: %s. Detected by model %s via assay security validation harness.",
		f.Class, cwe, f.State, f.Model.Model)
}

func sarifURI(f model.Finding) string {
	uri := f.Location.PathTemplate
	if uri == "" {
		uri = f.Lab
	}
	return uri
}

func sarifMessage_(f model.Finding) string {
	msg := f.Summary
	if msg == "" {
		msg = fmt.Sprintf("%s %s at %s %s", f.State, f.Class, f.Location.Method, f.Location.PathTemplate)
	}
	if f.Location.Param != "" {
		msg += fmt.Sprintf(" (param: %s)", f.Location.Param)
	}
	return msg
}
