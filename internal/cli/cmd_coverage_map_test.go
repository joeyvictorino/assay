package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverageMapMissingPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"coverage-map"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}

func TestCoverageMapBadFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"coverage-map", "--path", "/tmp", "--format", "xml"}, &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("expected exit %d, got %d", ExitError, code)
	}
}

func TestCoverageMapText(t *testing.T) {
	dir := t.TempDir()
	rr := `{"run_id":"r1","started":"2024-01-01T00:00:00Z","finished":"2024-01-01T01:00:00Z","scope":{},"models":[],"labs":[],"audit_head_hash":"abc","audit_record_count":1,"reflection":[{"lab":"l1","model":"m1","reviewer":"injection-scanner","scored":true,"scores":[7],"final":7,"passed":true,"revisions":0,"validated":1,"refuted":0,"theorized":0}]}`
	os.WriteFile(filepath.Join(dir, "run.json"), []byte(rr), 0644)

	var stdout, stderr bytes.Buffer
	code := Main([]string{"coverage-map", "--path", dir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "injection-scanner") {
		t.Error("expected checker name in output")
	}
}

func TestCoverageMapJSON(t *testing.T) {
	dir := t.TempDir()
	rr := `{"run_id":"r1","started":"2024-01-01T00:00:00Z","finished":"2024-01-01T01:00:00Z","scope":{},"models":[],"labs":[],"audit_head_hash":"abc","audit_record_count":1,"reflection":[]}`
	os.WriteFile(filepath.Join(dir, "run.json"), []byte(rr), 0644)

	var stdout, stderr bytes.Buffer
	code := Main([]string{"coverage-map", "--path", dir, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	var m map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestCoverageMapShowGaps(t *testing.T) {
	dir := t.TempDir()
	rr := `{"run_id":"r1","started":"2024-01-01T00:00:00Z","finished":"2024-01-01T01:00:00Z","scope":{},"models":[],"labs":[],"audit_head_hash":"abc","audit_record_count":1,"reflection":[]}`
	os.WriteFile(filepath.Join(dir, "run.json"), []byte(rr), 0644)

	var stdout, stderr bytes.Buffer
	code := Main([]string{"coverage-map", "--path", dir, "--show-gaps"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Coverage gaps") {
		t.Error("expected gaps output")
	}
}
