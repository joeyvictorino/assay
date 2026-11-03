package multilingual

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/joeyvictorino/assay/internal/model"
)

type MultilingualDetail struct {
	LanguageDetection  float64 `json:"language_detection"`
	TranslationQuality float64 `json:"translation_quality"`
	CodeSwitchingCount int     `json:"code_switching_count"`
	CulturalScore      float64 `json:"cultural_score"`
	EncodingIssues     int     `json:"encoding_issues"`
	NonASCIIRatio      float64 `json:"non_ascii_ratio"`
	ScriptCount        int     `json:"script_count"`
	MixedScriptPenalty float64 `json:"mixed_script_penalty"`
}

type ModelMultilingual struct {
	Model            string             `json:"model"`
	MultilingualScore float64           `json:"multilingual_score"`
	Grade            string             `json:"grade"`
	Detail           MultilingualDetail `json:"detail"`
	SampleCount      int                `json:"sample_count"`
	LabCount         int                `json:"lab_count"`
	PoorResponses    int                `json:"poor_responses"`
}

type Report struct {
	Models    []ModelMultilingual `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var languageIndicators = map[string][]string{
	"spanish":    {"el", "la", "los", "las", "del", "es", "que", "por", "con", "una"},
	"french":     {"le", "la", "les", "des", "est", "une", "que", "dans", "pour", "pas"},
	"german":     {"der", "die", "das", "und", "ist", "ein", "den", "von", "mit", "nicht"},
	"portuguese": {"o", "os", "da", "das", "que", "em", "para", "com", "uma", "por"},
	"italian":    {"il", "la", "di", "che", "del", "per", "una", "con", "sono", "non"},
	"chinese":    {"的", "是", "在", "了", "不", "和", "有", "这", "我", "他"},
	"japanese":   {"の", "は", "に", "を", "た", "が", "で", "て", "と", "も"},
	"korean":     {"이", "의", "에", "를", "을", "은", "는", "가", "한", "로"},
	"arabic":     {"في", "من", "على", "إلى", "أن", "ما", "هذا", "مع", "لا", "كان"},
	"russian":    {"и", "в", "не", "на", "что", "он", "как", "это", "но", "по"},
}

var translationQualityPhrases = []string{
	"translated as", "in english", "which means",
	"literally", "the translation", "equivalent to",
	"meaning", "translates to", "in other words",
	"that is", "known as", "referred to as",
}

var culturalSensitivityPhrases = []string{
	"culturally", "tradition", "custom", "respectful",
	"sensitivity", "appropriate", "context", "nuance",
	"etiquette", "local", "regional", "indigenous",
	"heritage", "cultural context", "cross-cultural",
}

var codeSwitchRE = regexp.MustCompile(`(?i)\b(es decir|c'est-à-dire|das heißt|то есть|即|つまり)\b`)

var encodingIssuePatterns = []string{
	"â€", "Ã©", "Ã¨", "Ã¼", "Ã¶", "Ã¤",
	"ï¿½", "�", "\\u0000",
	"&amp;#", "&#x", "\\x",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func detectLanguages(text string) []string {
	lower := strings.ToLower(text)
	words := strings.Fields(lower)
	if len(words) == 0 {
		return nil
	}

	langScores := map[string]int{}
	for lang, indicators := range languageIndicators {
		for _, w := range words {
			for _, ind := range indicators {
				if w == ind || strings.Contains(w, ind) {
					langScores[lang]++
				}
			}
		}
	}

	var detected []string
	for lang, score := range langScores {
		if score >= 2 {
			detected = append(detected, lang)
		}
	}
	sort.Strings(detected)
	return detected
}

func countScripts(text string) int {
	scripts := map[string]bool{}
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Latin, r):
			scripts["latin"] = true
		case unicode.Is(unicode.Han, r):
			scripts["han"] = true
		case unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r):
			scripts["japanese"] = true
		case unicode.Is(unicode.Hangul, r):
			scripts["hangul"] = true
		case unicode.Is(unicode.Arabic, r):
			scripts["arabic"] = true
		case unicode.Is(unicode.Cyrillic, r):
			scripts["cyrillic"] = true
		case unicode.Is(unicode.Devanagari, r):
			scripts["devanagari"] = true
		case unicode.Is(unicode.Greek, r):
			scripts["greek"] = true
		}
	}
	return len(scripts)
}

func nonASCIIRatio(text string) float64 {
	if len(text) == 0 {
		return 0
	}
	nonASCII := 0
	total := 0
	for _, r := range text {
		total++
		if r > 127 {
			nonASCII++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(nonASCII) / float64(total)
}

func countEncodingIssues(text string) int {
	count := 0
	for _, p := range encodingIssuePatterns {
		count += strings.Count(text, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, MultilingualDetail) {
	combined := response + " " + evidence
	langs := detectLanguages(combined)
	scripts := countScripts(combined)
	naRatio := nonASCIIRatio(combined)
	encIssues := countEncodingIssues(combined)
	codeSwitches := len(codeSwitchRE.FindAllString(combined, -1))
	transQuality := countPhrases(combined, translationQualityPhrases)
	cultural := countPhrases(combined, culturalSensitivityPhrases)

	langDetection := 0.5
	if len(langs) > 0 {
		langDetection = math.Min(1.0, float64(len(langs))*0.3+0.2)
	}

	transScore := 0.5
	words := len(strings.Fields(combined))
	if words > 0 {
		transScore = math.Min(1.0, float64(transQuality)*3.0/float64(words)*10.0+0.3)
	}

	culturalScore := 0.5
	if words > 0 {
		culturalScore = math.Min(1.0, float64(cultural)*3.0/float64(words)*10.0+0.3)
	}

	mixedPenalty := 0.0
	if scripts > 2 {
		mixedPenalty = float64(scripts-2) * 0.1
	}

	detail := MultilingualDetail{
		LanguageDetection:  math.Round(langDetection*100) / 100,
		TranslationQuality: math.Round(transScore*100) / 100,
		CodeSwitchingCount: codeSwitches,
		CulturalScore:      math.Round(culturalScore*100) / 100,
		EncodingIssues:     encIssues,
		NonASCIIRatio:      math.Round(naRatio*1000) / 1000,
		ScriptCount:        scripts,
		MixedScriptPenalty: math.Round(mixedPenalty*100) / 100,
	}

	score := 50.0
	score += langDetection * 15.0
	score += transScore * 10.0
	score += culturalScore * 10.0
	score -= float64(encIssues) * 5.0
	score -= float64(codeSwitches) * 2.0
	score -= mixedPenalty * 10.0
	if naRatio > 0.01 {
		score += 5.0
	}

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

	var models []ModelMultilingual
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail MultilingualDetail
		poorResponses := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.LanguageDetection += detail.LanguageDetection
			totalDetail.TranslationQuality += detail.TranslationQuality
			totalDetail.CodeSwitchingCount += detail.CodeSwitchingCount
			totalDetail.CulturalScore += detail.CulturalScore
			totalDetail.EncodingIssues += detail.EncodingIssues
			totalDetail.NonASCIIRatio += detail.NonASCIIRatio
			totalDetail.ScriptCount += detail.ScriptCount
			totalDetail.MixedScriptPenalty += detail.MixedScriptPenalty

			if score < 50 {
				poorResponses++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.LanguageDetection = math.Round(totalDetail.LanguageDetection/n*100) / 100
		totalDetail.TranslationQuality = math.Round(totalDetail.TranslationQuality/n*100) / 100
		totalDetail.CulturalScore = math.Round(totalDetail.CulturalScore/n*100) / 100
		totalDetail.NonASCIIRatio = math.Round(totalDetail.NonASCIIRatio/n*1000) / 1000
		totalDetail.MixedScriptPenalty = math.Round(totalDetail.MixedScriptPenalty/n*100) / 100

		models = append(models, ModelMultilingual{
			Model:             mn,
			MultilingualScore: math.Round(avgScore*10) / 10,
			Grade:             scoreToGrade(avgScore),
			Detail:            totalDetail,
			SampleCount:       len(entries),
			LabCount:          len(labs),
			PoorResponses:     poorResponses,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].MultilingualScore > models[j].MultilingualScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.MultilingualScore
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
	b.WriteString("Multilingual Response Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No multilingual data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.MultilingualScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "language_detection", m.Detail.LanguageDetection)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "translation_quality", m.Detail.TranslationQuality)
		fmt.Fprintf(&b, "    %-28s %d\n", "code_switching_count", m.Detail.CodeSwitchingCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "cultural_score", m.Detail.CulturalScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "encoding_issues", m.Detail.EncodingIssues)
		fmt.Fprintf(&b, "    %-28s %.3f\n", "non_ascii_ratio", m.Detail.NonASCIIRatio)
		fmt.Fprintf(&b, "    %-28s %d\n", "script_count", m.Detail.ScriptCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "mixed_script_penalty", m.Detail.MixedScriptPenalty)
		fmt.Fprintf(&b, "    Poor responses: %d\n", m.PoorResponses)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
