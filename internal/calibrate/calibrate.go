package calibrate

import (
	"fmt"
	"math"
	"sort"

	"github.com/joeyvictorino/assay/internal/model"
)

type CheckerCalibration struct {
	Checker       string  `json:"checker"`
	SampleCount   int     `json:"sample_count"`
	MeanSelfScore float64 `json:"mean_self_score"`
	MeanPeerScore float64 `json:"mean_peer_score"`
	Bias          float64 `json:"bias"`
	AbsError      float64 `json:"abs_error"`
	Correlation   float64 `json:"correlation"`
	Rating        string  `json:"rating"`
}

type OverallCalibration struct {
	MeanBias     float64 `json:"mean_bias"`
	MeanAbsError float64 `json:"mean_abs_error"`
	BestChecker  string  `json:"best_checker"`
	WorstChecker string  `json:"worst_checker"`
}

type CalibrationReport struct {
	Checkers []CheckerCalibration `json:"checkers"`
	Overall  OverallCalibration   `json:"overall"`
}

type scorePair struct {
	self float64
	peer float64
}

func Analyze(rr model.RunReport) CalibrationReport {
	pairs := map[string][]scorePair{}

	for _, r := range rr.Reflection {
		if !r.Scored || r.Final == 0 {
			continue
		}
		key := r.Model
		if key == "" {
			continue
		}
		switch r.ReviewerKind {
		case model.ReviewerSelf:
			existing := pairs[key]
			idx := findUnpairedSelf(existing)
			if idx >= 0 {
				existing[idx].self = float64(r.Final)
			} else {
				pairs[key] = append(pairs[key], scorePair{self: float64(r.Final)})
			}
		case model.ReviewerPeer:
			existing := pairs[key]
			idx := findUnpairedPeer(existing)
			if idx >= 0 {
				existing[idx].peer = float64(r.Final)
			} else {
				pairs[key] = append(pairs[key], scorePair{peer: float64(r.Final)})
			}
		}
	}

	// Also pair by lab: match self and peer reviews for the same model+lab.
	labPairs := map[string][]scorePair{}
	selfByLab := map[string]map[string]float64{}  // model -> lab -> score
	peerByLab := map[string]map[string]float64{}

	for _, r := range rr.Reflection {
		if !r.Scored || r.Final == 0 || r.Model == "" {
			continue
		}
		switch r.ReviewerKind {
		case model.ReviewerSelf:
			if selfByLab[r.Model] == nil {
				selfByLab[r.Model] = map[string]float64{}
			}
			selfByLab[r.Model][r.Lab] = float64(r.Final)
		case model.ReviewerPeer:
			if peerByLab[r.Model] == nil {
				peerByLab[r.Model] = map[string]float64{}
			}
			peerByLab[r.Model][r.Lab] = float64(r.Final)
		}
	}

	for mdl, selfLabs := range selfByLab {
		pl := peerByLab[mdl]
		if pl == nil {
			continue
		}
		for lab, ss := range selfLabs {
			if ps, ok := pl[lab]; ok {
				labPairs[mdl] = append(labPairs[mdl], scorePair{self: ss, peer: ps})
			}
		}
	}

	// Prefer lab-matched pairs; fall back to positional.
	usePairs := labPairs
	if len(labPairs) == 0 {
		usePairs = pairs
	}

	var checkers []CheckerCalibration
	names := make([]string, 0, len(usePairs))
	for n := range usePairs {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		pp := usePairs[name]
		complete := filterComplete(pp)
		if len(complete) == 0 {
			continue
		}
		c := computeCalibration(name, complete)
		checkers = append(checkers, c)
	}

	report := CalibrationReport{Checkers: checkers}
	if len(checkers) > 0 {
		report.Overall = computeOverall(checkers)
	}
	return report
}

func filterComplete(pp []scorePair) []scorePair {
	var out []scorePair
	for _, p := range pp {
		if p.self > 0 && p.peer > 0 {
			out = append(out, p)
		}
	}
	return out
}

func computeCalibration(name string, pp []scorePair) CheckerCalibration {
	n := float64(len(pp))
	var sumSelf, sumPeer, sumBias, sumAbs float64
	for _, p := range pp {
		sumSelf += p.self
		sumPeer += p.peer
		diff := p.self - p.peer
		sumBias += diff
		sumAbs += math.Abs(diff)
	}

	c := CheckerCalibration{
		Checker:       name,
		SampleCount:   len(pp),
		MeanSelfScore: sumSelf / n,
		MeanPeerScore: sumPeer / n,
		Bias:          sumBias / n,
		AbsError:      sumAbs / n,
		Correlation:   pearson(pp),
	}
	c.Rating = rate(c.Bias, c.Correlation)
	return c
}

func pearson(pp []scorePair) float64 {
	n := float64(len(pp))
	if n < 2 {
		return 0
	}
	var sumX, sumY float64
	for _, p := range pp {
		sumX += p.self
		sumY += p.peer
	}
	meanX := sumX / n
	meanY := sumY / n

	var num, denX, denY float64
	for _, p := range pp {
		dx := p.self - meanX
		dy := p.peer - meanY
		num += dx * dy
		denX += dx * dx
		denY += dy * dy
	}
	den := math.Sqrt(denX * denY)
	if den == 0 {
		return 0
	}
	return num / den
}

func rate(bias, corr float64) string {
	absBias := math.Abs(bias)
	switch {
	case absBias < 0.1 && corr > 0.8:
		return "excellent"
	case absBias < 0.2 && corr > 0.6:
		return "good"
	case absBias < 0.3:
		return "fair"
	default:
		return "poor"
	}
}

func computeOverall(cc []CheckerCalibration) OverallCalibration {
	var sumBias, sumAbs float64
	best := cc[0]
	worst := cc[0]
	for _, c := range cc {
		sumBias += c.Bias
		sumAbs += c.AbsError
		if c.AbsError < best.AbsError {
			best = c
		}
		if c.AbsError > worst.AbsError {
			worst = c
		}
	}
	n := float64(len(cc))
	return OverallCalibration{
		MeanBias:     sumBias / n,
		MeanAbsError: sumAbs / n,
		BestChecker:  best.Checker,
		WorstChecker: worst.Checker,
	}
}

func FormatText(r CalibrationReport) string {
	if len(r.Checkers) == 0 {
		return "No calibration data available (need both self and peer review scores).\n"
	}
	s := "Checker Calibration Report\n"
	s += "══════════════════════════════════════════════════════════════════════════════\n"
	s += fmt.Sprintf("  %-20s %6s %8s %8s %7s %7s %7s %s\n",
		"Checker", "N", "Self", "Peer", "Bias", "AbsErr", "Corr", "Rating")
	s += "  ────────────────────────────────────────────────────────────────────────────\n"
	for _, c := range r.Checkers {
		s += fmt.Sprintf("  %-20s %6d %8.2f %8.2f %+7.2f %7.2f %7.2f %s\n",
			c.Checker, c.SampleCount, c.MeanSelfScore, c.MeanPeerScore,
			c.Bias, c.AbsError, c.Correlation, c.Rating)
	}
	s += "\n"
	s += fmt.Sprintf("  Overall: bias %+.2f, abs error %.2f\n", r.Overall.MeanBias, r.Overall.MeanAbsError)
	s += fmt.Sprintf("  Best:  %s  Worst: %s\n", r.Overall.BestChecker, r.Overall.WorstChecker)
	return s
}

func findUnpairedSelf(pp []scorePair) int {
	for i, p := range pp {
		if p.self == 0 {
			return i
		}
	}
	return -1
}

func findUnpairedPeer(pp []scorePair) int {
	for i, p := range pp {
		if p.peer == 0 {
			return i
		}
	}
	return -1
}
