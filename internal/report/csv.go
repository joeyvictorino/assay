package report

import (
	"bytes"
	"encoding/csv"

	"github.com/joeyvictorino/assay/internal/model"
)

var csvHeader = []string{
	"id", "class", "severity", "state", "method", "path", "param",
	"lab", "model", "cwe", "summary", "reconciled", "first_seen",
}

// FindingsCSV renders findings as RFC 4180 CSV.
func FindingsCSV(findings []model.Finding) string {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write(csvHeader)
	for _, f := range findings {
		reconciled := "false"
		if f.Reconcile.Checked && f.Reconcile.Match {
			reconciled = "true"
		}
		w.Write([]string{
			f.ID,
			string(f.Class),
			string(f.Severity),
			string(f.State),
			f.Location.Method,
			f.Location.PathTemplate,
			f.Location.Param,
			f.Lab,
			f.Model.Model,
			f.CWE,
			f.Summary,
			reconciled,
			f.FirstSeen.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	w.Flush()
	return buf.String()
}
