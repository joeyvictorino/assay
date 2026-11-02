package validate

import (
	"testing"
)

func TestCheckerInfosCoversAllCheckers(t *testing.T) {
	checkers := Checkers()
	infos := CheckerInfos()
	if len(infos) != len(checkers) {
		t.Fatalf("CheckerInfos() returned %d entries, want %d (one per Checker)", len(infos), len(checkers))
	}
	for _, info := range infos {
		if info.Description == "" {
			t.Errorf("checker %s has empty description", info.Class)
		}
		if info.Validates == "(no description)" {
			t.Errorf("checker %s missing validation description", info.Class)
		}
	}
}
