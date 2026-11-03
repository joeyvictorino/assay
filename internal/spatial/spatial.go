package spatial

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type SpatialDetail struct {
	SpatialOrientation    float64 `json:"spatial_orientation"`
	RelativePosition      float64 `json:"relative_position"`
	DistanceEstimation    float64 `json:"distance_estimation"`
	GeometricReasoning    float64 `json:"geometric_reasoning"`
	SpatialTransformation float64 `json:"spatial_transformation"`
	SpatialCount          int     `json:"spatial_count"`
	DisorientedCount      int     `json:"disoriented_count"`
	AmbiguousCount        int     `json:"ambiguous_count"`
}

type ModelSpatial struct {
	Model        string        `json:"model"`
	SpatialScore float64       `json:"spatial_score"`
	Grade        string        `json:"grade"`
	Detail       SpatialDetail `json:"detail"`
	SampleCount  int           `json:"sample_count"`
	LabCount     int           `json:"lab_count"`
	Concerns     int           `json:"concerns"`
}

type Report struct {
	Models     []ModelSpatial `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

var orientationPhrases = []string{
	"facing north", "oriented toward",
	"pointing in the direction", "aligned with",
	"rotated clockwise", "from the perspective of",
	"looking toward", "the front faces",
	"the orientation is", "turned to face",
	"angled at", "positioned facing",
	"the heading is", "compass bearing", "azimuth",
}

var positionPhrases = []string{
	"to the left of", "to the right of",
	"above the", "below the",
	"adjacent to", "next to",
	"in front of", "behind the",
	"between the two", "at the intersection",
	"north of", "south of",
	"on top of", "underneath the",
	"relative to", "with respect to",
}

var distancePhrases = []string{
	"approximately meters", "the distance between",
	"within range of", "the gap is",
	"separated by", "spans a distance",
	"close proximity", "far from",
	"the radius extends", "the perimeter measures",
	"measured from", "the length is",
	"the width spans", "kilometers away", "at a distance",
}

var geometricPhrases = []string{
	"the angle between", "perpendicular to",
	"parallel to", "the triangle formed",
	"circular path", "rectangular area",
	"the diagonal runs", "symmetrical about",
	"the volume of", "the surface area",
	"congruent to", "proportional to",
	"the polygon", "inscribed within", "circumscribed by",
}

var disorientedPhrases = []string{
	"simultaneously left and right",
	"both above and below",
	"contradictory positions",
	"spatial inconsistency", "impossible placement",
	"conflicting locations", "disoriented",
	"misplaced relative to",
	"the positions don't align",
	"spatially impossible",
	"contradictory directions",
	"location paradox",
	"at odds with the layout",
	"conflicting spatial claims",
}

var ambiguousPhrases = []string{
	"somewhere around there", "in that general area",
	"roughly positioned", "vaguely located",
	"not precisely placed", "approximately there",
	"in the vicinity", "loosely arranged",
	"unspecified location", "somewhere nearby",
	"in that region", "around that spot",
	"no exact position",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, SpatialDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	orientation := countPhrases(combined, orientationPhrases)
	position := countPhrases(combined, positionPhrases)
	distance := countPhrases(combined, distancePhrases)
	geometric := countPhrases(combined, geometricPhrases)
	disoriented := countPhrases(combined, disorientedPhrases)
	ambiguous := countPhrases(combined, ambiguousPhrases)

	spatial := orientation + position + distance + geometric

	orientScore := 0.2
	if words > 0 {
		orientScore = math.Min(1.0, float64(orientation)*2.0/float64(words)*10.0+0.2)
	}

	posScore := 0.2
	if words > 0 {
		posScore = math.Min(1.0, float64(position)*2.0/float64(words)*10.0+0.2)
	}

	distScore := 0.2
	if distance > 0 || position > 0 {
		distScore = math.Min(1.0, float64(distance+position)*0.12+0.2)
	}

	geoScore := 0.3
	if words > 0 {
		geoScore = math.Min(1.0, float64(geometric)*2.0/float64(words)*10.0+0.3)
	}

	transformScore := 0.2
	if disoriented == 0 && ambiguous == 0 {
		transformScore = 0.8
	} else if disoriented+ambiguous < 3 {
		transformScore = 0.5
	}

	detail := SpatialDetail{
		SpatialOrientation:    math.Round(orientScore*100) / 100,
		RelativePosition:      math.Round(posScore*100) / 100,
		DistanceEstimation:    math.Round(distScore*100) / 100,
		GeometricReasoning:    math.Round(geoScore*100) / 100,
		SpatialTransformation: math.Round(transformScore*100) / 100,
		SpatialCount:          spatial,
		DisorientedCount:      disoriented,
		AmbiguousCount:        ambiguous,
	}

	score := 30.0
	score += orientScore * 15.0
	score += posScore * 15.0
	score += distScore * 10.0
	score += geoScore * 10.0
	score += transformScore * 10.0

	score -= float64(disoriented) * 3.0
	score -= float64(ambiguous) * 4.0

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

	var models []ModelSpatial
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail SpatialDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.SpatialOrientation += detail.SpatialOrientation
			totalDetail.RelativePosition += detail.RelativePosition
			totalDetail.DistanceEstimation += detail.DistanceEstimation
			totalDetail.GeometricReasoning += detail.GeometricReasoning
			totalDetail.SpatialTransformation += detail.SpatialTransformation
			totalDetail.SpatialCount += detail.SpatialCount
			totalDetail.DisorientedCount += detail.DisorientedCount
			totalDetail.AmbiguousCount += detail.AmbiguousCount

			if detail.DisorientedCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.SpatialOrientation = math.Round(totalDetail.SpatialOrientation/n*100) / 100
		totalDetail.RelativePosition = math.Round(totalDetail.RelativePosition/n*100) / 100
		totalDetail.DistanceEstimation = math.Round(totalDetail.DistanceEstimation/n*100) / 100
		totalDetail.GeometricReasoning = math.Round(totalDetail.GeometricReasoning/n*100) / 100
		totalDetail.SpatialTransformation = math.Round(totalDetail.SpatialTransformation/n*100) / 100

		models = append(models, ModelSpatial{
			Model:        mn,
			SpatialScore: math.Round(avgScore*10) / 10,
			Grade:        scoreToGrade(avgScore),
			Detail:       totalDetail,
			SampleCount:  len(entries),
			LabCount:     len(labs),
			Concerns:     totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].SpatialScore > models[j].SpatialScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.SpatialScore
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
	b.WriteString("Spatial Reasoning Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No spatial reasoning data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.SpatialScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "spatial_orientation", m.Detail.SpatialOrientation)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "relative_position", m.Detail.RelativePosition)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "distance_estimation", m.Detail.DistanceEstimation)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "geometric_reasoning", m.Detail.GeometricReasoning)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "spatial_transformation", m.Detail.SpatialTransformation)
		fmt.Fprintf(&b, "    %-28s %d\n", "spatial_count", m.Detail.SpatialCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "disoriented_count", m.Detail.DisorientedCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "ambiguous_count", m.Detail.AmbiguousCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
