// Package orchestrate turns a list of tasks into dependency waves and runs
// them with self-reflection, bounded delegation and one retry per task.
//
// A Plan is validated before anything runs: every dependency must name a
// task in the plan, no task may depend on itself, and the dependency graph
// must be acyclic. Waves are the topological levels of that graph; tasks in
// the same wave share no dependency on each other and are sorted by id, so
// the schedule is a pure function of the plan.
package orchestrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/joeyvictorino/assay/internal/model"
)

// ErrInvalidPlan wraps every plan validation failure.
var ErrInvalidPlan = errors.New("orchestrate: invalid plan")

// Plan is an ordered set of tasks with dependencies between them.
type Plan struct {
	Tasks []model.Task `json:"tasks"`
}

// Load reads a YAML (or JSON) plan file. Field names follow the json tags
// of model.Task; unknown fields are rejected.
func Load(path string) (Plan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, fmt.Errorf("orchestrate: read %s: %w", path, err)
	}
	p, err := Parse(raw)
	if err != nil {
		return Plan{}, fmt.Errorf("orchestrate: %s: %w", path, err)
	}
	return p, nil
}

// Parse builds a Plan from YAML bytes. The document is a mapping with one
// key, tasks, holding a list of tasks.
func Parse(raw []byte) (Plan, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Plan{}, fmt.Errorf("%w: empty document", ErrInvalidPlan)
	}
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		return Plan{}, fmt.Errorf("%w: yaml: %v", ErrInvalidPlan, err)
	}
	js, err := json.Marshal(generic)
	if err != nil {
		return Plan{}, fmt.Errorf("%w: %v", ErrInvalidPlan, err)
	}
	dec := json.NewDecoder(bytes.NewReader(js))
	dec.DisallowUnknownFields()
	var p Plan
	if err := dec.Decode(&p); err != nil {
		return Plan{}, fmt.Errorf("%w: %v", ErrInvalidPlan, err)
	}
	return p, nil
}

// Validate checks ids, dependencies and acyclicity. A cycle error names
// the cycle path, for example "cycle: a -> b -> c -> a".
func (p Plan) Validate() error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidPlan, fmt.Sprintf(format, args...))
	}
	if len(p.Tasks) == 0 {
		return bad("plan has no tasks")
	}
	ids := map[string]bool{}
	for i, t := range p.Tasks {
		if strings.TrimSpace(t.ID) == "" {
			return bad("task %d has no id", i)
		}
		if ids[t.ID] {
			return bad("duplicate task id %q", t.ID)
		}
		ids[t.ID] = true
	}
	for _, t := range p.Tasks {
		for _, d := range t.DependsOn {
			if d == t.ID {
				return bad("task %q depends on itself", t.ID)
			}
			if !ids[d] {
				return bad("task %q depends on unknown task %q", t.ID, d)
			}
		}
	}
	if cycle := p.findCycle(); cycle != nil {
		return bad("cycle: %s", strings.Join(cycle, " -> "))
	}
	return nil
}

// findCycle returns the first dependency cycle found by depth-first search
// in id order, as a path that starts and ends at the same task, or nil.
func (p Plan) findCycle() []string {
	deps := map[string][]string{}
	ids := make([]string, 0, len(p.Tasks))
	for _, t := range p.Tasks {
		ids = append(ids, t.ID)
		d := append([]string(nil), t.DependsOn...)
		sort.Strings(d)
		deps[t.ID] = d
	}
	sort.Strings(ids)
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	var visit func(id string) []string
	visit = func(id string) []string {
		color[id] = grey
		stack = append(stack, id)
		for _, d := range deps[id] {
			switch color[d] {
			case grey:
				for i, s := range stack {
					if s == d {
						return append(append([]string(nil), stack[i:]...), d)
					}
				}
			case white:
				if c := visit(d); c != nil {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = black
		return nil
	}
	for _, id := range ids {
		if color[id] == white {
			if c := visit(id); c != nil {
				return c
			}
		}
	}
	return nil
}

// Waves returns the topological levels of the plan: a task is in wave n
// when its longest dependency chain has n edges. Each returned task has
// Wave set and tasks within a wave are sorted by id. Waves validates first.
func (p Plan) Waves() ([][]model.Task, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	byID := map[string]model.Task{}
	for _, t := range p.Tasks {
		byID[t.ID] = t
	}
	level := map[string]int{}
	var depth func(id string) int
	depth = func(id string) int {
		if l, ok := level[id]; ok {
			return l
		}
		l := 0
		for _, d := range byID[id].DependsOn {
			if dl := depth(d) + 1; dl > l {
				l = dl
			}
		}
		level[id] = l
		return l
	}
	maxLevel := 0
	for _, t := range p.Tasks {
		if l := depth(t.ID); l > maxLevel {
			maxLevel = l
		}
	}
	waves := make([][]model.Task, maxLevel+1)
	for _, t := range p.Tasks {
		t.Wave = level[t.ID]
		waves[t.Wave] = append(waves[t.Wave], t)
	}
	for _, w := range waves {
		sort.Slice(w, func(i, j int) bool { return w[i].ID < w[j].ID })
	}
	return waves, nil
}
