package scope

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

var now = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

const roeHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func okVerify(map[string]any) (string, string, error) { return "test-key", "", nil }

func doc(extra string) string {
	return `version: "1"
authorized_by: "lab-owner"
issued_at: "2026-10-01T00:00:00Z"
expires_at: "2026-12-31T00:00:00Z"
rules_of_engagement_sha256: "` + roeHash + `"
targets:
  - host: 127.0.0.1
    ports: [9000, 8080]
    schemes: [http]
  - host: localhost
    ports: [3000]
    schemes: [http, https]
  - host: api.example.test
    ports: [443]
    schemes: [https]
    path_prefixes: ["/v1/", "/health"]
  - host: "*.wild.test"
    ports: [80]
    schemes: [http]
  - host: "[::1]"
    ports: [9000]
    schemes: [http]
methods: [GET, POST, HEAD]
rate_limit_per_minute: 120
max_body_bytes: 1048576
` + extra
}

func signed() string { return doc("signature: {key_id: test-key, sig: AAAA}\n") }

func mustGate(t *testing.T, data string) *Gate {
	t.Helper()
	g, err := Parse([]byte(data), now, okVerify)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return g
}

func TestLoadFailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		data   string
		verify Verifier
		now    time.Time
		code   string
	}{
		{"nil verifier", signed(), nil, now, CodeVerifierMissing},
		{"no signature", doc(""), okVerify, now, CodeSignatureInvalid},
		{"bad signature", signed(), func(map[string]any) (string, string, error) {
			return "", "SIG_MISMATCH", errors.New("mismatch")
		}, now, CodeSignatureInvalid},
		{"empty key id", signed(), func(map[string]any) (string, string, error) { return "", "", nil }, now, CodeSignatureInvalid},
		{"unparseable", "{{{", okVerify, now, CodeParseFailed},
		{"empty", "", okVerify, now, CodeParseFailed},
		{"expired", signed(), okVerify, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), CodeExpired},
		{"not yet valid", signed(), okVerify, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), CodeNotYetValid},
		{"zero targets", strings.Replace(signed(), "targets:", "targets: []\nignored:", 1), okVerify, now, CodeNoTargets},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := Parse([]byte(tc.data), tc.now, tc.verify)
			if g != nil || err == nil {
				t.Fatalf("expected failure, got gate=%v err=%v", g, err)
			}
			if !IsCode(err, tc.code) {
				t.Fatalf("code: got %v want %s", err, tc.code)
			}
		})
	}
}

func TestZeroTargets(t *testing.T) {
	d := `version: "1"
authorized_by: a
issued_at: "2026-10-01T00:00:00Z"
expires_at: "2026-12-31T00:00:00Z"
rules_of_engagement_sha256: "` + roeHash + `"
targets: []
methods: [GET]
rate_limit_per_minute: 10
max_body_bytes: 10
signature: x
`
	_, err := Parse([]byte(d), now, okVerify)
	if !IsCode(err, CodeNoTargets) {
		t.Fatalf("got %v", err)
	}
}

func TestMissingFields(t *testing.T) {
	for _, field := range []string{"version", "authorized_by", "issued_at", "expires_at", "rules_of_engagement_sha256", "methods", "rate_limit_per_minute", "max_body_bytes"} {
		lines := strings.Split(signed(), "\n")
		var kept []string
		for _, l := range lines {
			if strings.HasPrefix(l, field+":") {
				continue
			}
			kept = append(kept, l)
		}
		_, err := Parse([]byte(strings.Join(kept, "\n")), now, okVerify)
		if !IsCode(err, CodeMissingField) {
			t.Errorf("%s: got %v", field, err)
		}
	}
}

func TestJSONDocumentLoadsAndFingerprintStable(t *testing.T) {
	js := `{"version":"1","authorized_by":"a","issued_at":"2026-10-01T00:00:00Z","expires_at":"2026-12-31T00:00:00Z",
"rules_of_engagement_sha256":"` + roeHash + `","targets":[{"host":"127.0.0.1","ports":[9000],"schemes":["http"]}],
"methods":["GET"],"rate_limit_per_minute":60,"max_body_bytes":1024,"signature":{"sig":"x"}}`
	dir := t.TempDir()
	p := filepath.Join(dir, "scope.json")
	if err := os.WriteFile(p, []byte(js), 0o600); err != nil {
		t.Fatal(err)
	}
	g1, err := Load(p, now, okVerify)
	if err != nil {
		t.Fatal(err)
	}
	// Same content with a different signature value: same fingerprint.
	g2 := mustGate(t, strings.Replace(js, `"sig":"x"`, `"sig":"y"`, 1))
	if g1.Fingerprint() != g2.Fingerprint() || len(g1.Fingerprint()) != 64 {
		t.Fatalf("fingerprint mismatch %s vs %s", g1.Fingerprint(), g2.Fingerprint())
	}
	g3 := mustGate(t, strings.Replace(js, `"ports":[9000]`, `"ports":[9001]`, 1))
	if g3.Fingerprint() == g1.Fingerprint() {
		t.Fatal("fingerprint must change with content")
	}
	if g1.KeyID() != "test-key" {
		t.Fatalf("key id %q", g1.KeyID())
	}
	if _, err := Load(filepath.Join(dir, "missing.yaml"), now, okVerify); !IsCode(err, CodeReadFailed) {
		t.Fatalf("missing file: %v", err)
	}
}

func TestAllowMatrix(t *testing.T) {
	g := mustGate(t, signed())
	ctx := context.Background()
	cases := []struct {
		url   string
		allow bool
		want  string // substring of rationale on deny
	}{
		{"http://127.0.0.1:9000/api/search?q=x", true, ""},
		{"http://127.0.0.1:8080/", true, ""},
		{"http://127.0.0.1/", false, "port 80"},
		{"https://127.0.0.1:9000/", false, "scheme https"},
		{"http://127.0.0.1:9001/", false, "port 9001"},
		{"http://localhost:3000/x", true, ""},
		{"https://localhost:3000/x", true, ""},
		{"http://localhost:9000/", false, "port 9000"},  // localhost is not 127.0.0.1
		{"http://127.0.0.1:3000/", false, "port 3000"},  // 127.0.0.1 is not localhost
		{"http://127.000.000.001:9000/", false, "host"}, // odd literal is not normalized to an authorized host
		{"http://[::1]:9000/", true, ""},
		{"http://[0:0:0:0:0:0:0:1]:9000/", true, ""},
		{"http://[::ffff:127.0.0.1]:9000/", true, ""},
		{"https://api.example.test/v1/users", true, ""},
		{"https://api.example.test/health", true, ""},
		{"https://api.example.test/", false, "outside prefixes"},
		{"https://api.example.test/v2/x", false, "outside prefixes"},
		{"https://api.example.test/v1/../admin", false, "traversal"},
		{"https://api.example.test/v1/%2e%2e/admin", false, "traversal"},
		{"https://api.example.test/v1/..%2fadmin", false, "traversal"},
		{"https://api.example.test/v1/%252e%252e/admin", false, "traversal"},
		{"https://api.example.test/v1/./x", false, "traversal"},
		{"https://api.example.test:443/v1/x", true, ""},
		{"http://api.example.test:443/v1/x", false, "scheme http"},
		{"http://a.wild.test/", true, ""},
		{"http://a.b.wild.test/", true, ""},
		{"http://wild.test/", false, "host"},
		{"http://evilwild.test/", false, "host"},
		{"http://user:pw@127.0.0.1:9000/", false, "userinfo"},
		{"http://user@127.0.0.1:9000/", false, "userinfo"},
		{"ftp://127.0.0.1:9000/", false, "scheme"},
		{"file:///etc/passwd", false, "scheme"},
		{"/relative", false, "absolute"},
		{"http://", false, "host"},
		{"http://127.0.0.1.:9000/", false, "trailing dot"},
		{"http://LOCALHOST:3000/", true, ""},
		{"http://127.0.0.1:99999/", false, "port"},
		{"http://evil.test@127.0.0.1:9000/", false, "userinfo"},
		{"http://127.0.0.1:9000#@evil.test/", true, ""},
		{"::not a url", false, ""},
	}
	for _, tc := range cases {
		d := g.Allow(ctx, tc.url)
		got := d.Effect == model.EffectAllow
		if got != tc.allow {
			t.Errorf("%s: effect %s (%s: %s), want allow=%v", tc.url, d.Effect, d.Reason, d.Rationale, tc.allow)
			continue
		}
		if !tc.allow {
			if d.Reason != ReasonOutOfScope {
				t.Errorf("%s: reason %s", tc.url, d.Reason)
			}
			if !strings.Contains(d.Rationale, tc.want) {
				t.Errorf("%s: rationale %q lacks %q", tc.url, d.Rationale, tc.want)
			}
		} else {
			if d.Reason != ReasonInScope || len(d.MatchedRules) != 1 || d.PolicyHash != g.Fingerprint() {
				t.Errorf("%s: allow decision malformed: %+v", tc.url, d)
			}
		}
	}
}

func TestNilGateDenies(t *testing.T) {
	var g *Gate
	if d := g.Allow(context.Background(), "http://127.0.0.1:9000/"); d.Effect != model.EffectDeny {
		t.Fatal("nil gate must deny")
	}
}

func TestAllowMethod(t *testing.T) {
	g := mustGate(t, signed())
	if d := g.AllowMethod("get"); d.Effect != model.EffectAllow {
		t.Fatalf("GET: %+v", d)
	}
	if d := g.AllowMethod("DELETE"); d.Effect != model.EffectDeny || d.Reason != ReasonMethodDenied {
		t.Fatalf("DELETE: %+v", d)
	}
}

func TestInvalidTargetHosts(t *testing.T) {
	for _, h := range []string{`"*"`, `"foo.*"`, `"a*.b.test"`, `"http://x"`, `"x:80"`} {
		d := strings.Replace(signed(), "host: 127.0.0.1", "host: "+h, 1)
		if _, err := Parse([]byte(d), now, okVerify); !IsCode(err, CodeInvalidField) {
			t.Errorf("%s: got %v", h, err)
		}
	}
}

func TestTakeRateLimits(t *testing.T) {
	g := mustGate(t, strings.Replace(signed(), "rate_limit_per_minute: 120", "rate_limit_per_minute: 3", 1))
	clock := now
	g.SetClock(func() time.Time { return clock })
	for i := 0; i < 3; i++ {
		if err := g.Take(); err != nil {
			t.Fatalf("take %d: %v", i, err)
		}
	}
	err := g.Take()
	if !errors.Is(err, ErrRateLimited) || !IsCode(err, CodeRateLimited) {
		t.Fatalf("expected RATE_LIMITED, got %v", err)
	}
	clock = clock.Add(20 * time.Second) // 3/min => one token per 20s
	if err := g.Take(); err != nil {
		t.Fatalf("after refill: %v", err)
	}
	if err := g.Take(); err == nil {
		t.Fatal("bucket should be empty again")
	}
	clock = clock.Add(time.Hour)
	for i := 0; i < 3; i++ {
		if err := g.Take(); err != nil {
			t.Fatalf("burst capped refill %d: %v", i, err)
		}
	}
	if err := g.Take(); err == nil {
		t.Fatal("burst must not exceed rate_limit_per_minute")
	}
}

func TestTargetsAndDocument(t *testing.T) {
	g := mustGate(t, signed())
	ts := g.Targets()
	if len(ts) != 6 || ts[0] != "*.wild.test:80" || ts[3] != "[::1]:9000" {
		t.Fatalf("targets %v", ts)
	}
	if g.Document().RateLimitPerMinute != 120 || g.MaxBodyBytes() != 1048576 {
		t.Fatal("document accessor")
	}
}
