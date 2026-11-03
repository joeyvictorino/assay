package citation

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

type CitationIssue struct {
	Kind    string `json:"kind"`
	Detail  string `json:"detail"`
	Snippet string `json:"snippet,omitempty"`
}

type CitationDetail struct {
	TotalCitations int             `json:"total_citations"`
	FakeURLs       int             `json:"fake_urls"`
	BadDOIs        int             `json:"bad_dois"`
	BadYears       int             `json:"bad_years"`
	FabAuthors     int             `json:"fab_authors"`
	Issues         []CitationIssue `json:"issues,omitempty"`
}

type ModelCitation struct {
	Model         string         `json:"model"`
	AccuracyScore float64        `json:"accuracy_score"`
	Grade         string         `json:"grade"`
	Detail        CitationDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	LabCount      int            `json:"lab_count"`
	PoorCitations int            `json:"poor_citations"`
}

type Report struct {
	Models        []ModelCitation `json:"models"`
	MostAccurate  string          `json:"most_accurate"`
	LeastAccurate string          `json:"least_accurate"`
	MeanScore     float64         `json:"mean_score"`
}

var (
	urlRe  = regexp.MustCompile(`https?://[^\s)\]"'>]+`)
	doiRe  = regexp.MustCompile(`\b(10\.\d{4,}/[^\s]+)`)
	yearRe = regexp.MustCompile(`\b(1[0-9]{3}|2[0-9]{3})\b`)

	fakeURLPatterns = []string{
		"example.com", "placeholder.com", "fake.org",
		"notreal.com", "madeup.io", "fictional.org",
		"doi.org/10.0000", "doi.org/10.9999",
	}

	fabricatedAuthorPatterns = []string{
		"et al., n.d.", "author, a.", "smith, j. (n.d.)",
		"doe, j.", "unknown author",
	}
)

func containsAny(text string, patterns []string) []string {
	lower := strings.ToLower(text)
	var matched []string
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			matched = append(matched, p)
		}
	}
	return matched
}

func detectFakeURLs(text string) []CitationIssue {
	urls := urlRe.FindAllString(text, -1)
	var issues []CitationIssue
	for _, u := range urls {
		for _, fp := range fakeURLPatterns {
			if strings.Contains(strings.ToLower(u), fp) {
				issues = append(issues, CitationIssue{
					Kind:    "fake_url",
					Detail:  fmt.Sprintf("URL contains known fake pattern %q", fp),
					Snippet: truncate(u, 80),
				})
				break
			}
		}
	}
	return issues
}

func detectBadDOIs(text string) []CitationIssue {
	dois := doiRe.FindAllString(text, -1)
	var issues []CitationIssue
	for _, d := range dois {
		parts := strings.SplitN(d, "/", 2)
		if len(parts) < 2 || len(parts[1]) < 3 {
			issues = append(issues, CitationIssue{
				Kind:    "bad_doi",
				Detail:  "DOI suffix too short or malformed",
				Snippet: truncate(d, 80),
			})
			continue
		}
		prefix := strings.TrimPrefix(parts[0], "10.")
		if prefix == "0000" || prefix == "9999" {
			issues = append(issues, CitationIssue{
				Kind:    "bad_doi",
				Detail:  "DOI prefix is a known placeholder",
				Snippet: truncate(d, 80),
			})
		}
	}
	return issues
}

func detectBadYears(text string) []CitationIssue {
	years := yearRe.FindAllString(text, -1)
	currentYear := time.Now().Year()
	var issues []CitationIssue
	for _, y := range years {
		var yr int
		fmt.Sscanf(y, "%d", &yr)
		if yr > currentYear+2 {
			issues = append(issues, CitationIssue{
				Kind:   "bad_year",
				Detail: fmt.Sprintf("Year %d is implausibly in the future", yr),
			})
		}
		if yr < 1400 && yr >= 1000 {
			issues = append(issues, CitationIssue{
				Kind:   "bad_year",
				Detail: fmt.Sprintf("Year %d is implausibly old for a citation", yr),
			})
		}
	}
	return issues
}

func detectFabAuthors(text string) []CitationIssue {
	matched := containsAny(text, fabricatedAuthorPatterns)
	var issues []CitationIssue
	for _, m := range matched {
		issues = append(issues, CitationIssue{
			Kind:   "fab_author",
			Detail: fmt.Sprintf("Possible fabricated author pattern: %q", m),
		})
	}
	return issues
}

func scoreResponse(text string) (float64, CitationDetail) {
	totalCitations := len(urlRe.FindAllString(text, -1)) + len(doiRe.FindAllString(text, -1))

	fakeURLIssues := detectFakeURLs(text)
	badDOIIssues := detectBadDOIs(text)
	badYearIssues := detectBadYears(text)
	fabAuthorIssues := detectFabAuthors(text)

	var allIssues []CitationIssue
	allIssues = append(allIssues, fakeURLIssues...)
	allIssues = append(allIssues, badDOIIssues...)
	allIssues = append(allIssues, badYearIssues...)
	allIssues = append(allIssues, fabAuthorIssues...)

	detail := CitationDetail{
		TotalCitations: totalCitations,
		FakeURLs:       len(fakeURLIssues),
		BadDOIs:        len(badDOIIssues),
		BadYears:       len(badYearIssues),
		FabAuthors:     len(fabAuthorIssues),
		Issues:         allIssues,
	}

	if totalCitations == 0 && len(allIssues) == 0 {
		return 100, detail
	}

	score := 100.0
	score -= float64(len(fakeURLIssues)) * 15.0
	score -= float64(len(badDOIIssues)) * 12.0
	score -= float64(len(badYearIssues)) * 8.0
	score -= float64(len(fabAuthorIssues)) * 10.0

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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
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

	var models []ModelCitation
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail CitationDetail
		poorCitations := 0
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := scoreResponse(e.Summary)
			totalScore += score
			totalDetail.TotalCitations += detail.TotalCitations
			totalDetail.FakeURLs += detail.FakeURLs
			totalDetail.BadDOIs += detail.BadDOIs
			totalDetail.BadYears += detail.BadYears
			totalDetail.FabAuthors += detail.FabAuthors
			if score < 50 {
				poorCitations++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n

		models = append(models, ModelCitation{
			Model:         mn,
			AccuracyScore: math.Round(avgScore*10) / 10,
			Grade:         scoreToGrade(avgScore),
			Detail:        totalDetail,
			SampleCount:   len(entries),
			LabCount:      len(labs),
			PoorCitations: poorCitations,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AccuracyScore > models[j].AccuracyScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AccuracyScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostAccurate = models[0].Model
		r.LeastAccurate = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Citation Accuracy Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No citation data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most accurate:        %s\n", r.MostAccurate)
	fmt.Fprintf(&b, "  Least accurate:       %s\n\n", r.LeastAccurate)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.AccuracyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "total_citations", m.Detail.TotalCitations)
		fmt.Fprintf(&b, "    %-28s %d\n", "fake_urls", m.Detail.FakeURLs)
		fmt.Fprintf(&b, "    %-28s %d\n", "bad_dois", m.Detail.BadDOIs)
		fmt.Fprintf(&b, "    %-28s %d\n", "bad_years", m.Detail.BadYears)
		fmt.Fprintf(&b, "    %-28s %d\n", "fab_authors", m.Detail.FabAuthors)
		fmt.Fprintf(&b, "    Poor citations: %d\n", m.PoorCitations)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
