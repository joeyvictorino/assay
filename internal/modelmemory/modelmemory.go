package modelmemory

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type Turn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Index   int    `json:"index"`
}

type MemoryFinding struct {
	FindingID   string   `json:"finding_id"`
	Category    string   `json:"category"`
	Severity    string   `json:"severity"`
	TurnIndex   int      `json:"turn_index"`
	Description string   `json:"description"`
	Evidence    []string `json:"evidence"`
}

type MemorySummary struct {
	TotalTurns           int    `json:"total_turns"`
	FactsIntroduced      int    `json:"facts_introduced"`
	FactsRetained        int    `json:"facts_retained"`
	InstructionsGiven    int    `json:"instructions_given"`
	InstructionsFollowed int    `json:"instructions_followed"`
	RetentionScore       int    `json:"retention_score"`
	Grade                string `json:"grade"`
}

type MemoryReport struct {
	Findings []MemoryFinding `json:"findings"`
	Summary  MemorySummary   `json:"summary"`
}

var (
	factIntroRe    = regexp.MustCompile(`(?i)\b(my name is|i am|i'm|the (?:answer|password|code|key|value|number) is|remember that|note that|keep in mind)\b`)
	instructionRe  = regexp.MustCompile(`(?i)\b(always|never|from now on|going forward|in all responses|every time|make sure to|do not forget|remember to)\b`)
	pronounRe      = regexp.MustCompile(`(?i)\b(it|they|them|this|that|these|those|the above|the previous|as mentioned)\b`)
)

func Analyze(turns []Turn) MemoryReport {
	if len(turns) == 0 {
		return MemoryReport{
			Summary: MemorySummary{
				RetentionScore: 100,
				Grade:          "A",
			},
		}
	}

	var findings []MemoryFinding
	factsIntroduced := 0
	factsRetained := 0
	instructionsGiven := 0
	instructionsFollowed := 0

	type fact struct {
		keywords []string
		turn     int
	}
	type instruction struct {
		keywords []string
		turn     int
	}

	var facts []fact
	var instructions []instruction

	for _, t := range turns {
		if t.Role != "user" {
			continue
		}
		if factIntroRe.MatchString(t.Content) {
			factsIntroduced++
			words := significantWords(t.Content)
			facts = append(facts, fact{keywords: words, turn: t.Index})
		}
		if instructionRe.MatchString(t.Content) {
			instructionsGiven++
			words := significantWords(t.Content)
			instructions = append(instructions, instruction{keywords: words, turn: t.Index})
		}
	}

	findingCounter := 0
	for _, t := range turns {
		if t.Role != "assistant" {
			continue
		}
		lower := strings.ToLower(t.Content)

		for _, f := range facts {
			if f.turn >= t.Index {
				continue
			}
			matched := 0
			for _, kw := range f.keywords {
				if strings.Contains(lower, kw) {
					matched++
				}
			}
			if len(f.keywords) > 0 && matched >= len(f.keywords)/2+1 {
				factsRetained++
			}
		}

		for _, inst := range instructions {
			if inst.turn >= t.Index {
				continue
			}
			matched := 0
			for _, kw := range inst.keywords {
				if strings.Contains(lower, kw) {
					matched++
				}
			}
			if len(inst.keywords) > 0 && matched >= len(inst.keywords)/3+1 {
				instructionsFollowed++
			}
		}

		pronounCount := len(pronounRe.FindAllString(t.Content, -1))
		if pronounCount > 5 && t.Index > 2 {
			findingCounter++
			findings = append(findings, MemoryFinding{
				FindingID:   fmt.Sprintf("MM-%03d", findingCounter),
				Category:    "reference-track",
				Severity:    "low",
				TurnIndex:   t.Index,
				Description: fmt.Sprintf("Turn %d has %d pronoun/anaphora references that may be ambiguous", t.Index, pronounCount),
				Evidence:    []string{fmt.Sprintf("pronoun_count=%d", pronounCount)},
			})
		}
	}

	if factsIntroduced > 0 && factsRetained == 0 {
		findingCounter++
		findings = append(findings, MemoryFinding{
			FindingID:   fmt.Sprintf("MM-%03d", findingCounter),
			Category:    "fact-recall",
			Severity:    "high",
			TurnIndex:   turns[len(turns)-1].Index,
			Description: fmt.Sprintf("None of %d introduced facts were retained in later responses", factsIntroduced),
			Evidence:    []string{fmt.Sprintf("facts_introduced=%d", factsIntroduced)},
		})
	}

	if instructionsGiven > 0 && instructionsFollowed == 0 {
		findingCounter++
		findings = append(findings, MemoryFinding{
			FindingID:   fmt.Sprintf("MM-%03d", findingCounter),
			Category:    "instruction-persist",
			Severity:    "high",
			TurnIndex:   turns[len(turns)-1].Index,
			Description: fmt.Sprintf("None of %d instructions were followed in later responses", instructionsGiven),
			Evidence:    []string{fmt.Sprintf("instructions_given=%d", instructionsGiven)},
		})
	}

	retentionScore := 100
	if factsIntroduced > 0 {
		factRate := float64(factsRetained) / float64(factsIntroduced)
		retentionScore = int(factRate * 50)
	}
	if instructionsGiven > 0 {
		instrRate := float64(instructionsFollowed) / float64(instructionsGiven)
		retentionScore += int(instrRate * 50)
	}
	if factsIntroduced == 0 && instructionsGiven == 0 {
		retentionScore = 100
	}
	if retentionScore > 100 {
		retentionScore = 100
	}

	return MemoryReport{
		Findings: findings,
		Summary: MemorySummary{
			TotalTurns:           len(turns),
			FactsIntroduced:      factsIntroduced,
			FactsRetained:        factsRetained,
			InstructionsGiven:    instructionsGiven,
			InstructionsFollowed: instructionsFollowed,
			RetentionScore:       retentionScore,
			Grade:                scoreGrade(retentionScore),
		},
	}
}

func significantWords(s string) []string {
	stop := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "that": true, "this": true,
		"it": true, "i": true, "my": true, "me": true, "we": true,
		"you": true, "your": true, "and": true, "or": true, "not": true,
		"no": true, "but": true, "if": true, "so": true, "as": true,
		"all": true, "am": true, "i'm": true, "name": true,
	}
	var out []string
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,!?;:\"'()")
		if len(w) > 2 && !stop[w] {
			out = append(out, w)
		}
	}
	return out
}

func scoreGrade(score int) string {
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

func FormatText(r MemoryReport) string {
	var sb strings.Builder
	sb.WriteString("Context Retention Analysis\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&sb, "  Total turns:            %d\n", r.Summary.TotalTurns)
	fmt.Fprintf(&sb, "  Facts introduced:       %d\n", r.Summary.FactsIntroduced)
	fmt.Fprintf(&sb, "  Facts retained:         %d\n", r.Summary.FactsRetained)
	fmt.Fprintf(&sb, "  Instructions given:     %d\n", r.Summary.InstructionsGiven)
	fmt.Fprintf(&sb, "  Instructions followed:  %d\n", r.Summary.InstructionsFollowed)
	fmt.Fprintf(&sb, "  Retention score:        %d/100 (%s)\n", r.Summary.RetentionScore, r.Summary.Grade)

	if len(r.Findings) > 0 {
		sb.WriteString("\nFindings:\n")
		for i, f := range r.Findings {
			fmt.Fprintf(&sb, "  %d. [%s] %s: %s\n", i+1, strings.ToUpper(f.Severity), f.FindingID, f.Description)
		}
	}

	return sb.String()
}

func FormatJSON(r MemoryReport) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}
