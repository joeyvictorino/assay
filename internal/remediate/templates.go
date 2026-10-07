package remediate

import (
	"github.com/joeyvictorino/assay/internal/model"
)

// template is the per-class remediation recipe. Placeholders {path},
// {param} and {method} are filled from the finding's Location.
type template struct {
	// Config is the human-readable configuration or code-level step.
	Config string
	// Target and Operator form the ModSecurity SecRule condition.
	Target   string
	Operator string
	// Phase is the ModSecurity phase (2 = request, 3 = response headers).
	Phase int
	// Action is deny (block) or pass (log only; used for response audits).
	Action string
	// Transforms are ModSecurity t: transformations applied before Operator.
	Transforms string
}

var templates = map[model.Class]template{
	model.ClassSQLi: {
		Config: "Replace string concatenation with a parameterized query (placeholders and bound arguments) for {method} {path} parameter {param}; reject input that fails a strict allowlist before it reaches the database; run the application's database role with SELECT-only grants where the endpoint only reads.",
		Target: "ARGS:{param}", Operator: "@detectSQLi", Phase: 2, Action: "deny", Transforms: "t:urlDecodeUni,t:lowercase",
	},
	model.ClassXSSReflected: {
		Config: "HTML-escape {param} before writing it into the page for {method} {path} (html/template or the framework's auto-escaping renderer); send Content-Type: text/html; charset=utf-8 and a Content-Security-Policy without 'unsafe-inline'.",
		Target: "ARGS:{param}", Operator: "@detectXSS", Phase: 2, Action: "deny", Transforms: "t:urlDecodeUni,t:htmlEntityDecode",
	},
	model.ClassXSSStored: {
		Config: "Escape {param} at render time wherever it is displayed (not only at {method} {path}); if rich text is required, sanitize with an allowlist HTML sanitizer at write time and store the sanitized form; add a Content-Security-Policy.",
		Target: "ARGS:{param}", Operator: "@detectXSS", Phase: 2, Action: "deny", Transforms: "t:urlDecodeUni,t:htmlEntityDecode",
	},
	model.ClassIDOR: {
		Config: "On {method} {path}, load the record and compare its owner to the authenticated principal (or scope the query by owner id) before returning it; return 404 for records outside the caller's scope; add an authorization test that reads another user's record and expects denial.",
		Target: "REQUEST_URI", Operator: "@rx ^{path_rx}", Phase: 2, Action: "pass", Transforms: "t:none",
	},
	model.ClassAuthMissing: {
		Config: "Require an authenticated session (and the appropriate role) on {method} {path}; register the check in shared middleware so new routes inherit it; return 401 without credentials and 403 for the wrong role.",
		Target: "REQUEST_URI", Operator: "@rx ^{path_rx}", Phase: 2, Action: "deny", Transforms: "t:none",
	},
	model.ClassAuthBypass: {
		Config: "Trace the authentication decision for {method} {path} to a single server-side check that cannot be influenced by client input; invalidate sessions created through the bypass; add a regression test.",
		Target: "REQUEST_URI", Operator: "@rx ^{path_rx}", Phase: 2, Action: "pass", Transforms: "t:none",
	},
	model.ClassDefaultCreds: {
		Config: "Remove or rotate the seeded default credential; force a password change on first login; enforce a minimum password policy and lock the account after repeated failures on {method} {path}.",
		Target: "ARGS:{param}", Operator: "@pmFromFile default-passwords.txt", Phase: 2, Action: "deny", Transforms: "t:none",
	},
	model.ClassOpenRedirect: {
		Config: "On {method} {path}, accept only relative paths or destinations from an explicit allowlist for {param}; resolve the target and compare host and scheme before redirecting; otherwise redirect to the site root.",
		Target: "ARGS:{param}", Operator: "@rx ^(?i)(https?:)?//", Phase: 2, Action: "deny", Transforms: "t:urlDecodeUni,t:lowercase",
	},
	model.ClassPathTraversal: {
		Config: "On {method} {path}, resolve {param} against the base directory, clean it, and refuse any result that does not stay inside the base (filepath.Rel without a leading ..); or map names to files through a fixed table.",
		Target: "ARGS:{param}", Operator: "@rx (?:\\.\\./|\\.\\.\\\\|%2e%2e)", Phase: 2, Action: "deny", Transforms: "t:urlDecodeUni,t:lowercase",
	},
	model.ClassSSRF: {
		Config: "On {method} {path}, resolve {param} and refuse loopback, link-local, private and metadata addresses after DNS resolution; allowlist destination hosts and schemes; disable redirects in the outbound client; apply a short timeout and response size cap.",
		Target: "ARGS:{param}", Operator: "@rx (?i)^(?:https?://)?(?:localhost|127\\.|0\\.0\\.0\\.0|10\\.|172\\.(?:1[6-9]|2\\d|3[01])\\.|192\\.168\\.|169\\.254\\.|\\[::1\\]|metadata)", Phase: 2, Action: "deny", Transforms: "t:urlDecodeUni,t:lowercase",
	},
	model.ClassInfoDisclosure: {
		Config: "Remove {method} {path} from production builds or require an administrator session; never serialize configuration values into responses; move secrets to an injected secret store so they cannot appear in any dump.",
		Target: "REQUEST_URI", Operator: "@rx ^{path_rx}", Phase: 2, Action: "deny", Transforms: "t:none",
	},
	model.ClassSecurityHeaders: {
		Config: "Set Content-Security-Policy, X-Content-Type-Options: nosniff, X-Frame-Options: DENY (or frame-ancestors), Referrer-Policy: no-referrer and, over TLS, Strict-Transport-Security on every response in shared middleware or at the reverse proxy.",
		Target: "&RESPONSE_HEADERS:Content-Security-Policy", Operator: "@eq 0", Phase: 3, Action: "pass", Transforms: "t:none",
	},
	model.ClassCORS: {
		Config: "Replace Access-Control-Allow-Origin: * with an explicit allowlist of origins compared exactly; never combine a wildcard or reflected origin with Access-Control-Allow-Credentials: true; omit CORS headers for same-origin APIs.",
		Target: "RESPONSE_HEADERS:Access-Control-Allow-Origin", Operator: "@streq *", Phase: 3, Action: "pass", Transforms: "t:none",
	},
	model.ClassMassAssignment: {
		Config: "Bind {method} {path} input to an explicit allowlist of fields (a dedicated request struct) and set privileged attributes such as role server-side only.",
		Target: "ARGS_NAMES", Operator: "@rx ^(?i)(role|is_admin|admin|isAdmin|permissions)$", Phase: 2, Action: "deny", Transforms: "t:none",
	},
	model.ClassJWTWeak: {
		Config: "Verify JWT signatures with an asymmetric algorithm pinned server-side (reject alg=none and algorithm switching); rotate the signing key; set short expiry and validate iss and aud on {method} {path}.",
		Target: "REQUEST_HEADERS:Authorization", Operator: "@rx (?i)^bearer\\s+eyJhbGciOiJub25l", Phase: 2, Action: "deny", Transforms: "t:none",
	},
	model.ClassRateLimit: {
		Config: "Rate-limit {method} {path} per account and per source address (for example 5 attempts per minute with exponential backoff); add account lockout or step-up verification after repeated failures.",
		Target: "IP:LOGIN_ATTEMPTS", Operator: "@gt 5", Phase: 2, Action: "deny", Transforms: "t:none",
	},
	model.ClassVerboseError: {
		Config: "Return a generic error page for {method} {path} and log the stack trace server-side only; disable debug mode in production; recover panics in middleware without echoing the panic value or stack.",
		Target: "RESPONSE_BODY", Operator: "@rx (?:goroutine \\d+ \\[|Traceback \\(most recent|Exception in thread|Stack trace:)", Phase: 4, Action: "deny", Transforms: "t:none",
	},
	model.ClassSensitiveFile: {
		Config: "Remove the file served at {path} from the web root; block dotfiles, backups and dumps at the server (deny .git, .env, *.sql, *.bak); store backups outside any served directory.",
		Target: "REQUEST_URI", Operator: "@rx (?i)\\.(?:sql|bak|env|git|old|zip|tar\\.gz)(?:$|\\?)", Phase: 2, Action: "deny", Transforms: "t:urlDecodeUni,t:lowercase",
	},
	model.ClassCSRF: {
		Config: "Require a per-session anti-CSRF token on state-changing requests to {path}; reject GET for state changes; set SameSite=Lax or Strict on the session cookie and verify Origin/Referer.",
		Target: "REQUEST_METHOD", Operator: "@rx ^(?:POST|PUT|PATCH|DELETE)$", Phase: 2, Action: "pass", Transforms: "t:none",
	},
	model.ClassOther: {
		Config: "Review {method} {path}: confirm the behavior against the application's threat model, add input validation and an authorization check, and record the decision.",
		Target: "REQUEST_URI", Operator: "@rx ^{path_rx}", Phase: 2, Action: "pass", Transforms: "t:none",
	},
}
