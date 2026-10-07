package zdr

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestNullSinkDiscards(t *testing.T) {
	var s NullSink
	body := []byte("prompt with " + NewCanary())
	if err := s.Write(context.Background(), "run", "recon", body); err != nil {
		t.Fatal(err)
	}
	// Nothing to inspect: the sink has no state by construction.
	if _, err := ScanTree(t.TempDir(), []string{string(body)}); err != nil {
		t.Fatal(err)
	}
}

func TestNewCanaryShape(t *testing.T) {
	re := regexp.MustCompile(`^ZDR-CANARY-[0-9a-f]{16}$`)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		c := NewCanary()
		if !re.MatchString(c) {
			t.Fatalf("canary %q", c)
		}
		if seen[c] {
			t.Fatalf("duplicate canary %q", c)
		}
		seen[c] = true
	}
}

func TestScanTree(t *testing.T) {
	root := t.TempDir()
	canary := NewCanary()
	other := NewCanary()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("clean.txt", "nothing to see")
	write("a/leak.log", "xx"+canary+"yy"+canary)
	write("a/b/both.bin", string([]byte{0, 1, 2})+other+canary)
	write("empty", "")
	if err := os.Symlink(filepath.Join(root, "a/leak.log"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		needles []string
		want    []Hit
		wantErr bool
	}{
		{"no needles", nil, nil, true},
		{"empty needle", []string{""}, nil, true},
		{"absent needle", []string{"ZDR-CANARY-0000000000000000"}, nil, false},
		{"canary", []string{canary}, []Hit{
			{filepath.Join(root, "a/b/both.bin"), 3 + int64(len(other)), canary},
			{filepath.Join(root, "a/leak.log"), 2, canary},
			{filepath.Join(root, "a/leak.log"), 2 + int64(len(canary)) + 2, canary},
		}, false},
		{"two needles", []string{other, canary}, []Hit{
			{filepath.Join(root, "a/b/both.bin"), 3, other},
			{filepath.Join(root, "a/b/both.bin"), 3 + int64(len(other)), canary},
			{filepath.Join(root, "a/leak.log"), 2, canary},
			{filepath.Join(root, "a/leak.log"), 2 + int64(len(canary)) + 2, canary},
		}, false},
		{"overlapping needle", []string{"CANARY"}, []Hit{
			{filepath.Join(root, "a/b/both.bin"), 7, "CANARY"},
			{filepath.Join(root, "a/b/both.bin"), 7 + int64(len(other)), "CANARY"},
			{filepath.Join(root, "a/leak.log"), 6, "CANARY"},
			{filepath.Join(root, "a/leak.log"), 6 + int64(len(canary)) + 2, "CANARY"},
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ScanTree(root, tc.needles)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("hit %d: got %+v want %+v", i, got[i], tc.want[i])
				}
			}
			for _, h := range got {
				if strings.HasSuffix(h.Path, "link") {
					t.Fatal("symlink was followed")
				}
			}
		})
	}
	if _, err := ScanTree(filepath.Join(root, "missing"), []string{"x"}); err == nil {
		t.Fatal("missing root should error")
	}
}
