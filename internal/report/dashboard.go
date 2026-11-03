package report

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// DashboardHTML produces a self-contained HTML page showing run history
// with severity breakdown, model coverage, and a sortable findings table.
func DashboardHTML(runs []model.RunReport) string {
	rows := Rows(runs)
	dataJSON, _ := json.Marshal(rows)

	var b strings.Builder
	w := func(s string) { b.WriteString(s) }

	w("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	w("<meta charset=\"utf-8\">\n")
	w("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	w("<title>assay dashboard</title>\n")
	w("<style>\n")
	w(dashboardCSS)
	w("</style>\n</head>\n<body>\n")

	w("<h1>assay dashboard</h1>\n")
	w(fmt.Sprintf("<p class=\"muted\">%d run(s)</p>\n", len(rows)))

	// Summary cards
	if len(rows) > 0 {
		totalValidated, totalTheorized, totalRefusals := 0, 0, 0
		modelSet := map[string]bool{}
		for _, r := range rows {
			for _, m := range r.Models {
				totalValidated += m.FindingsValidated
				totalTheorized += m.FindingsTheorized
				totalRefusals += m.Refusals
				modelSet[m.Model] = true
			}
		}
		w("<div class=\"cards\">\n")
		w(fmt.Sprintf("<div class=\"card\"><div class=\"card-val\">%d</div><div class=\"card-label\">Runs</div></div>\n", len(rows)))
		w(fmt.Sprintf("<div class=\"card\"><div class=\"card-val\">%d</div><div class=\"card-label\">Models</div></div>\n", len(modelSet)))
		w(fmt.Sprintf("<div class=\"card\"><div class=\"card-val\">%d</div><div class=\"card-label\">Validated</div></div>\n", totalValidated))
		w(fmt.Sprintf("<div class=\"card\"><div class=\"card-val\">%d</div><div class=\"card-label\">Theorized</div></div>\n", totalTheorized))
		w(fmt.Sprintf("<div class=\"card\"><div class=\"card-val\">%d</div><div class=\"card-label\">Refusals</div></div>\n", totalRefusals))
		w("</div>\n")
	}

	// Runs table
	w("<h2>Run History</h2>\n")
	w("<table>\n<thead><tr>")
	w("<th>Run</th><th>Date</th><th>Verdict</th><th>Models</th><th>Validated</th><th>Theorized</th><th>Refusals</th><th>Overlap</th>")
	w("</tr></thead>\n<tbody>\n")
	for _, r := range rows {
		verdictClass := "pass"
		if r.Verdict != "pass" && r.Verdict != "" {
			verdictClass = "fail"
		}
		validated, theorized, refusals := 0, 0, 0
		modelNames := make([]string, 0, len(r.Models))
		for _, m := range r.Models {
			validated += m.FindingsValidated
			theorized += m.FindingsTheorized
			refusals += m.Refusals
			modelNames = append(modelNames, m.Model)
		}
		overlap := "-"
		if r.OverlapUnion > 0 {
			overlap = fmt.Sprintf("%d/%d", r.OverlapIntersection, r.OverlapUnion)
		}
		w("<tr>")
		w(fmt.Sprintf("<td>%s</td>", truncID(r.RunID)))
		w(fmt.Sprintf("<td>%s</td>", r.Date))
		w(fmt.Sprintf("<td class=\"%s\">%s</td>", verdictClass, verdictOrDash(r.Verdict)))
		w(fmt.Sprintf("<td>%s</td>", strings.Join(modelNames, ", ")))
		w(fmt.Sprintf("<td>%d</td>", validated))
		w(fmt.Sprintf("<td>%d</td>", theorized))
		w(fmt.Sprintf("<td>%d</td>", refusals))
		w(fmt.Sprintf("<td>%s</td>", overlap))
		w("</tr>\n")
	}
	w("</tbody></table>\n")

	// Model breakdown table
	if len(rows) > 0 {
		w("<h2>Model Breakdown (Latest Run)</h2>\n")
		latest := rows[len(rows)-1]
		w("<table>\n<thead><tr><th>Model</th><th>Validated</th><th>Theorized</th><th>Refusals</th><th>Cost (USD)</th><th>P50 (ms)</th><th>P95 (ms)</th></tr></thead>\n<tbody>\n")
		for _, m := range latest.Models {
			w("<tr>")
			w(fmt.Sprintf("<td>%s</td>", m.Model))
			w(fmt.Sprintf("<td>%d</td>", m.FindingsValidated))
			w(fmt.Sprintf("<td>%d</td>", m.FindingsTheorized))
			w(fmt.Sprintf("<td>%d</td>", m.Refusals))
			w(fmt.Sprintf("<td>%.4f</td>", m.CostUSD))
			w(fmt.Sprintf("<td>%d</td>", m.P50MS))
			w(fmt.Sprintf("<td>%d</td>", m.P95MS))
			w("</tr>\n")
		}
		w("</tbody></table>\n")
	}

	w(fmt.Sprintf("<script>const DATA=%s;</script>\n", dataJSON))
	w("</body>\n</html>\n")
	return b.String()
}

func truncID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func verdictOrDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

const dashboardCSS = `:root { --bg: #fff; --fg: #1a1a2e; --muted: #6b7280; --border: #e5e7eb; --accent: #2563eb; --card-bg: #f9fafb; --pass: #059669; --fail: #dc2626; }
@media (prefers-color-scheme: dark) {
  :root { --bg: #0f172a; --fg: #e2e8f0; --muted: #94a3b8; --border: #334155; --accent: #60a5fa; --card-bg: #1e293b; --pass: #34d399; --fail: #f87171; }
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; color: var(--fg); background: var(--bg); max-width: 1100px; margin: 0 auto; padding: 2rem 1rem; line-height: 1.6; }
h1 { font-size: 1.5rem; margin-bottom: 0.25rem; }
h2 { font-size: 1.15rem; margin: 1.5rem 0 0.5rem; color: var(--accent); }
.muted { color: var(--muted); font-size: 0.875rem; }
.cards { display: flex; gap: 1rem; flex-wrap: wrap; margin: 1rem 0; }
.card { background: var(--card-bg); border: 1px solid var(--border); border-radius: 8px; padding: 1rem 1.25rem; min-width: 120px; text-align: center; }
.card-val { font-size: 1.75rem; font-weight: 700; }
.card-label { font-size: 0.75rem; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
table { width: 100%; border-collapse: collapse; font-size: 0.875rem; margin-bottom: 1rem; }
th, td { padding: 0.5rem 0.75rem; text-align: left; border-bottom: 1px solid var(--border); }
th { font-weight: 600; color: var(--muted); font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; }
tr:hover { background: var(--card-bg); }
.pass { color: var(--pass); font-weight: 600; }
.fail { color: var(--fail); font-weight: 600; }
`
