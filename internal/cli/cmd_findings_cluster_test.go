package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFindingsClusterBasic(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high", "message": "xss found"},
		{"checker": "c1", "class": "xss", "severity": "high", "message": "xss found again"},
		{"checker": "c2", "class": "sqli", "severity": "critical", "message": "sql injection"},
	}
	data, _ := json.Marshal(findings)
	f := writeClusterTemp(t, data)

	var out bytes.Buffer
	code := runFindingsCluster([]string{"--input", f, "--format", "json"}, &out, &bytes.Buffer{})
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}

	var result map[string]any
	json.Unmarshal(out.Bytes(), &result)
	if int(result["cluster_count"].(float64)) != 2 {
		t.Fatalf("expected 2 clusters, got %v", result["cluster_count"])
	}
}

func TestFindingsClusterText(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high", "message": "xss"},
		{"checker": "c1", "class": "xss", "severity": "medium", "message": "xss 2"},
	}
	data, _ := json.Marshal(findings)
	f := writeClusterTemp(t, data)

	var out bytes.Buffer
	code := runFindingsCluster([]string{"--input", f}, &out, &bytes.Buffer{})
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "Finding Clusters") {
		t.Fatal("expected header in text output")
	}
}

func TestFindingsClusterCustomFields(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high"},
		{"checker": "c2", "class": "xss", "severity": "high"},
	}
	data, _ := json.Marshal(findings)
	f := writeClusterTemp(t, data)

	var out bytes.Buffer
	code := runFindingsCluster([]string{"--input", f, "--fields", "class,severity", "--format", "json"}, &out, &bytes.Buffer{})
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}

	var result map[string]any
	json.Unmarshal(out.Bytes(), &result)
	if int(result["cluster_count"].(float64)) != 1 {
		t.Fatalf("expected 1 cluster with class,severity key, got %v", result["cluster_count"])
	}
}

func TestFindingsClusterShowMembers(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high"},
		{"checker": "c1", "class": "xss", "severity": "medium"},
	}
	data, _ := json.Marshal(findings)
	f := writeClusterTemp(t, data)

	var out bytes.Buffer
	runFindingsCluster([]string{"--input", f, "--show-members", "--format", "json"}, &out, &bytes.Buffer{})

	var result map[string]any
	json.Unmarshal(out.Bytes(), &result)
	clusters := result["clusters"].([]any)
	c := clusters[0].(map[string]any)
	if c["members"] == nil {
		t.Fatal("expected members to be present")
	}
}

func TestFindingsClusterMissingInput(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runFindingsCluster(nil, &out, &errOut)
	if code != ExitError {
		t.Fatalf("expected error exit, got %d", code)
	}
}

func writeClusterTemp(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "findings-*.json")
	if err != nil {
		t.Fatal(err)
	}
	f.Write(data)
	f.Close()
	return f.Name()
}
