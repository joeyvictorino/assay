package ensemble

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Strategy string

const (
	StratMajority  Strategy = "majority"
	StratUnanimous Strategy = "unanimous"
	StratWeighted  Strategy = "weighted"
)

type ConsensusFinding struct {
	DedupKey     string         `json:"dedup_key"`
	Class        model.Class    `json:"class"`
	Location     model.Location `json:"location"`
	Severity     model.Severity `json:"severity"`
	Summary      string         `json:"summary"`
	Agreement    float64        `json:"agreement"`
	VoterCount   int            `json:"voter_count"`
	TotalVoters  int            `json:"total_voters"`
	Models       []string       `json:"models"`
}

type Report struct {
	Strategy      Strategy           `json:"strategy"`
	MinAgreement  float64            `json:"min_agreement"`
	RunCount      int                `json:"run_count"`
	TotalUnique   int                `json:"total_unique_findings"`
	Consensus     []ConsensusFinding `json:"consensus_findings"`
	Rejected      int                `json:"rejected_findings"`
	AgreementDist map[string]int     `json:"agreement_distribution"`
}

type runData struct {
	RunID    string          `json:"run_id"`
	Model    string          `json:"model,omitempty"`
	Findings []model.Finding `json:"findings"`
}

func LoadRun(path string) (runData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runData{}, err
	}
	var rd runData
	if err := json.Unmarshal(data, &rd); err != nil {
		var fs []model.Finding
		if err2 := json.Unmarshal(data, &fs); err2 == nil {
			rd.Findings = fs
			if len(fs) > 0 {
				rd.Model = fs[0].Model.Model
				rd.RunID = fs[0].RunID
			}
			return rd, nil
		}
		var wrapped struct {
			Findings []model.Finding `json:"findings"`
			RunID    string          `json:"run_id"`
		}
		if err3 := json.Unmarshal(data, &wrapped); err3 == nil && wrapped.Findings != nil {
			rd.RunID = wrapped.RunID
			rd.Findings = wrapped.Findings
			if len(wrapped.Findings) > 0 {
				rd.Model = wrapped.Findings[0].Model.Model
			}
			return rd, nil
		}
		return runData{}, fmt.Errorf("%s: %w", path, err)
	}
	return rd, nil
}

func Analyze(paths []string, strategy Strategy, minAgreement float64) (Report, error) {
	var runs []runData
	for _, p := range paths {
		rd, err := LoadRun(p)
		if err != nil {
			return Report{}, err
		}
		runs = append(runs, rd)
	}
	return AnalyzeRuns(runs, strategy, minAgreement), nil
}

type vote struct {
	finding model.Finding
	model   string
}

func AnalyzeRuns(runs []runData, strategy Strategy, minAgreement float64) Report {
	buckets := make(map[string][]vote)
	for _, rd := range runs {
		modelName := rd.Model
		if modelName == "" {
			modelName = rd.RunID
		}
		for _, f := range rd.Findings {
			key := f.DedupKey
			if key == "" {
				key = string(f.Class) + "|" + f.Location.Method + "|" + f.Location.PathTemplate
			}
			buckets[key] = append(buckets[key], vote{finding: f, model: modelName})
		}
	}

	totalRuns := len(runs)
	var consensus []ConsensusFinding
	rejected := 0
	agreeDist := map[string]int{}

	for key, votes := range buckets {
		agreement := float64(len(votes)) / float64(totalRuns)
		bucket := agreementBucket(agreement)
		agreeDist[bucket]++

		pass := false
		switch strategy {
		case StratUnanimous:
			pass = len(votes) == totalRuns
		case StratWeighted:
			pass = agreement >= minAgreement
		default:
			pass = agreement > 0.5
		}

		if !pass || agreement < minAgreement {
			rejected++
			continue
		}

		models := make(map[string]bool)
		for _, v := range votes {
			models[v.model] = true
		}
		modelList := make([]string, 0, len(models))
		for m := range models {
			modelList = append(modelList, m)
		}
		sort.Strings(modelList)

		best := pickBestFinding(votes)
		consensus = append(consensus, ConsensusFinding{
			DedupKey:    key,
			Class:       best.Class,
			Location:    best.Location,
			Severity:    best.Severity,
			Summary:     best.Summary,
			Agreement:   agreement,
			VoterCount:  len(votes),
			TotalVoters: totalRuns,
			Models:      modelList,
		})
	}

	sort.Slice(consensus, func(i, j int) bool {
		if consensus[i].Agreement != consensus[j].Agreement {
			return consensus[i].Agreement > consensus[j].Agreement
		}
		return consensus[i].Severity.Rank() > consensus[j].Severity.Rank()
	})

	return Report{
		Strategy:      strategy,
		MinAgreement:  minAgreement,
		RunCount:      totalRuns,
		TotalUnique:   len(buckets),
		Consensus:     consensus,
		Rejected:      rejected,
		AgreementDist: agreeDist,
	}
}

func pickBestFinding(votes []vote) model.Finding {
	best := votes[0].finding
	for _, v := range votes[1:] {
		if v.finding.Severity.Rank() > best.Severity.Rank() {
			best = v.finding
		}
	}
	return best
}

func agreementBucket(a float64) string {
	switch {
	case a >= 1.0:
		return "100%"
	case a >= 0.75:
		return "75-99%"
	case a >= 0.50:
		return "50-74%"
	case a >= 0.25:
		return "25-49%"
	default:
		return "0-24%"
	}
}

func FormatText(r Report) string {
	var sb strings.Builder
	sb.WriteString("Model Ensemble Consensus Report\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")

	fmt.Fprintf(&sb, "  Strategy:       %s\n", r.Strategy)
	fmt.Fprintf(&sb, "  Min Agreement:  %.0f%%\n", r.MinAgreement*100)
	fmt.Fprintf(&sb, "  Runs Analyzed:  %d\n", r.RunCount)
	fmt.Fprintf(&sb, "  Unique Findings: %d\n", r.TotalUnique)
	fmt.Fprintf(&sb, "  Consensus:      %d\n", len(r.Consensus))
	fmt.Fprintf(&sb, "  Rejected:       %d\n\n", r.Rejected)

	if len(r.AgreementDist) > 0 {
		sb.WriteString("Agreement Distribution:\n")
		keys := []string{"100%", "75-99%", "50-74%", "25-49%", "0-24%"}
		for _, k := range keys {
			if v, ok := r.AgreementDist[k]; ok {
				fmt.Fprintf(&sb, "  %-8s %d findings\n", k, v)
			}
		}
		sb.WriteString("\n")
	}

	if len(r.Consensus) > 0 {
		sb.WriteString("Consensus Findings:\n")
		for i, f := range r.Consensus {
			fmt.Fprintf(&sb, "  %d. [%s] %s (%.0f%% agreement, %d/%d models)\n",
				i+1, f.Severity, f.Summary, f.Agreement*100, f.VoterCount, f.TotalVoters)
			fmt.Fprintf(&sb, "     Class: %s  Location: %s %s\n", f.Class, f.Location.Method, f.Location.PathTemplate)
			fmt.Fprintf(&sb, "     Models: %s\n", strings.Join(f.Models, ", "))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
