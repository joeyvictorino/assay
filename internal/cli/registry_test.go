package cli

import (
	"bytes"
	"testing"
)

func TestUnknownSubcommandIsError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"nope"}, &out, &errb); code != ExitError {
		t.Fatalf("exit %d, want %d", code, ExitError)
	}
}

func TestVersion(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"version"}, &out, &errb); code != ExitPass {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("assay")) {
		t.Fatalf("unexpected output %q", out.String())
	}
}
