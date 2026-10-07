package overlap

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/joeyvictorino/assay/internal/finding"
	"github.com/joeyvictorino/assay/internal/model"
)

func f(lab, class, method, path, param string) model.Finding {
	return model.Finding{Lab: lab, Class: model.Class(class), Location: model.Location{Method: method, PathTemplate: path, Param: param}}
}

var (
	fa = f("lab", "idor", "GET", "/users/{id}", "id")
	fb = f("lab", "sqli", "GET", "/search", "q")
	fc = f("lab", "xss-reflected", "GET", "/echo", "msg")
	fd = f("lab", "csrf", "POST", "/transfer", "")
)

func key(x model.Finding) string { return KeyOf(x) }

func TestCompute(t *testing.T) {
	per := map[string][]model.Finding{
		"zeta":  {fa, fb, fb}, // duplicate counts once
		"alpha": {fa, fc},
		"mid":   {fd},
	}
	inputs := []model.FileDigest{{Name: "z.json", SHA256: "bb", Bytes: 2}, {Name: "a.json", SHA256: "aa", Bytes: 1}}
	m := Compute(per, inputs)

	if len(m.Models) != 3 || m.Models[0] != "alpha" || m.Models[1] != "mid" || m.Models[2] != "zeta" {
		t.Fatalf("models = %v", m.Models)
	}
	if m.Union != 4 || len(m.Keys) != 4 || m.Intersection != 0 {
		t.Fatalf("union=%d keys=%d intersection=%d", m.Union, len(m.Keys), m.Intersection)
	}
	for i := 1; i < len(m.Keys); i++ {
		if m.Keys[i-1] >= m.Keys[i] {
			t.Fatalf("keys not sorted: %v", m.Keys)
		}
	}
	// fa is found by alpha(0) and zeta(2).
	if got := m.Membership[key(fa)]; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("membership(fa) = %v", got)
	}
	if got := m.Membership[key(fd)]; len(got) != 1 || got[0] != 1 {
		t.Fatalf("membership(fd) = %v", got)
	}
	// Unique per model.
	if u := m.UniquePer["alpha"]; len(u) != 1 || u[0] != key(fc) {
		t.Fatalf("unique alpha = %v", u)
	}
	if u := m.UniquePer["zeta"]; len(u) != 1 || u[0] != key(fb) {
		t.Fatalf("unique zeta = %v", u)
	}
	if u := m.UniquePer["mid"]; len(u) != 1 || u[0] != key(fd) {
		t.Fatalf("unique mid = %v", u)
	}
	// Pairwise alpha(0) vs zeta(2): both=1 (fa), onlyA=1 (fc), onlyB=1 (fb) -> jaccard 1/3.
	c := m.Pairwise[0][2]
	if c.Both != 1 || c.OnlyA != 1 || c.OnlyB != 1 || math.Abs(c.Jaccard-1.0/3) > 1e-12 {
		t.Fatalf("pairwise alpha/zeta = %+v", c)
	}
	r := m.Pairwise[2][0]
	if r.Both != 1 || r.OnlyA != 1 || r.OnlyB != 1 {
		t.Fatalf("pairwise zeta/alpha = %+v", r)
	}
	// Diagonal: identical sets.
	d := m.Pairwise[2][2]
	if d.Both != 2 || d.OnlyA != 0 || d.OnlyB != 0 || d.Jaccard != 1 {
		t.Fatalf("diagonal = %+v", d)
	}
	// Disjoint: mid vs alpha.
	if x := m.Pairwise[1][0]; x.Both != 0 || x.Jaccard != 0 || x.OnlyA != 1 || x.OnlyB != 2 {
		t.Fatalf("mid/alpha = %+v", x)
	}
	if m.Inputs[0].Name != "a.json" || m.Inputs[1].Name != "z.json" {
		t.Fatalf("inputs not sorted: %v", m.Inputs)
	}
}

func TestComputeIntersectionAndEmpty(t *testing.T) {
	m := Compute(map[string][]model.Finding{"a": {fa, fb}, "b": {fa, fb, fc}}, nil)
	if m.Intersection != 2 || m.Union != 3 {
		t.Fatalf("intersection=%d union=%d", m.Intersection, m.Union)
	}
	if len(m.UniquePer["a"]) != 0 || len(m.UniquePer["b"]) != 1 {
		t.Fatalf("unique = %v", m.UniquePer)
	}
	e := Compute(nil, nil)
	if e.Union != 0 || e.Intersection != 0 || len(e.Models) != 0 || len(e.Pairwise) != 0 {
		t.Fatalf("empty = %+v", e)
	}
	if _, err := Encode(e); err != nil {
		t.Fatal(err)
	}
}

func TestKeyOfPrefersStoredKey(t *testing.T) {
	x := fa
	x.DedupKey = "stored"
	if KeyOf(x) != "stored" {
		t.Fatal("stored key ignored")
	}
	want := finding.DedupKey("lab", model.ClassIDOR, "GET", "/users/{id}", "id")
	if KeyOf(fa) != want {
		t.Fatalf("derived key = %s want %s", KeyOf(fa), want)
	}
}

func TestPRF(t *testing.T) {
	gt := []GroundTruthEntry{
		{Lab: "lab", Class: "idor", Method: "get", PathTemplate: "/users/7", Param: "id"}, // normalizes to fa
		{Lab: "lab", Class: "sqli", Method: "GET", PathTemplate: "/search", Param: "q"},   // fb
		{Lab: "lab", Class: "csrf", Method: "POST", PathTemplate: "/transfer", Param: ""}, // fd
	}
	tests := []struct {
		name       string
		found      []model.Finding
		tp, fp, fn int
		p, r, f1   float64
	}{
		{"perfect", []model.Finding{fa, fb, fd}, 3, 0, 0, 1, 1, 1},
		{"one fp one fn", []model.Finding{fa, fb, fc}, 2, 1, 1, 2.0 / 3, 2.0 / 3, 2.0 / 3},
		{"duplicates once", []model.Finding{fa, fa, fa}, 1, 0, 2, 1, 1.0 / 3, 0.5},
		{"nothing found", nil, 0, 0, 3, 0, 0, 0},
		{"all wrong", []model.Finding{fc}, 0, 1, 3, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PRF(tt.found, gt)
			if got.TP != tt.tp || got.FP != tt.fp || got.FN != tt.fn {
				t.Fatalf("counts = %+v", got)
			}
			if math.Abs(got.Precision-tt.p) > 1e-12 || math.Abs(got.Recall-tt.r) > 1e-12 || math.Abs(got.F1-tt.f1) > 1e-12 {
				t.Fatalf("prf = %+v", got)
			}
		})
	}
	if got := PRF([]model.Finding{fa}, nil); got.TP != 0 || got.FP != 1 || got.Recall != 0 {
		t.Fatalf("no ground truth = %+v", got)
	}
}

func TestEncodeStable(t *testing.T) {
	per := map[string][]model.Finding{"zeta": {fa, fb}, "alpha": {fa, fc}, "mid": {fd}}
	inputs := []model.FileDigest{{Name: "z.json", SHA256: "bb"}, {Name: "a.json", SHA256: "aa"}}
	first, err := Encode(Compute(per, inputs))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		// Rebuild from a differently ordered map and inputs slice.
		again, err := Encode(Compute(map[string][]model.Finding{"mid": {fd}, "alpha": {fc, fa}, "zeta": {fb, fa}},
			[]model.FileDigest{{Name: "a.json", SHA256: "aa"}, {Name: "z.json", SHA256: "bb"}}))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, again) {
			t.Fatalf("encoding unstable:\n%s\n---\n%s", first, again)
		}
	}
	if first[len(first)-1] != '\n' {
		t.Fatal("missing trailing newline")
	}
	var back model.OverlapMatrix
	if err := json.Unmarshal(first, &back); err != nil {
		t.Fatal(err)
	}
	if back.Union != 4 || len(back.Models) != 3 {
		t.Fatalf("round trip = %+v", back)
	}
	// nil-vs-empty differences in the input must not change the bytes.
	a, _ := Encode(model.OverlapMatrix{})
	b, _ := Encode(normalize(model.OverlapMatrix{}))
	if !bytes.Equal(a, b) {
		t.Fatalf("nil/empty differ:\n%s\n%s", a, b)
	}
}

func TestEncodeReport(t *testing.T) {
	m := Compute(map[string][]model.Finding{"a": {fa}}, nil)
	r1, err := EncodeReport(Report{Overlap: m, Precision: map[string]model.PRF{"a": PRF([]model.Finding{fa}, nil)}})
	if err != nil {
		t.Fatal(err)
	}
	r2, _ := EncodeReport(Report{Overlap: m, Precision: map[string]model.PRF{"a": PRF([]model.Finding{fa}, nil)}})
	if !bytes.Equal(r1, r2) {
		t.Fatal("report encoding unstable")
	}
	noPrec, _ := EncodeReport(Report{Overlap: m, Precision: map[string]model.PRF{}})
	if bytes.Contains(noPrec, []byte(`"precision"`)) {
		t.Fatalf("empty precision should be omitted: %s", noPrec)
	}
	var back Report
	if err := json.Unmarshal(r1, &back); err != nil || back.Precision["a"].FP != 1 {
		t.Fatalf("round trip = %+v err=%v", back, err)
	}
}
