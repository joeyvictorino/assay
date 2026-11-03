package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutlierMissingPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"outlier"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}

func TestOutlierBadFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"outlier", "--path", "/tmp", "--format", "xml"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}

func TestOutlierText(t *testing.T) {
	dir := t.TempDir()
	rr := `{"run_id":"r1","started":"2024-01-01T00:00:00Z","finished":"2024-01-01T01:00:00Z","scope":{},"models":[],"labs":[],"audit_head_hash":"abc","audit_record_count":1,"reflection":[
		{"lab":"l1","model":"m1","reviewer":"c1","scored":true,"scores":[5],"final":5,"passed":true,"revisions":0,"validated":1,"refuted":0,"theorized":0},
		{"lab":"l2","model":"m2","reviewer":"c1","scored":true,"scores":[5],"final":5,"passed":true,"revisions":0,"validated":1,"refuted":0,"theorized":0},
		{"lab":"l3","model":"m3","reviewer":"c1","scored":true,"scores":[5],"final":5,"passed":true,"revisions":0,"validated":1,"refuted":0,"theorized":0}
	]}`
	os.WriteFile(filepath.Join(dir, "run.json"), []byte(rr), 0644)

	var stdout, stderr bytes.Buffer
	code := Main([]string{"outlier", "--path", dir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Outlier Detection") {
		t.Error("expected outlier header in output")
	}
}

func TestOutlierJSON(t *testing.T) {
	dir := t.TempDir()
	rr := `{"run_id":"r1","started":"2024-01-01T00:00:00Z","finished":"2024-01-01T01:00:00Z","scope":{},"models":[],"labs":[],"audit_head_hash":"abc","audit_record_count":1,"reflection":[]}`
	os.WriteFile(filepath.Join(dir, "run.json"), []byte(rr), 0644)

	var stdout, stderr bytes.Buffer
	code := Main([]string{"outlier", "--path", dir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var m map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestOutlierTypeFilter(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"outlier", "--path", "/tmp", "--type", "badtype"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}
