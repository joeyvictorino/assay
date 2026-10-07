package demolog

import (
	"path/filepath"
	"testing"

	"github.com/joeyvictorino/assay/internal/audit"
	"github.com/joeyvictorino/assay/internal/reconcile"
)

func TestWriteProducesVerifiableLog(t *testing.T) {
	cases := []struct {
		name  string
		runID string
	}{
		{"default run id", "demo-run"},
		{"other run id", "demo-2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			master := audit.GenerateMaster()
			path := filepath.Join(t.TempDir(), "demo.log")
			sum, err := Write(path, tc.runID, master)
			if err != nil {
				t.Fatal(err)
			}
			want := len(Records(tc.runID))
			if int(sum.Records) != want || sum.RunID != tc.runID || !sum.Keyed {
				t.Fatalf("summary=%+v want %d records", sum, want)
			}
			// Unkeyed verification must also pass (chain only).
			if _, err := audit.Verify(path, nil); err != nil {
				t.Fatalf("unkeyed verify: %v", err)
			}
			// A wrong key is rejected.
			if _, err := audit.Verify(path, audit.GenerateMaster()); err == nil {
				t.Fatal("wrong key accepted")
			}
			// The demo transcript and control plane reconcile cleanly.
			if r := reconcile.Reconcile(Records(tc.runID)); r.Checked != 1 || len(r.Mismatches) != 0 {
				t.Fatalf("demo records do not reconcile: %+v", r)
			}
		})
	}
}
