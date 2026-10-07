package config

import (
	"path/filepath"
	"testing"
)

// Every committed run configuration must load and validate.
func TestCommittedRunConfigsLoad(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "runs", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no run configs found: %v", err)
	}
	for _, f := range files {
		if _, err := Load(f); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}
