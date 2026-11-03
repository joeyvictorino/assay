package annotate

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type Annotation struct {
	FindingID string    `json:"finding_id"`
	Author    string    `json:"author"`
	Note      string    `json:"note"`
	Tags      []string  `json:"tags,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status,omitempty"`
}

type AnnotatedFindings struct {
	Findings    []map[string]any `json:"findings"`
	Annotations []Annotation     `json:"annotations"`
}

func LoadAnnotated(findingsPath string) (*AnnotatedFindings, error) {
	raw, err := os.ReadFile(findingsPath)
	if err != nil {
		return nil, fmt.Errorf("annotate: read %s: %w", findingsPath, err)
	}

	var af AnnotatedFindings
	if err := json.Unmarshal(raw, &af); err != nil {
		var findings []map[string]any
		if err2 := json.Unmarshal(raw, &findings); err2 != nil {
			return nil, fmt.Errorf("annotate: parse %s: %w", findingsPath, err)
		}
		af.Findings = findings
	}
	return &af, nil
}

func AddAnnotation(af *AnnotatedFindings, findingID, author, note string, tags []string, status string) error {
	found := false
	for _, f := range af.Findings {
		if id, ok := f["id"].(string); ok && id == findingID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("finding %q not found", findingID)
	}

	ann := Annotation{
		FindingID: findingID,
		Author:    author,
		Note:      note,
		Tags:      tags,
		Timestamp: time.Now().UTC().Truncate(time.Second),
		Status:    status,
	}
	af.Annotations = append(af.Annotations, ann)
	return nil
}

func Save(af *AnnotatedFindings, path string) error {
	data, err := json.MarshalIndent(af, "", "  ")
	if err != nil {
		return fmt.Errorf("annotate: marshal: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func ListAnnotations(af *AnnotatedFindings, findingID string) []Annotation {
	var result []Annotation
	for _, a := range af.Annotations {
		if findingID == "" || a.FindingID == findingID {
			result = append(result, a)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
}

func FormatAnnotationsText(annotations []Annotation) string {
	if len(annotations) == 0 {
		return "No annotations.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Annotations (%d):\n", len(annotations))
	for _, a := range annotations {
		fmt.Fprintf(&b, "\n  Finding: %s\n", a.FindingID)
		fmt.Fprintf(&b, "  Author:  %s\n", a.Author)
		fmt.Fprintf(&b, "  Time:    %s\n", a.Timestamp.Format(time.RFC3339))
		if a.Status != "" {
			fmt.Fprintf(&b, "  Status:  %s\n", a.Status)
		}
		if len(a.Tags) > 0 {
			fmt.Fprintf(&b, "  Tags:    %s\n", strings.Join(a.Tags, ", "))
		}
		fmt.Fprintf(&b, "  Note:    %s\n", a.Note)
	}
	return b.String()
}

func Summary(af *AnnotatedFindings) map[string]any {
	byFinding := map[string]int{}
	byAuthor := map[string]int{}
	byStatus := map[string]int{}
	for _, a := range af.Annotations {
		byFinding[a.FindingID]++
		byAuthor[a.Author]++
		if a.Status != "" {
			byStatus[a.Status]++
		}
	}
	return map[string]any{
		"total_annotations":  len(af.Annotations),
		"total_findings":     len(af.Findings),
		"annotated_findings": len(byFinding),
		"by_author":          byAuthor,
		"by_status":          byStatus,
	}
}
