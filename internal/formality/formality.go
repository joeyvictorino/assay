package formality

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FormalityDetail struct {
	ProfessionalTone   float64 `json:"professional_tone"`
	TechnicalPrecision float64 `json:"technical_precision"`
	RegisterConsistency float64 `json:"register_consistency"`
	StructuredResponse float64 `json:"structured_response"`
	AcademicRigor      float64 `json:"academic_rigor"`
	FormalCount        int     `json:"formal_count"`
	InformalCount      int     `json:"informal_count"`
	SlangCount         int     `json:"slang_count"`
}

type ModelFormality struct {
	Model          string          `json:"model"`
	FormalityScore float64         `json:"formality_score"`
	Grade          string          `json:"grade"`
	Detail         FormalityDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	Concerns       int             `json:"concerns"`
}

type Report struct {
	Models     []ModelFormality `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var professionalPhrases = []string{
	"recommend", "advise", "suggest",
	"in accordance with", "pursuant to",
	"with respect to", "regarding",
	"please note", "it is important to",
	"as per", "per the documentation",
	"the recommended approach", "best practice",
}

var technicalPhrases = []string{
	"implementation", "architecture", "configuration",
	"parameter", "specification", "protocol",
	"interface", "module", "component",
	"algorithm", "framework", "methodology",
	"deployment", "infrastructure", "pipeline",
}

var structuredPhrases = []string{
	"first", "second", "third",
	"step 1", "step 2", "step 3",
	"the following", "as follows",
	"in conclusion", "to summarize",
	"overview", "prerequisites",
	"requirements", "considerations",
}

var academicPhrases = []string{
	"furthermore", "moreover", "consequently",
	"nevertheless", "notwithstanding",
	"in addition", "accordingly",
	"with regard to", "in light of",
	"it should be noted", "it is worth mentioning",
	"the evidence suggests",
}

var registerPhrases = []string{
	"shall", "hereby", "thereof",
	"whereas", "henceforth", "aforementioned",
	"the undersigned", "duly noted",
	"per se", "vis-a-vis", "ipso facto",
	"prima facie",
}

var informalPhrases = []string{
	"gonna", "wanna", "gotta",
	"kinda", "sorta", "dunno",
	"ain't", "y'all", "nah",
	"yeah", "yep", "nope",
	"cool", "awesome", "super",
}

var slangPhrases = []string{
	"lol", "omg", "bruh",
	"tbh", "imo", "btw",
	"fyi for real", "no cap",
	"lowkey", "highkey", "vibe",
	"slay", "fire", "lit",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, FormalityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	professional := countPhrases(combined, professionalPhrases)
	technical := countPhrases(combined, technicalPhrases)
	structured := countPhrases(combined, structuredPhrases)
	academic := countPhrases(combined, academicPhrases)
	register := countPhrases(combined, registerPhrases)
	informal := countPhrases(combined, informalPhrases)
	slang := countPhrases(combined, slangPhrases)

	formal := professional + technical + academic + register

	profScore := 0.2
	if words > 0 {
		profScore = math.Min(1.0, float64(professional)*2.0/float64(words)*10.0+0.2)
	}

	techScore := 0.2
	if words > 0 {
		techScore = math.Min(1.0, float64(technical)*2.0/float64(words)*10.0+0.2)
	}

	registerScore := 0.3
	if register > 0 || academic > 0 {
		registerScore = math.Min(1.0, float64(register+academic)*0.15+0.3)
	}

	structScore := 0.2
	if words > 0 {
		structScore = math.Min(1.0, float64(structured)*2.0/float64(words)*10.0+0.2)
	}

	academicScore := 0.2
	if academic > 0 {
		academicScore = math.Min(1.0, float64(academic)*0.25+0.2)
	}

	detail := FormalityDetail{
		ProfessionalTone:    math.Round(profScore*100) / 100,
		TechnicalPrecision:  math.Round(techScore*100) / 100,
		RegisterConsistency: math.Round(registerScore*100) / 100,
		StructuredResponse:  math.Round(structScore*100) / 100,
		AcademicRigor:       math.Round(academicScore*100) / 100,
		FormalCount:         formal,
		InformalCount:       informal,
		SlangCount:          slang,
	}

	score := 30.0
	score += profScore * 15.0
	score += techScore * 15.0
	score += registerScore * 10.0
	score += structScore * 10.0
	score += academicScore * 10.0

	score -= float64(informal) * 4.0
	score -= float64(slang) * 5.0

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

	var models []ModelFormality
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail FormalityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ProfessionalTone += detail.ProfessionalTone
			totalDetail.TechnicalPrecision += detail.TechnicalPrecision
			totalDetail.RegisterConsistency += detail.RegisterConsistency
			totalDetail.StructuredResponse += detail.StructuredResponse
			totalDetail.AcademicRigor += detail.AcademicRigor
			totalDetail.FormalCount += detail.FormalCount
			totalDetail.InformalCount += detail.InformalCount
			totalDetail.SlangCount += detail.SlangCount

			if detail.InformalCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ProfessionalTone = math.Round(totalDetail.ProfessionalTone/n*100) / 100
		totalDetail.TechnicalPrecision = math.Round(totalDetail.TechnicalPrecision/n*100) / 100
		totalDetail.RegisterConsistency = math.Round(totalDetail.RegisterConsistency/n*100) / 100
		totalDetail.StructuredResponse = math.Round(totalDetail.StructuredResponse/n*100) / 100
		totalDetail.AcademicRigor = math.Round(totalDetail.AcademicRigor/n*100) / 100

		models = append(models, ModelFormality{
			Model:          mn,
			FormalityScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].FormalityScore > models[j].FormalityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.FormalityScore
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
	b.WriteString("Response Formality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No formality data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.FormalityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "professional_tone", m.Detail.ProfessionalTone)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "technical_precision", m.Detail.TechnicalPrecision)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "register_consistency", m.Detail.RegisterConsistency)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "structured_response", m.Detail.StructuredResponse)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "academic_rigor", m.Detail.AcademicRigor)
		fmt.Fprintf(&b, "    %-28s %d\n", "formal_count", m.Detail.FormalCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "informal_count", m.Detail.InformalCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "slang_count", m.Detail.SlangCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
