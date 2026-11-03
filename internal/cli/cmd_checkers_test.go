package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCheckersCommand(t *testing.T) {
	var out bytes.Buffer
	code := runCheckers(nil, &out, &bytes.Buffer{})
	if code != ExitPass {
		t.Fatalf("checkers command returned %d, want %d", code, ExitPass)
	}
	s := out.String()
	if !strings.Contains(s, "sqli") {
		t.Error("output missing sqli checker")
	}
	if !strings.Contains(s, "csrf") {
		t.Error("output missing csrf checker")
	}
	if !strings.Contains(s, "19 checkers registered") {
		t.Errorf("unexpected checker count in output: %s", s)
	}
}
