package outputsanitize

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(m, text string) model.Finding {
	return model.Finding{
		Model:    model.ModelRef{Model: m},
		Summary:  text,
		Evidence: []model.Evidence{{Kind: "text", Marker: text}},
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected no models")
	}
}

func TestHTMLInjection(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", `Here is the result: <script>alert(1)</script>`),
		mkFinding("m", "Safe plain text response"),
	}
	r := Analyze(findings)
	var html TypeScore
	for _, s := range r.Models[0].Scores {
		if s.Type == TypeHTML {
			html = s
		}
	}
	if html.Count != 1 {
		t.Errorf("expected 1 HTML injection, got %d", html.Count)
	}
}

func TestJSInjection(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", `Use eval("code") to execute`),
		mkFinding("m", "No JS here"),
	}
	r := Analyze(findings)
	var js TypeScore
	for _, s := range r.Models[0].Scores {
		if s.Type == TypeJS {
			js = s
		}
	}
	if js.Count != 1 {
		t.Errorf("expected 1 JS injection, got %d", js.Count)
	}
}

func TestSQLInjection(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "input ' OR '1'='1"),
		mkFinding("m", "Normal SELECT query results"),
	}
	r := Analyze(findings)
	var sql TypeScore
	for _, s := range r.Models[0].Scores {
		if s.Type == TypeSQL {
			sql = s
		}
	}
	if sql.Count != 1 {
		t.Errorf("expected 1 SQL injection, got %d", sql.Count)
	}
}

func TestCommandInjection(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "run this: ; rm -rf /tmp/data"),
		mkFinding("m", "Safe command output"),
	}
	r := Analyze(findings)
	var cmd TypeScore
	for _, s := range r.Models[0].Scores {
		if s.Type == TypeCmd {
			cmd = s
		}
	}
	if cmd.Count != 1 {
		t.Errorf("expected 1 command injection, got %d", cmd.Count)
	}
}

func TestPathTraversal(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "Read file at ../../etc/passwd"),
		mkFinding("m", "Normal file path /app/data.txt"),
	}
	r := Analyze(findings)
	var path TypeScore
	for _, s := range r.Models[0].Scores {
		if s.Type == TypePath {
			path = s
		}
	}
	if path.Count != 1 {
		t.Errorf("expected 1 path traversal, got %d", path.Count)
	}
}

func TestNoInjections(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "This is a safe response"),
		mkFinding("m", "Another clean output"),
	}
	r := Analyze(findings)
	if r.Models[0].RiskIdx != 0 {
		t.Errorf("expected 0 risk index for clean outputs, got %.2f", r.Models[0].RiskIdx)
	}
}

func TestMultiModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", `<script>xss</script>`),
		mkFinding("claude", "Safe output"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(r.Models))
	}
}

func TestSafestWorst(t *testing.T) {
	findings := []model.Finding{
		mkFinding("safe", "Clean text"),
		mkFinding("safe", "More clean text"),
		mkFinding("risky", `<script>alert(1)</script>`),
		mkFinding("risky", `eval("dangerous")`),
	}
	r := Analyze(findings)
	if r.Safest == "" || r.Worst == "" {
		t.Error("expected safest and worst")
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", `<iframe src="evil">`),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Output Sanitization") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "test"),
	}
	r := Analyze(findings)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(FormatJSON(r)), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestRiskRange(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "safe output"),
	}
	r := Analyze(findings)
	if r.Models[0].RiskIdx < 0 || r.Models[0].RiskIdx > 1 {
		t.Errorf("risk index out of range: %.2f", r.Models[0].RiskIdx)
	}
}

func TestRiskLevel(t *testing.T) {
	tests := []struct {
		idx  float64
		want string
	}{
		{0.6, "critical"},
		{0.35, "high"},
		{0.15, "medium"},
		{0.05, "low"},
		{0.0, "none"},
	}
	for _, tt := range tests {
		got := riskLevel(tt.idx)
		if got != tt.want {
			t.Errorf("riskLevel(%.2f) = %s, want %s", tt.idx, got, tt.want)
		}
	}
}
