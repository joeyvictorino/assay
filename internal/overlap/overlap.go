// Package overlap computes the cross-model finding overlap matrix and
// precision/recall against ground truth. Output is byte-stable: models and
// keys are sorted and encoding is done with encoding/json on structs and
// maps (whose keys encoding/json sorts).
package overlap

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/joeyvictorino/assay/internal/finding"
	"github.com/joeyvictorino/assay/internal/model"
)

// GroundTruthEntry is one known vulnerability in a lab, keyed the same way
// findings are.
type GroundTruthEntry struct {
	Lab          string `json:"lab"`
	Class        string `json:"class"`
	Method       string `json:"method"`
	PathTemplate string `json:"path_template"`
	Param        string `json:"param"`
}

// Key returns the dedup key for the entry.
func (g GroundTruthEntry) Key() string {
	return finding.DedupKey(g.Lab, model.Class(g.Class), g.Method, g.PathTemplate, g.Param)
}

// Report bundles the matrix with optional per-model precision.
type Report struct {
	Overlap   model.OverlapMatrix  `json:"overlap"`
	Precision map[string]model.PRF `json:"precision,omitempty"`
}

// KeyOf returns a finding's dedup key, deriving it when the field is empty.
func KeyOf(f model.Finding) string {
	if f.DedupKey != "" {
		return f.DedupKey
	}
	return finding.DedupKey(f.Lab, f.Class, f.Location.Method, f.Location.PathTemplate, f.Location.Param)
}

func keySet(fs []model.Finding) map[string]bool {
	s := make(map[string]bool, len(fs))
	for _, f := range fs {
		s[KeyOf(f)] = true
	}
	return s
}

// Compute builds the overlap matrix. Model names and keys are sorted;
// Membership maps each key to the sorted indexes of the models that found
// it; Pairwise[i][j] compares model i with model j.
func Compute(perModel map[string][]model.Finding, inputs []model.FileDigest) model.OverlapMatrix {
	models := make([]string, 0, len(perModel))
	for m := range perModel {
		models = append(models, m)
	}
	sort.Strings(models)

	sets := make([]map[string]bool, len(models))
	union := map[string]bool{}
	for i, m := range models {
		sets[i] = keySet(perModel[m])
		for k := range sets[i] {
			union[k] = true
		}
	}
	keys := make([]string, 0, len(union))
	for k := range union {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	membership := make(map[string][]int, len(keys))
	intersection := 0
	for _, k := range keys {
		idx := []int{}
		for i := range models {
			if sets[i][k] {
				idx = append(idx, i)
			}
		}
		membership[k] = idx
		if len(idx) == len(models) && len(models) > 0 {
			intersection++
		}
	}

	unique := make(map[string][]string, len(models))
	for i, m := range models {
		u := []string{}
		for _, k := range keys {
			if len(membership[k]) == 1 && membership[k][0] == i {
				u = append(u, k)
			}
		}
		unique[m] = u
	}

	pairwise := make([][]model.PairCell, len(models))
	for i := range models {
		pairwise[i] = make([]model.PairCell, len(models))
		for j := range models {
			pairwise[i][j] = pair(sets[i], sets[j])
		}
	}

	in := append([]model.FileDigest(nil), inputs...)
	sort.Slice(in, func(a, b int) bool {
		if in[a].Name != in[b].Name {
			return in[a].Name < in[b].Name
		}
		return in[a].SHA256 < in[b].SHA256
	})
	if in == nil {
		in = []model.FileDigest{}
	}

	return model.OverlapMatrix{
		Models:       models,
		Keys:         keys,
		Membership:   membership,
		Pairwise:     pairwise,
		UniquePer:    unique,
		Union:        len(keys),
		Intersection: intersection,
		Inputs:       in,
	}
}

func pair(a, b map[string]bool) model.PairCell {
	var cell model.PairCell
	for k := range a {
		if b[k] {
			cell.Both++
		} else {
			cell.OnlyA++
		}
	}
	for k := range b {
		if !a[k] {
			cell.OnlyB++
		}
	}
	if u := cell.Both + cell.OnlyA + cell.OnlyB; u > 0 {
		cell.Jaccard = float64(cell.Both) / float64(u)
	}
	return cell
}

// PRF scores found against groundTruth by dedup key. Duplicate findings
// for one key count once.
func PRF(found []model.Finding, groundTruth []GroundTruthEntry) model.PRF {
	gt := make(map[string]bool, len(groundTruth))
	for _, g := range groundTruth {
		gt[g.Key()] = true
	}
	fs := keySet(found)
	var out model.PRF
	for k := range fs {
		if gt[k] {
			out.TP++
		} else {
			out.FP++
		}
	}
	for k := range gt {
		if !fs[k] {
			out.FN++
		}
	}
	if out.TP+out.FP > 0 {
		out.Precision = float64(out.TP) / float64(out.TP+out.FP)
	}
	if out.TP+out.FN > 0 {
		out.Recall = float64(out.TP) / float64(out.TP+out.FN)
	}
	if out.Precision+out.Recall > 0 {
		out.F1 = 2 * out.Precision * out.Recall / (out.Precision + out.Recall)
	}
	return out
}

// Encode renders the matrix as indented, byte-stable JSON with a trailing
// newline.
func Encode(m model.OverlapMatrix) ([]byte, error) {
	return encode(normalize(m))
}

// EncodeReport renders a Report the same way.
func EncodeReport(r Report) ([]byte, error) {
	r.Overlap = normalize(r.Overlap)
	if len(r.Precision) == 0 {
		r.Precision = nil
	}
	return encode(r)
}

// normalize replaces nil slices and maps with empty ones so the encoding
// does not depend on how the matrix was built.
func normalize(m model.OverlapMatrix) model.OverlapMatrix {
	if m.Models == nil {
		m.Models = []string{}
	}
	if m.Keys == nil {
		m.Keys = []string{}
	}
	if m.Membership == nil {
		m.Membership = map[string][]int{}
	}
	if m.Pairwise == nil {
		m.Pairwise = [][]model.PairCell{}
	}
	if m.UniquePer == nil {
		m.UniquePer = map[string][]string{}
	}
	if m.Inputs == nil {
		m.Inputs = []model.FileDigest{}
	}
	return m
}

func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
