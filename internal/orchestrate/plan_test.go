package orchestrate

import (
	"errors"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func task(id string, deps ...string) model.Task {
	return model.Task{ID: id, Kind: "probe", Agent: "probe", Lab: "synthetic-ops", DependsOn: deps}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		plan    Plan
		wantErr string // substring; "" means valid
	}{
		{"single", Plan{Tasks: []model.Task{task("a")}}, ""},
		{"chain", Plan{Tasks: []model.Task{task("a"), task("b", "a"), task("c", "b")}}, ""},
		{"diamond", Plan{Tasks: []model.Task{task("a"), task("b", "a"), task("c", "a"), task("d", "b", "c")}}, ""},
		{"empty", Plan{}, "no tasks"},
		{"blank id", Plan{Tasks: []model.Task{task("")}}, "has no id"},
		{"duplicate id", Plan{Tasks: []model.Task{task("a"), task("a")}}, `duplicate task id "a"`},
		{"unknown dependency", Plan{Tasks: []model.Task{task("a", "zz")}}, `depends on unknown task "zz"`},
		{"self dependency", Plan{Tasks: []model.Task{task("a", "a")}}, `"a" depends on itself`},
		{"two cycle", Plan{Tasks: []model.Task{task("a", "b"), task("b", "a")}}, "cycle: a -> b -> a"},
		{"three cycle behind a root", Plan{Tasks: []model.Task{task("root"), task("x", "root", "z"), task("y", "x"), task("z", "y")}}, "cycle: x -> z -> y -> x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.plan.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() = %v, want substring %q", err, tc.wantErr)
			}
			if !errors.Is(err, ErrInvalidPlan) {
				t.Fatalf("error does not wrap ErrInvalidPlan: %v", err)
			}
		})
	}
}

func TestWaves(t *testing.T) {
	cases := []struct {
		name string
		plan Plan
		want [][]string
	}{
		{"single", Plan{Tasks: []model.Task{task("a")}}, [][]string{{"a"}}},
		{"independent sorted", Plan{Tasks: []model.Task{task("b"), task("c"), task("a")}}, [][]string{{"a", "b", "c"}}},
		{"chain", Plan{Tasks: []model.Task{task("c", "b"), task("a"), task("b", "a")}}, [][]string{{"a"}, {"b"}, {"c"}}},
		{"diamond", Plan{Tasks: []model.Task{task("d", "b", "c"), task("c", "a"), task("b", "a"), task("a")}}, [][]string{{"a"}, {"b", "c"}, {"d"}}},
		{"longest chain wins", Plan{Tasks: []model.Task{task("a"), task("b", "a"), task("c", "a", "b")}}, [][]string{{"a"}, {"b"}, {"c"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			waves, err := tc.plan.Waves()
			if err != nil {
				t.Fatal(err)
			}
			got := make([][]string, len(waves))
			for i, w := range waves {
				for _, task := range w {
					if task.Wave != i {
						t.Errorf("task %s Wave = %d, want %d", task.ID, task.Wave, i)
					}
					got[i] = append(got[i], task.ID)
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("waves = %v, want %v", got, tc.want)
			}
			for i := range got {
				if strings.Join(got[i], ",") != strings.Join(tc.want[i], ",") {
					t.Fatalf("wave %d = %v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
	if _, err := (Plan{Tasks: []model.Task{task("a", "a")}}).Waves(); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("Waves on invalid plan = %v, want ErrInvalidPlan", err)
	}
}

func TestWavesDeterministic(t *testing.T) {
	plan := Plan{Tasks: []model.Task{task("z"), task("m", "z"), task("a", "z"), task("q", "a", "m"), task("b")}}
	first, err := plan.Waves()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, err := plan.Waves()
		if err != nil {
			t.Fatal(err)
		}
		for w := range first {
			for j := range first[w] {
				if first[w][j].ID != again[w][j].ID {
					t.Fatalf("run %d wave %d differs: %v vs %v", i, w, first[w], again[w])
				}
			}
		}
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantIDs []string
		wantErr string
	}{
		{"yaml", "tasks:\n  - id: a\n    kind: recon\n    agent: recon\n    lab: l\n  - id: b\n    depends_on: [a]\n    kind: probe\n    agent: probe\n    lab: l\n", []string{"a", "b"}, ""},
		{"json", `{"tasks":[{"id":"only","kind":"recon","agent":"r","lab":"l"}]}`, []string{"only"}, ""},
		{"empty", "   \n", nil, "empty document"},
		{"unknown field", "tasks:\n  - id: a\n    bogus: 1\n", nil, "unknown field"},
		{"bad yaml", "tasks: [", nil, "yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Parse([]byte(tc.raw))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !errors.Is(err, ErrInvalidPlan) {
					t.Fatalf("Parse() err = %v, want %q wrapping ErrInvalidPlan", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Tasks) != len(tc.wantIDs) {
				t.Fatalf("tasks = %+v", p.Tasks)
			}
			for i, id := range tc.wantIDs {
				if p.Tasks[i].ID != id {
					t.Fatalf("task %d id = %q, want %q", i, p.Tasks[i].ID, id)
				}
			}
		})
	}
}

func TestLoadExamplePlan(t *testing.T) {
	p, err := Load("../../evals/plans/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	waves, err := p.Waves()
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) < 3 {
		t.Fatalf("example plan has %d waves, want at least 3", len(waves))
	}
	if _, err := Load("../../evals/plans/does-not-exist.yaml"); err == nil {
		t.Fatal("Load of a missing file succeeded")
	}
}
