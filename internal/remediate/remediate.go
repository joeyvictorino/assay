// Package remediate generates fix sets for findings: a configuration step
// and a ModSecurity virtual patch for every class, plus a real unified diff
// when the finding maps to a planted weakness in labs/synthetic-ops whose fix
// is committed under labs/synthetic-ops/fixes. Verify rebuilds a lab copy
// with a diff applied and re-runs a checker against it.
//
// This is the only internal package that writes files, and only under a
// directory created by os.MkdirTemp.
package remediate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/model"
)

// DefaultFixesDir is where Generate looks for <ground-truth-id>.diff files,
// relative to the working directory (the repository root for the CLI).
const DefaultFixesDir = "labs/synthetic-ops/fixes"

// SyntheticLab is the only lab whose code fixes we own.
const SyntheticLab = "synthetic-ops"

// Generator produces remediations; FixesDir overrides DefaultFixesDir.
type Generator struct {
	FixesDir string
}

// Generate uses DefaultFixesDir.
func Generate(f model.Finding, gt *labs.GroundTruthEntry) model.Remediation {
	return (&Generator{}).Generate(f, gt)
}

// Generate builds the remediation for one finding. ConfigChange and
// VirtualPatch are always set; CodeDiff only when gt points into the
// synthetic lab and a committed fix exists.
func (g *Generator) Generate(f model.Finding, gt *labs.GroundTruthEntry) model.Remediation {
	tpl, ok := templates[f.Class]
	if !ok {
		tpl = templates[model.ClassOther]
	}
	rem := model.Remediation{
		ConfigChange: fill(tpl.Config, f),
		VirtualPatch: VirtualPatch(f),
	}
	if gt != nil && gt.Lab == SyntheticLab && strings.HasPrefix(gt.SourceRef, "handlers/") && validID(gt.ID) {
		dir := g.FixesDir
		if dir == "" {
			dir = DefaultFixesDir
		}
		if data, err := os.ReadFile(filepath.Join(dir, gt.ID+".diff")); err == nil && len(data) > 0 {
			rem.CodeDiff = string(data)
		}
	}
	return rem
}

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func validID(id string) bool { return idRe.MatchString(id) }

// fill substitutes {method}, {path}, {param} in a template string.
func fill(s string, f model.Finding) string {
	method := strings.ToUpper(f.Location.Method)
	if method == "" {
		method = "GET"
	}
	path := f.Location.PathTemplate
	if path == "" {
		path = "/"
	}
	param := f.Location.Param
	if param == "" {
		param = "<parameter>"
	}
	r := strings.NewReplacer("{method}", method, "{path}", path, "{param}", param)
	return r.Replace(s)
}

// RuleID is the deterministic ModSecurity rule id for a class.
func RuleID(c model.Class) int {
	for i, x := range model.AllClasses {
		if x == c {
			return 9100001 + i
		}
	}
	return 9100000
}

// pathRegex turns a path template into an anchored regex fragment where
// {id}-style variables match one path segment.
func pathRegex(tpl string) string {
	if tpl == "" {
		tpl = "/"
	}
	parts := strings.Split(tpl, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			parts[i] = `[^/]+`
		} else {
			parts[i] = regexp.QuoteMeta(p)
		}
	}
	return strings.Join(parts, "/")
}

// VirtualPatch renders the SecRule line followed by its JSON form.
func VirtualPatch(f model.Finding) string {
	tpl, ok := templates[f.Class]
	if !ok {
		tpl = templates[model.ClassOther]
	}
	param := f.Location.Param
	if param == "" {
		param = "*" // all arguments when the finding names none
	}
	target := strings.ReplaceAll(tpl.Target, "{param}", param)
	operator := strings.ReplaceAll(tpl.Operator, "{path_rx}", pathRegex(f.Location.PathTemplate))
	id := RuleID(f.Class)
	actions := fmt.Sprintf("id:%d,phase:%d,%s", id, tpl.Phase, tpl.Action)
	if tpl.Action == "deny" {
		actions += ",status:403"
	}
	actions += ",log," + tpl.Transforms
	msg := fmt.Sprintf("assay %s virtual patch", f.Class)
	if f.ID != "" {
		msg += " " + sanitizeMsg(f.ID)
	}
	actions += fmt.Sprintf(",msg:'%s',tag:'assay/%s'", msg, f.Class)
	rule := fmt.Sprintf(`SecRule %s "%s" "%s"`, target, operator, actions)
	js, _ := json.Marshal(map[string]any{
		"engine":  "modsecurity",
		"id":      id,
		"class":   f.Class,
		"finding": f.ID,
		"rule":    rule,
	})
	return rule + "\n" + string(js)
}

var msgRe = regexp.MustCompile(`[^A-Za-z0-9._:-]`)

func sanitizeMsg(s string) string { return msgRe.ReplaceAllString(s, "_") }

// stateRank orders states for Order: validated first, then theorized,
// refuted and declined.
func stateRank(s model.FindingState) int {
	switch s {
	case model.StateValidated:
		return 0
	case model.StateTheorized:
		return 1
	case model.StateRefuted:
		return 2
	case model.StateDeclined:
		return 3
	}
	return 4
}

// Order returns a copy sorted by severity (critical first), then state
// (validated first), then class and id, so remediation lists are stable.
func Order(fs []model.Finding) []model.Finding {
	out := make([]model.Finding, len(fs))
	copy(out, fs)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Severity.Rank() != b.Severity.Rank() {
			return a.Severity.Rank() > b.Severity.Rank()
		}
		if stateRank(a.State) != stateRank(b.State) {
			return stateRank(a.State) < stateRank(b.State)
		}
		if a.Class != b.Class {
			return a.Class < b.Class
		}
		return a.ID < b.ID
	})
	return out
}

// Classes lists every class with a template (all of model.AllClasses).
func Classes() []model.Class {
	out := make([]model.Class, 0, len(templates))
	for _, c := range model.AllClasses {
		if _, ok := templates[c]; ok {
			out = append(out, c)
		}
	}
	return out
}
