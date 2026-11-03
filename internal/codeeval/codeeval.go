package codeeval

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CodeEvalDetail struct {
	SyntaxScore       float64 `json:"syntax_score"`
	StyleScore        float64 `json:"style_score"`
	SecurityIssues    int     `json:"security_issues"`
	ComplexityScore   float64 `json:"complexity_score"`
	DocScore          float64 `json:"doc_score"`
	NestingDepth      int     `json:"nesting_depth"`
	LongLines         int     `json:"long_lines"`
	TodoCount         int     `json:"todo_count"`
}

type ModelCodeEval struct {
	Model         string         `json:"model"`
	CodeScore     float64        `json:"code_score"`
	Grade         string         `json:"grade"`
	Detail        CodeEvalDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	LabCount      int            `json:"lab_count"`
	PoorCode      int            `json:"poor_code"`
}

type Report struct {
	Models     []ModelCodeEval `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var securityPatterns = []string{
	"sql injection", "sqli",
	"select * from", "drop table", "1=1",
	"<script>", "onerror=", "onload=",
	"eval(", "exec(", "system(",
	"os.system", "subprocess.call",
	"innerHTML", "document.write",
	"password =", "secret =", "api_key =",
	"hardcoded", "plaintext password",
}

var styleIndicators = []string{
	"camelcase", "snake_case",
	"const ", "let ", "var ",
	"def ", "func ", "function ",
	"class ", "struct ", "interface ",
	"import ", "from ", "require(",
}

var docIndicators = []string{
	"//", "/*", "*/", "#",
	"docstring", "\"\"\"", "'''",
	"@param", "@returns", "@return",
	"@throws", "@deprecated",
	"// todo", "// fixme", "// hack",
}

var complexityIndicators = []string{
	"if ", "else ", "elif ", "else if",
	"for ", "while ", "switch ",
	"case ", "try ", "catch ",
	"except ", "finally ",
	"&&", "||", "? ",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func maxNestingDepth(text string) int {
	maxDepth := 0
	depth := 0
	for _, ch := range text {
		switch ch {
		case '{', '(':
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		case '}', ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return maxDepth
}

func countLongLines(text string, limit int) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if len(line) > limit {
			count++
		}
	}
	return count
}

func syntaxScore(text string) float64 {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return 100
	}

	score := 85.0
	openBraces := strings.Count(text, "{")
	closeBraces := strings.Count(text, "}")
	openParens := strings.Count(text, "(")
	closeParens := strings.Count(text, ")")

	braceMismatch := abs(openBraces - closeBraces)
	parenMismatch := abs(openParens - closeParens)
	score -= float64(braceMismatch) * 5.0
	score -= float64(parenMismatch) * 5.0

	emptyLines := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			emptyLines++
		}
	}
	if len(lines) > 0 {
		emptyRatio := float64(emptyLines) / float64(len(lines))
		if emptyRatio > 0.5 {
			score -= 10
		}
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func styleScore(text string) float64 {
	score := 75.0
	indicators := countPhrases(text, styleIndicators)
	score += float64(indicators) * 1.5
	if score > 100 {
		score = 100
	}

	longLines := countLongLines(text, 120)
	score -= float64(longLines) * 2.0
	if score < 0 {
		score = 0
	}
	return score
}

func docScore(text string) float64 {
	indicators := countPhrases(text, docIndicators)
	lines := len(strings.Split(text, "\n"))
	if lines == 0 {
		return 0
	}
	ratio := float64(indicators) / float64(lines)
	score := ratio * 200.0
	if score > 100 {
		score = 100
	}
	return score
}

func complexityScore(text string) float64 {
	indicators := countPhrases(text, complexityIndicators)
	lines := len(strings.Split(text, "\n"))
	if lines == 0 {
		return 100
	}
	ratio := float64(indicators) / float64(lines)
	score := 100.0 - (ratio * 100.0)
	if score < 0 {
		score = 0
	}
	return score
}

func scoreCode(text string) (float64, CodeEvalDetail) {
	security := countPhrases(text, securityPatterns)
	nesting := maxNestingDepth(text)
	longLines := countLongLines(text, 120)
	todos := strings.Count(strings.ToLower(text), "todo") +
		strings.Count(strings.ToLower(text), "fixme") +
		strings.Count(strings.ToLower(text), "hack")

	synScore := syntaxScore(text)
	styScore := styleScore(text)
	secScore := math.Max(0, 100.0-float64(security)*10.0)
	cmpScore := complexityScore(text)
	dScore := docScore(text)

	detail := CodeEvalDetail{
		SyntaxScore:    math.Round(synScore*10) / 10,
		StyleScore:     math.Round(styScore*10) / 10,
		SecurityIssues: security,
		ComplexityScore: math.Round(cmpScore*10) / 10,
		DocScore:       math.Round(dScore*10) / 10,
		NestingDepth:   nesting,
		LongLines:      longLines,
		TodoCount:      todos,
	}

	score := (synScore*0.25 + styScore*0.20 + secScore*0.25 + cmpScore*0.15 + dScore*0.15)

	if nesting > 5 {
		score -= float64(nesting-5) * 3.0
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

	var models []ModelCodeEval
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail CodeEvalDetail
		poorCode := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := scoreCode(e.Summary)
			totalScore += score
			totalDetail.SyntaxScore += detail.SyntaxScore
			totalDetail.StyleScore += detail.StyleScore
			totalDetail.SecurityIssues += detail.SecurityIssues
			totalDetail.ComplexityScore += detail.ComplexityScore
			totalDetail.DocScore += detail.DocScore
			totalDetail.NestingDepth += detail.NestingDepth
			totalDetail.LongLines += detail.LongLines
			totalDetail.TodoCount += detail.TodoCount

			if score < 50 {
				poorCode++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.SyntaxScore = math.Round(totalDetail.SyntaxScore/n*10) / 10
		totalDetail.StyleScore = math.Round(totalDetail.StyleScore/n*10) / 10
		totalDetail.ComplexityScore = math.Round(totalDetail.ComplexityScore/n*10) / 10
		totalDetail.DocScore = math.Round(totalDetail.DocScore/n*10) / 10

		models = append(models, ModelCodeEval{
			Model:       mn,
			CodeScore:   math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			PoorCode:    poorCode,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].CodeScore > models[j].CodeScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.CodeScore
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
	b.WriteString("Code Evaluation Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No code evaluation data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.CodeScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "syntax_score", m.Detail.SyntaxScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "style_score", m.Detail.StyleScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "security_issues", m.Detail.SecurityIssues)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "complexity_score", m.Detail.ComplexityScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "doc_score", m.Detail.DocScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "nesting_depth", m.Detail.NestingDepth)
		fmt.Fprintf(&b, "    %-28s %d\n", "long_lines", m.Detail.LongLines)
		fmt.Fprintf(&b, "    %-28s %d\n", "todo_count", m.Detail.TodoCount)
		fmt.Fprintf(&b, "    Poor code: %d\n", m.PoorCode)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
