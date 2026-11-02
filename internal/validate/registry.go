package validate

import "github.com/joeyvictorino/assay/internal/model"

// CheckerInfo describes a registered checker for the --list-checkers output.
type CheckerInfo struct {
	Class       model.Class
	Description string
	Validates   string
}

var checkerDescriptions = map[model.Class]CheckerInfo{
	model.ClassSQLi:            {model.ClassSQLi, "SQL injection", "Sends parameterised payloads and checks for error signatures or tautology-based response changes"},
	model.ClassXSSReflected:    {model.ClassXSSReflected, "Reflected cross-site scripting", "Injects a unique marker in reflected parameters and checks whether it appears unescaped in the HTML response"},
	model.ClassXSSStored:       {model.ClassXSSStored, "Stored cross-site scripting", "Submits a marker via one endpoint and retrieves it from another to detect persistence of unescaped content"},
	model.ClassIDOR:            {model.ClassIDOR, "Insecure direct object reference", "Requests a resource belonging to another user and checks for unauthorised disclosure"},
	model.ClassAuthMissing:     {model.ClassAuthMissing, "Missing authentication", "Sends unauthenticated requests to protected endpoints and checks for 200-level responses"},
	model.ClassAuthBypass:      {model.ClassAuthBypass, "Authentication bypass", "Attempts header, path-traversal and encoding tricks to reach authenticated endpoints without credentials"},
	model.ClassDefaultCreds:    {model.ClassDefaultCreds, "Default credentials", "Tries well-known default username/password pairs against login endpoints"},
	model.ClassOpenRedirect:    {model.ClassOpenRedirect, "Open redirect", "Supplies an external domain in redirect parameters and checks the Location header"},
	model.ClassPathTraversal:   {model.ClassPathTraversal, "Path traversal", "Requests ../-prefixed paths and checks for expected file markers in the response"},
	model.ClassSSRF:            {model.ClassSSRF, "Server-side request forgery", "Injects internal URLs (localhost, metadata endpoints) and checks for out-of-band or in-band disclosure"},
	model.ClassInfoDisclosure:  {model.ClassInfoDisclosure, "Information disclosure", "Checks for planted markers, stack traces, or internal identifiers in public responses"},
	model.ClassSecurityHeaders: {model.ClassSecurityHeaders, "Missing security headers", "Checks for X-Frame-Options, Content-Security-Policy, X-Content-Type-Options and similar headers"},
	model.ClassCORS:            {model.ClassCORS, "CORS misconfiguration", "Sends requests with Origin headers and checks whether the server reflects arbitrary origins in Access-Control-Allow-Origin"},
	model.ClassMassAssignment:  {model.ClassMassAssignment, "Mass assignment", "Sends extra fields (role, admin, is_staff) in update requests and checks if they are persisted"},
	model.ClassJWTWeak:         {model.ClassJWTWeak, "Weak JWT validation", "Tests for alg:none, key confusion and unsigned-token acceptance"},
	model.ClassRateLimit:       {model.ClassRateLimit, "Missing rate limiting", "Sends a burst of identical requests and checks whether any are throttled (429) or slowed"},
	model.ClassVerboseError:    {model.ClassVerboseError, "Verbose error messages", "Triggers errors and checks for stack traces, SQL fragments, or framework internals in the response"},
	model.ClassSensitiveFile:   {model.ClassSensitiveFile, "Sensitive file exposure", "Requests well-known paths (.env, .git/config, etc.) and checks for planted markers or known signatures"},
	model.ClassCSRF:            {model.ClassCSRF, "Cross-site request forgery", "Submits state-changing requests without CSRF tokens and checks whether the server accepts them"},
}

// CheckerInfos returns metadata for every registered checker in AllClasses
// order. Classes without a checker are omitted.
func CheckerInfos() []CheckerInfo {
	checkers := Checkers()
	out := make([]CheckerInfo, 0, len(checkers))
	for _, c := range checkers {
		if info, ok := checkerDescriptions[c.Class()]; ok {
			out = append(out, info)
		} else {
			out = append(out, CheckerInfo{
				Class:       c.Class(),
				Description: string(c.Class()),
				Validates:   "(no description)",
			})
		}
	}
	return out
}
