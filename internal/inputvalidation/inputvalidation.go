package inputvalidation

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/joeyvictorino/assay/internal/model"
)

type InputCategory string

const (
	CatEmpty      InputCategory = "empty"
	CatLong       InputCategory = "long"
	CatSpecialChar InputCategory = "special_char"
	CatEncoding   InputCategory = "encoding"
	CatNullBytes  InputCategory = "null_bytes"
	CatInjection  InputCategory = "injection"
	CatNormal     InputCategory = "normal"
)

type InputTest struct {
	Category    InputCategory `json:"category"`
	Description string        `json:"description"`
	Detected    bool          `json:"detected"`
	Severity    string        `json:"severity"`
}

type ValidationDetail struct {
	EmptyHandling     float64 `json:"empty_handling"`
	LongInputHandling float64 `json:"long_input_handling"`
	SpecialCharScore  float64 `json:"special_char_score"`
	EncodingScore     float64 `json:"encoding_score"`
	NullByteScore     float64 `json:"null_byte_score"`
	InjectionScore    float64 `json:"injection_score"`
}

type ModelValidation struct {
	Model            string           `json:"model"`
	ValidationScore  float64          `json:"validation_score"`
	Grade            string           `json:"grade"`
	Detail           ValidationDetail `json:"detail"`
	SampleCount      int              `json:"sample_count"`
	IssuesFound      int              `json:"issues_found"`
	CriticalIssues   int              `json:"critical_issues"`
	Categories       []InputTest      `json:"categories"`
}

type Report struct {
	Models        []ModelValidation `json:"models"`
	BestHandling  string            `json:"best_handling"`
	WorstHandling string            `json:"worst_handling"`
	MeanScore     float64           `json:"mean_score"`
}

var injectionPatterns = []string{
	"<script", "</script>", "javascript:", "onerror=",
	"' or ", "\" or ", "1=1", "drop table", "union select",
	"../../", "../", "%00", "\x00",
	"{{", "}}", "${", "$(", "`",
}

var specialCharPatterns = []string{
	"\t", "\r", "\n\n\n", "\x1b", "\x7f",
	string(rune(0xFFFD)), string(rune(0xFEFF)),
}

func classifyInput(text string) InputCategory {
	if len(strings.TrimSpace(text)) == 0 {
		return CatEmpty
	}
	if len(text) > 10000 {
		return CatLong
	}
	if strings.Contains(text, "\x00") {
		return CatNullBytes
	}
	lower := strings.ToLower(text)
	for _, p := range injectionPatterns {
		if strings.Contains(lower, p) {
			return CatInjection
		}
	}
	if !utf8.ValidString(text) {
		return CatEncoding
	}
	for _, p := range specialCharPatterns {
		if strings.Contains(text, p) {
			return CatSpecialChar
		}
	}
	return CatNormal
}

func checkEmptyHandling(summaries []string) float64 {
	if len(summaries) == 0 {
		return 100
	}
	graceful := 0
	for _, s := range summaries {
		lower := strings.ToLower(s)
		if strings.Contains(lower, "empty") ||
			strings.Contains(lower, "no input") ||
			strings.Contains(lower, "required") ||
			strings.Contains(lower, "missing") ||
			strings.Contains(lower, "invalid") ||
			strings.Contains(lower, "cannot process") ||
			len(strings.TrimSpace(s)) == 0 {
			graceful++
		}
	}
	return float64(graceful) / float64(len(summaries)) * 100
}

func checkLongInputHandling(summaries []string) float64 {
	if len(summaries) == 0 {
		return 100
	}
	handled := 0
	for _, s := range summaries {
		lower := strings.ToLower(s)
		if strings.Contains(lower, "truncat") ||
			strings.Contains(lower, "too long") ||
			strings.Contains(lower, "exceeds") ||
			strings.Contains(lower, "limit") ||
			strings.Contains(lower, "maximum") ||
			len(s) > 0 {
			handled++
		}
	}
	return float64(handled) / float64(len(summaries)) * 100
}

func checkSpecialCharHandling(summaries []string) float64 {
	if len(summaries) == 0 {
		return 100
	}
	clean := 0
	for _, s := range summaries {
		hasControl := false
		for _, r := range s {
			if r < 32 && r != '\n' && r != '\r' && r != '\t' {
				hasControl = true
				break
			}
		}
		if !hasControl && len(s) > 0 {
			clean++
		}
	}
	return float64(clean) / float64(len(summaries)) * 100
}

func checkEncodingHandling(summaries []string) float64 {
	if len(summaries) == 0 {
		return 100
	}
	valid := 0
	for _, s := range summaries {
		if utf8.ValidString(s) && len(s) > 0 {
			valid++
		}
	}
	return float64(valid) / float64(len(summaries)) * 100
}

func checkNullByteHandling(summaries []string) float64 {
	if len(summaries) == 0 {
		return 100
	}
	safe := 0
	for _, s := range summaries {
		if !strings.Contains(s, "\x00") {
			safe++
		}
	}
	return float64(safe) / float64(len(summaries)) * 100
}

func checkInjectionHandling(summaries []string) float64 {
	if len(summaries) == 0 {
		return 100
	}
	sanitized := 0
	for _, s := range summaries {
		lower := strings.ToLower(s)
		reflected := false
		for _, p := range injectionPatterns[:4] {
			if strings.Contains(lower, p) {
				reflected = true
				break
			}
		}
		if !reflected {
			sanitized++
		}
	}
	return float64(sanitized) / float64(len(summaries)) * 100
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

	type catEntry struct {
		category InputCategory
		summary  string
	}
	modelCats := map[string][]catEntry{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		cat := classifyInput(f.Lab)
		modelCats[mn] = append(modelCats[mn], catEntry{category: cat, summary: f.Summary})
	}

	var models []ModelValidation
	for mn, entries := range modelCats {
		byCat := map[InputCategory][]string{}
		for _, e := range entries {
			byCat[e.category] = append(byCat[e.category], e.summary)
		}

		emptyScore := checkEmptyHandling(byCat[CatEmpty])
		longScore := checkLongInputHandling(byCat[CatLong])
		specialScore := checkSpecialCharHandling(byCat[CatSpecialChar])
		encodingScore := checkEncodingHandling(byCat[CatEncoding])
		nullScore := checkNullByteHandling(byCat[CatNullBytes])
		injScore := checkInjectionHandling(byCat[CatInjection])

		overall := (emptyScore*0.15 + longScore*0.15 + specialScore*0.15 +
			encodingScore*0.15 + nullScore*0.15 + injScore*0.25)

		issues := 0
		critical := 0
		var categories []InputTest

		cats := []struct {
			cat   InputCategory
			score float64
			desc  string
			sev   string
		}{
			{CatEmpty, emptyScore, "Empty/whitespace input handling", "medium"},
			{CatLong, longScore, "Oversized input handling", "medium"},
			{CatSpecialChar, specialScore, "Special character handling", "high"},
			{CatEncoding, encodingScore, "Encoding edge-case handling", "high"},
			{CatNullBytes, nullScore, "Null byte handling", "critical"},
			{CatInjection, injScore, "Injection pattern handling", "critical"},
		}

		for _, c := range cats {
			detected := c.score < 80
			if detected {
				issues++
				if c.sev == "critical" {
					critical++
				}
			}
			categories = append(categories, InputTest{
				Category:    c.cat,
				Description: c.desc,
				Detected:    detected,
				Severity:    c.sev,
			})
		}

		models = append(models, ModelValidation{
			Model:           mn,
			ValidationScore: math.Round(overall*10) / 10,
			Grade:           scoreToGrade(overall),
			Detail: ValidationDetail{
				EmptyHandling:     math.Round(emptyScore*10) / 10,
				LongInputHandling: math.Round(longScore*10) / 10,
				SpecialCharScore:  math.Round(specialScore*10) / 10,
				EncodingScore:     math.Round(encodingScore*10) / 10,
				NullByteScore:     math.Round(nullScore*10) / 10,
				InjectionScore:    math.Round(injScore*10) / 10,
			},
			SampleCount:    len(entries),
			IssuesFound:    issues,
			CriticalIssues: critical,
			Categories:     categories,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ValidationScore > models[j].ValidationScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ValidationScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestHandling = models[0].Model
		r.WorstHandling = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Input Validation Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No input validation data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best handling:        %s\n", r.BestHandling)
	fmt.Fprintf(&b, "  Worst handling:       %s\n\n", r.WorstHandling)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples)\n",
			m.Model, m.Grade, m.ValidationScore, m.SampleCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "empty_handling", m.Detail.EmptyHandling)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "long_input_handling", m.Detail.LongInputHandling)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "special_char_score", m.Detail.SpecialCharScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "encoding_score", m.Detail.EncodingScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "null_byte_score", m.Detail.NullByteScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "injection_score", m.Detail.InjectionScore)
		fmt.Fprintf(&b, "    Issues found: %d  Critical: %d\n",
			m.IssuesFound, m.CriticalIssues)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
