package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/joeyvictorino/assay/internal/provider/fake"
	"github.com/joeyvictorino/assay/internal/zdr"
)

// The whole pipeline, end to end, with a canary planted where transcripts
// live: in the lab's responses (which reach the model as tool results), in
// the scripted model's text, and in the harness's own prompt text. After the
// run, no byte under the sandboxed HOME, TMPDIR, working directory, results
// directory or audit directory may contain any of them, while the audit
// export must still prove the calls happened by holding their digests.
func TestCanaryAndPromptTextNeverReachDisk(t *testing.T) {
	canary := zdr.NewCanary()
	f := newPipelineFixture(t, canary)
	fakeProviderFn = func(name, baseURL string) *fake.Provider {
		return fake.New(fakeSteps(baseURL), fake.Options{Name: name, Canary: canary})
	}
	if code, stderr := f.run(t); code != ExitPass {
		t.Fatalf("exit %d\n%s", code, stderr)
	}

	// Positive control: the scanner must find a string that is on disk, or a
	// clean result would prove nothing.
	if ctl, err := zdr.ScanTree(f.root, []string{f.opts.runID}); err != nil || len(ctl) == 0 {
		t.Fatalf("scanner positive control failed: hits=%d err=%v", len(ctl), err)
	}

	needles := []string{canary, "You are assessing an intentionally vulnerable", "Enumerate its visible endpoints",
		"You are reviewing a security assessment", "Covered the visible surface of the root page"}
	hits, err := zdr.ScanTree(f.root, needles)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("transcript text reached disk: %+v", hits)
	}

	// Digests, not bodies: the decrypted export shows the model and tool
	// calls as sha256 values.
	export, err := os.ReadFile(filepath.Join(f.opts.out, f.opts.runID, "audit.export.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	digest := regexp.MustCompile(`[0-9a-f]{64}`)
	if len(digest.FindAll(export, -1)) < 10 {
		t.Fatalf("export holds too few digests to show the calls happened")
	}
	for _, kind := range []string{`"kind":"model_call"`, `"kind":"tool_call"`, `"kind":"policy_decision"`, `"kind":"gate_decision"`} {
		if !bytes.Contains(export, []byte(kind)) {
			t.Errorf("audit export lacks %s", kind)
		}
	}
}
