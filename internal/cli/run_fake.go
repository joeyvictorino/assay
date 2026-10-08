package cli

import (
	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider/fake"
)

// fakeScript is the deterministic script the "fake" provider follows on a
// pull-request run: inspect the lab root, then report two low-risk
// observations that the synthetic lab is known to exhibit, then stop. It
// exercises routing, signing, policy, the gate, auditing and overlap without
// spending money or touching a model API. Findings it reports are
// theorized until a checker confirms them like any other.
func fakeScript(name, baseURL string) *fake.Provider {
	return fake.New(fakeSteps(baseURL), fake.Options{Name: name})
}

// fakeSteps is the script itself, exported to tests so they can wrap it
// with provider options such as a canary.
func fakeSteps(baseURL string) []fake.Step {
	return []fake.Step{
		fake.ToolCalls(model.ToolCall{ID: "fk-1", Name: "inspect_headers", Args: map[string]any{"url": baseURL + "/"}}),
		fake.ToolCalls(
			model.ToolCall{ID: "fk-2", Name: "report_finding", Args: map[string]any{
				"class": "security-headers", "method": "GET", "path": "/", "param": "",
				"severity": "low", "summary": "Response lacks Content-Security-Policy and X-Content-Type-Options headers.",
				"evidence_tool_call_ids": []any{"fk-1"},
			}},
			model.ToolCall{ID: "fk-3", Name: "report_finding", Args: map[string]any{
				"class": "verbose-error", "method": "GET", "path": "/api/boom", "param": "",
				"severity": "low", "summary": "Error page returns an internal stack trace.",
				"evidence_tool_call_ids": []any{"fk-1"},
			}},
		),
		fake.Text("Assessment complete."),
		// Self-reflection turn: the same scripted model scores its own report.
		fake.ToolCalls(model.ToolCall{ID: "fk-s1", Name: "score_task", Args: map[string]any{
			"score": 8, "critique": "Covered the visible surface of the root page.",
		}}),
		fake.Text("Scored."),
	}
}
