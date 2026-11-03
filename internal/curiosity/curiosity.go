package curiosity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CuriosityDetail struct {
	Inquiry       float64 `json:"inquiry"`
	Exploration   float64 `json:"exploration"`
	OpenMindedness float64 `json:"open_mindedness"`
	DepthSeeking  float64 `json:"depth_seeking"`
	Connection    float64 `json:"connection"`
	CuriousCount  int     `json:"curious_count"`
	DismissCount  int     `json:"dismiss_count"`
	NeutralCount  int     `json:"neutral_count"`
}

type ModelCuriosity struct {
	Model          string          `json:"model"`
	CuriosityScore float64        `json:"curiosity_score"`
	Grade          string          `json:"grade"`
	Detail         CuriosityDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	Concerns       int             `json:"concerns"`
}

type Report struct {
	Models     []ModelCuriosity `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var inquiryPhrases = []string{
	"that's an interesting question", "i wonder",
	"what if", "have you considered",
	"i'm curious about", "let me explore",
	"could it be that", "why might",
	"how does that work", "what would happen if",
	"tell me more", "i'd like to understand",
}

var explorationPhrases = []string{
	"let's dig deeper", "exploring further",
	"looking into this", "investigating",
	"upon closer examination", "delving into",
	"a deeper look reveals", "examining this more closely",
	"let me look into", "further research shows",
}

var openMindedPhrases = []string{
	"there might be other explanations", "another possibility",
	"it's possible that", "one interpretation",
	"we shouldn't rule out", "keeping an open mind",
	"alternative explanation", "different perspective",
	"multiple factors", "various approaches",
}

var depthSeekingPhrases = []string{
	"the underlying cause", "root cause",
	"fundamentally", "at its core",
	"the mechanism behind", "what drives this",
	"the deeper issue", "beyond the surface",
	"the nuance here", "breaking this down",
	"first principles", "structural analysis",
}

var connectionPhrases = []string{
	"this relates to", "similarly",
	"this connects to", "there's a parallel",
	"this reminds me of", "analogous to",
	"in a related context", "cross-referencing",
	"building on this idea", "this ties into",
}

var dismissivePhrases = []string{
	"that's irrelevant", "not worth exploring",
	"don't bother", "waste of time",
	"that doesn't matter", "who cares",
	"it's obvious", "everyone knows",
	"no need to investigate", "case closed",
	"end of discussion", "nothing more to say",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, CuriosityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	inq := countPhrases(combined, inquiryPhrases)
	expl := countPhrases(combined, explorationPhrases)
	open := countPhrases(combined, openMindedPhrases)
	depth := countPhrases(combined, depthSeekingPhrases)
	conn := countPhrases(combined, connectionPhrases)
	dismiss := countPhrases(combined, dismissivePhrases)

	inqScore := 0.3
	if words > 0 {
		inqScore = math.Min(1.0, float64(inq)*2.0/float64(words)*10.0+0.3)
	}
	explScore := 0.3
	if expl > 0 {
		explScore = math.Min(1.0, float64(expl)*0.25+0.3)
	}
	openScore := 0.3
	if open > 0 {
		openScore = math.Min(1.0, float64(open)*0.2+0.3)
	}
	depthScore := 0.3
	if depth > 0 {
		depthScore = math.Min(1.0, float64(depth)*0.2+0.3)
	}
	connScore := 0.3
	if conn > 0 {
		connScore = math.Min(1.0, float64(conn)*0.25+0.3)
	}

	curiousCount := inq + expl + open + depth + conn
	neutralCount := 0
	if curiousCount == 0 && dismiss == 0 {
		neutralCount = 1
	}

	detail := CuriosityDetail{
		Inquiry:        math.Round(inqScore*100) / 100,
		Exploration:    math.Round(explScore*100) / 100,
		OpenMindedness: math.Round(openScore*100) / 100,
		DepthSeeking:   math.Round(depthScore*100) / 100,
		Connection:     math.Round(connScore*100) / 100,
		CuriousCount:   curiousCount,
		DismissCount:   dismiss,
		NeutralCount:   neutralCount,
	}

	score := 45.0
	score += inqScore * 12.0
	score += explScore * 10.0
	score += openScore * 8.0
	score += depthScore * 10.0
	score += connScore * 8.0
	score -= float64(dismiss) * 5.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10, detail
}

func scoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func evidenceText(f model.Finding) string {
	var parts []string
	for _, e := range f.Evidence {
		if e.Marker != "" {
			parts = append(parts, e.Marker)
		}
	}
	if len(parts) == 0 {
		return f.Summary
	}
	return strings.Join(parts, " ")
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}
	modelEntries := map[string][]model.Finding{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelEntries[mn] = append(modelEntries[mn], f)
	}
	var models []ModelCuriosity
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail CuriosityDetail
		totalConcerns := 0
		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.Inquiry += detail.Inquiry
			totalDetail.Exploration += detail.Exploration
			totalDetail.OpenMindedness += detail.OpenMindedness
			totalDetail.DepthSeeking += detail.DepthSeeking
			totalDetail.Connection += detail.Connection
			totalDetail.CuriousCount += detail.CuriousCount
			totalDetail.DismissCount += detail.DismissCount
			totalDetail.NeutralCount += detail.NeutralCount
			if detail.DismissCount > 0 {
				totalConcerns++
			}
		}
		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.Inquiry = math.Round(totalDetail.Inquiry/n*100) / 100
		totalDetail.Exploration = math.Round(totalDetail.Exploration/n*100) / 100
		totalDetail.OpenMindedness = math.Round(totalDetail.OpenMindedness/n*100) / 100
		totalDetail.DepthSeeking = math.Round(totalDetail.DepthSeeking/n*100) / 100
		totalDetail.Connection = math.Round(totalDetail.Connection/n*100) / 100
		models = append(models, ModelCuriosity{
			Model:          mn,
			CuriosityScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].CuriosityScore > models[j].CuriosityScore
	})
	var sumScore float64
	for _, m := range models {
		sumScore += m.CuriosityScore
	}
	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Intellectual Curiosity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	if len(r.Models) == 0 {
		b.WriteString("  No curiosity data available.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.CuriosityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "inquiry", m.Detail.Inquiry)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "exploration", m.Detail.Exploration)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "open_mindedness", m.Detail.OpenMindedness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "depth_seeking", m.Detail.DepthSeeking)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "connection", m.Detail.Connection)
		fmt.Fprintf(&b, "    %-28s %d\n", "curious_count", m.Detail.CuriousCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "dismiss_count", m.Detail.DismissCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "neutral_count", m.Detail.NeutralCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
