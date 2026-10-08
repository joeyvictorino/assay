package validate

// Checkers that need a legitimate lab session or send a write-shaped
// request. The writes are bounded to a field the checker owns and are
// undone afterwards; nothing here escalates, deletes or locks out.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"net/url"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/model"
)

// session is a logged-in lab account.
type session struct {
	headers map[string]string
	userID  string
	account labs.Account
}

// login signs in as a lab account, preferring a non-admin one. A nil
// session with a note means the lab cannot provide one (not an error); err
// is a gate denial or transport failure, which leaves the finding
// theorized.
func login(ctx context.Context, env Env) (*session, string, error) {
	auth, ok := env.Lab.(labs.Authenticator)
	if !ok {
		return nil, "lab cannot issue sessions", nil
	}
	acct, ok := pickUser(env.Lab.Accounts())
	if !ok {
		return nil, "lab lists no account", nil
	}
	hdrs, id, err := auth.Login(ctx, acct)
	if errors.Is(err, labs.ErrLoginRejected) {
		return nil, "lab rejected the documented account", nil
	}
	if err != nil {
		return nil, "", err
	}
	return &session{headers: hdrs, userID: id, account: acct}, "", nil
}

// pickUser returns the first non-admin account, else the first account.
func pickUser(accts []labs.Account) (labs.Account, bool) {
	var fallback *labs.Account
	for i := range accts {
		if accts[i].Username == "" {
			continue
		}
		if !strings.EqualFold(accts[i].Role, "admin") {
			return accts[i], true
		}
		if fallback == nil {
			fallback = &accts[i]
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return labs.Account{}, false
}

func withHeaders(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

var jsonHeaders = map[string]string{"Content-Type": "application/json"}

func isWrite(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH":
		return true
	}
	return false
}

func is2xx(r *httpx.Response) bool { return r.Status >= 200 && r.Status < 300 }

// ---- auth-bypass ----------------------------------------------------------

type authBypass struct{}

func (authBypass) Class() model.Class { return model.ClassAuthBypass }

// bypassVariant is one read-only request shape that a misconfigured front
// door may let through: a header a proxy trusts, or an alternate spelling
// of the path. No credentials are involved and nothing is written.
type bypassVariant struct {
	name  string
	shape func(u *url.URL) (target string, headers map[string]string)
}

var bypassVariants = []bypassVariant{
	{"x-forwarded-for-loopback", func(u *url.URL) (string, map[string]string) {
		return u.String(), map[string]string{"X-Forwarded-For": "127.0.0.1"}
	}},
	{"x-original-url", func(u *url.URL) (string, map[string]string) {
		root := *u
		root.Path, root.RawPath, root.RawQuery = "/", "", ""
		return root.String(), map[string]string{"X-Original-URL": u.RequestURI()}
	}},
	{"trailing-slash", func(u *url.URL) (string, map[string]string) {
		v := *u
		if !strings.HasSuffix(v.Path, "/") {
			v.Path += "/"
		}
		return v.String(), nil
	}},
}

// catchAllProbe returns a URL for a path that does not exist next to target.
func catchAllProbe(target string) string {
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	v := *u
	v.Path = strings.TrimRight(v.Path, "/") + "-" + newMarker() + "/"
	v.RawPath, v.RawQuery = "", ""
	return v.String()
}

// Check first confirms the resource is closed to an anonymous request, then
// tries each bypass shape. A header-bearing shape counts only when the same
// request without the header answers differently, so a server that ignores
// the header and serves its home page is not mistaken for a bypass.
func (authBypass) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	method := methodOf(f)
	target := targetURL(env, f, nil)
	denied, open := 0, 0
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.Exchange(ctx, method, target, nil, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		switch {
		case r.Status == 401 || r.Status == 403:
			denied++
		case r.Status == 200:
			open++
		}
	}
	if open == MinObservations {
		return result(model.StateTheorized, "resource answers 200 without credentials; that is auth-missing, not a bypass"), nil
	}
	if denied < MinObservations {
		return result(model.StateTheorized, "anonymous baseline is not consistently denied"), nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return model.ValidationResult{}, err
	}
	closed := 0
	for _, v := range bypassVariants {
		shaped, hdrs := v.shape(u)
		// The control is what the server says to the same shape without the
		// trick: the request without the header, or, for a header-less
		// shape, a nonexistent sibling path. A catch-all route that serves
		// one page for every unknown path answers both the same way, so it
		// is not mistaken for a bypass.
		var control *httpx.Response
		if len(hdrs) > 0 {
			control, err = env.HTTP.Exchange(ctx, method, shaped, nil, nil)
		} else {
			control, err = env.HTTP.Exchange(ctx, method, catchAllProbe(shaped), nil, nil)
		}
		if err != nil {
			return model.ValidationResult{}, err
		}
		var evs []model.Evidence
		blocked := 0
		for i := 0; i < MinObservations; i++ {
			r, err := env.HTTP.Exchange(ctx, method, shaped, hdrs, nil)
			if err != nil {
				return model.ValidationResult{}, err
			}
			sameAsControl := control != nil && control.Status == r.Status && control.BodyDigest == r.BodyDigest
			if r.Status == 200 && len(r.Body) > 0 && !sameAsControl {
				evs = append(evs, evidence("auth-bypass", r, v.name))
			} else {
				blocked++
			}
		}
		if len(evs) == MinObservations {
			return result(model.StateValidated, "protected resource returned 200 via "+v.name+" while the direct request is denied", evs...), nil
		}
		if blocked == MinObservations {
			closed++
		}
	}
	if closed == len(bypassVariants) {
		return result(model.StateRefuted, "direct request denied and every bypass shape denied"), nil
	}
	return result(model.StateTheorized, "inconsistent responses to bypass shapes"), nil
}

// ---- jwt-weak -------------------------------------------------------------

type jwtWeak struct{}

func (jwtWeak) Class() model.Class { return model.ClassJWTWeak }

// weakJWTSecrets is a short list of defaults seen in tutorials and starter
// templates. Verification is offline: the candidates never leave this
// process and never appear in evidence or notes (only the list index does).
var weakJWTSecrets = []string{"secret", "password", "changeme", "123456", "jwt_secret", "jwtsecret", "default", "key", "test", "admin"}

type jwt struct {
	header, payload, sig string
	alg                  string
}

// parseJWT accepts a compact JWS whose header decodes to JSON with an alg.
func parseJWT(token string) (*jwt, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, false
	}
	hb, err := b64url(parts[0])
	if err != nil {
		return nil, false
	}
	var h struct {
		Alg string `json:"alg"`
	}
	if json.Unmarshal(hb, &h) != nil || h.Alg == "" {
		return nil, false
	}
	if _, err := b64url(parts[1]); err != nil {
		return nil, false
	}
	return &jwt{header: parts[0], payload: parts[1], sig: parts[2], alg: h.Alg}, true
}

func b64url(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// weakSecretIndex returns the index of the list entry whose HMAC verifies
// the token, or -1 (also for non-HMAC algorithms).
func weakSecretIndex(t *jwt) int {
	var newHash func() hash.Hash
	switch strings.ToUpper(t.alg) {
	case "HS256":
		newHash = sha256.New
	case "HS384":
		newHash = sha512.New384
	case "HS512":
		newHash = sha512.New
	default:
		return -1
	}
	sig, err := b64url(t.sig)
	if err != nil {
		return -1
	}
	msg := []byte(t.header + "." + t.payload)
	for i, s := range weakJWTSecrets {
		m := hmac.New(newHash, []byte(s))
		m.Write(msg)
		if hmac.Equal(m.Sum(nil), sig) {
			return i
		}
	}
	return -1
}

// unsigned returns the same claims under alg "none" with an empty signature.
func (t *jwt) unsigned() string {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	return h + "." + t.payload + "."
}

func bearerToken(hdrs map[string]string) (string, bool) {
	for k, v := range hdrs {
		if strings.EqualFold(k, "Authorization") && strings.HasPrefix(v, "Bearer ") {
			return strings.TrimPrefix(v, "Bearer "), true
		}
	}
	return "", false
}

// Check logs in twice and inspects the tokens offline for a default-list
// HMAC secret, then (read-only) GETs the finding's path with the claims
// re-encoded under alg "none". Nothing is written and no secret is sent.
func (jwtWeak) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	auth, ok := env.Lab.(labs.Authenticator)
	if !ok {
		return result(model.StateTheorized, "lab cannot issue sessions"), nil
	}
	acct, ok := pickUser(env.Lab.Accounts())
	if !ok {
		return result(model.StateTheorized, "lab lists no account"), nil
	}
	var tokens []*jwt
	for i := 0; i < MinObservations; i++ {
		hdrs, _, err := auth.Login(ctx, acct)
		if errors.Is(err, labs.ErrLoginRejected) {
			return result(model.StateTheorized, "lab rejected the documented account"), nil
		}
		if err != nil {
			return model.ValidationResult{}, err
		}
		tok, ok := bearerToken(hdrs)
		if !ok {
			return result(model.StateTheorized, "session is not a bearer token"), nil
		}
		j, ok := parseJWT(tok)
		if !ok {
			return result(model.StateTheorized, "bearer token is not a JWT"), nil
		}
		tokens = append(tokens, j)
	}
	idx := weakSecretIndex(tokens[0])
	if idx >= 0 && weakSecretIndex(tokens[1]) == idx {
		var evs []model.Evidence
		for _, t := range tokens {
			evs = append(evs, model.Evidence{Kind: "tool-output", Marker: fmt.Sprintf("%s signature verifies with default-list secret #%d", strings.ToUpper(t.alg), idx)})
		}
		return result(model.StateValidated, fmt.Sprintf("token signature verifies with built-in default secret #%d (offline); alg:none not attempted", idx), evs...), nil
	}
	target := targetURL(env, f, nil)
	forged := map[string]string{"Authorization": "Bearer " + tokens[0].unsigned()}
	var evs []model.Evidence
	rejected := 0
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.Exchange(ctx, "GET", target, forged, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		switch {
		case r.Status == 200 && len(r.Body) > 0:
			evs = append(evs, evidence("http-exchange", r, "alg:none accepted"))
		case r.Status == 401 || r.Status == 403:
			rejected++
		}
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, "unsigned token (alg none) accepted on "+f.Location.PathTemplate, evs...), nil
	case rejected == MinObservations:
		return result(model.StateRefuted, "alg:none rejected and signature matches no default-list secret"), nil
	}
	return result(model.StateTheorized, "inconsistent handling of an unsigned token"), nil
}

// ---- mass-assignment ------------------------------------------------------

type massAssignment struct{}

func (massAssignment) Class() model.Class { return model.ClassMassAssignment }

// probeField is the client-supplied field the checker owns. It is never a
// privileged attribute: the check shows that the endpoint binds fields
// outside its schema, without changing a role or an owner.
const probeField = "assay_probe"

// Check sends the probe field with a marker as the authenticated user and
// looks for it echoed in the response or persisted on read-back. The probe
// field is overwritten with an empty value afterwards.
func (massAssignment) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	method := methodOf(f)
	if !isWrite(method) {
		return result(model.StateTheorized, "finding method is not state-changing"), nil
	}
	sess, note, err := login(ctx, env)
	if err != nil {
		return model.ValidationResult{}, err
	}
	if sess == nil {
		return result(model.StateTheorized, note), nil
	}
	target := targetURL(env, f, map[string]string{"id": sess.userID})
	hdrs := withHeaders(sess.headers, jsonHeaders)
	var evs []model.Evidence
	dropped, rejected, unobservable := 0, 0, 0
	for i := 0; i < MinObservations; i++ {
		marker := newMarker()
		body, _ := json.Marshal(map[string]string{probeField: marker})
		w, err := env.HTTP.Exchange(ctx, method, target, hdrs, body)
		if err != nil {
			return model.ValidationResult{}, err
		}
		switch {
		case !is2xx(w):
			rejected++
		case jsonHasValue(w.Body, probeField, marker):
			evs = append(evs, evidence("http-exchange", w, probeField+" echoed"))
		default:
			rb, err := env.HTTP.Exchange(ctx, "GET", target, sess.headers, nil)
			if err != nil {
				return model.ValidationResult{}, err
			}
			switch {
			case rb.Status == 200 && jsonHasValue(rb.Body, probeField, marker):
				evs = append(evs, evidence("http-exchange", rb, probeField+" persisted"))
			case rb.Status == 200 && isJSONObject(rb.Body):
				dropped++
			default:
				unobservable++
			}
		}
	}
	if rejected < MinObservations {
		body, _ := json.Marshal(map[string]string{probeField: ""})
		if _, err := env.HTTP.Exchange(ctx, method, target, hdrs, body); err != nil {
			return model.ValidationResult{}, err
		}
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, fmt.Sprintf("endpoint binds client-supplied field %q outside its schema; privileged field %q not exercised", probeField, f.Location.Param), evs...), nil
	case dropped == MinObservations:
		return result(model.StateRefuted, "unknown field dropped on write and absent on read-back"), nil
	case rejected == MinObservations:
		return result(model.StateTheorized, "write rejected; binding not observable"), nil
	case unobservable == MinObservations:
		return result(model.StateTheorized, "field not echoed and resource not readable; binding not observable"), nil
	}
	return result(model.StateTheorized, "inconsistent binding", evs...), nil
}

// jsonHasValue reports whether a JSON object holds key with exactly value,
// at the top level or inside any top-level object.
func jsonHasValue(body []byte, key, value string) bool {
	var obj map[string]any
	if json.Unmarshal(body, &obj) != nil {
		return false
	}
	if v, ok := obj[key]; ok && fmt.Sprint(v) == value {
		return true
	}
	for _, v := range obj {
		if m, ok := v.(map[string]any); ok {
			if x, ok := m[key]; ok && fmt.Sprint(x) == value {
				return true
			}
		}
	}
	return false
}

func isJSONObject(body []byte) bool {
	var obj map[string]any
	return json.Unmarshal(body, &obj) == nil
}

// ---- xss-stored -----------------------------------------------------------

type xssStored struct{}

func (xssStored) Class() model.Class { return model.ClassXSSStored }

// Check stores "<b>assay-<hex></b>" in the finding's field as the
// authenticated user, reads it back, and then overwrites the field with an
// empty value. The payload is a formatting tag, never a script. Read-back
// locations are, in order: Env.Markers["stored-readback"] (a path template
// with {id}), the write's Location header, path-like string fields in the
// write's JSON response, and the written path itself.
func (xssStored) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	param := f.Location.Param
	if param == "" {
		return result(model.StateTheorized, "finding names no parameter"), nil
	}
	method := methodOf(f)
	if !isWrite(method) {
		return result(model.StateTheorized, "finding method is not state-changing"), nil
	}
	sess, note, err := login(ctx, env)
	if err != nil {
		return model.ValidationResult{}, err
	}
	if sess == nil {
		return result(model.StateTheorized, note), nil
	}
	target := targetURL(env, f, map[string]string{"id": sess.userID})
	hdrs := withHeaders(sess.headers, jsonHeaders)
	var evs []model.Evidence
	escaped, missing, rejected := 0, 0, 0
	wrote := false
	for i := 0; i < MinObservations; i++ {
		marker := newMarker()
		payload := "<b>" + marker + "</b>"
		body, _ := json.Marshal(map[string]string{param: payload})
		w, err := env.HTTP.Exchange(ctx, method, target, hdrs, body)
		if err != nil {
			return model.ValidationResult{}, err
		}
		if !is2xx(w) {
			rejected++
			continue
		}
		wrote = true
		found := false
		for _, rb := range readbackURLs(env, sess, target, w) {
			r, err := env.HTTP.Exchange(ctx, "GET", rb, sess.headers, nil)
			if err != nil {
				return model.ValidationResult{}, err
			}
			if r.Status != 200 {
				continue
			}
			s := string(r.Body)
			switch {
			case strings.Contains(s, payload) && isHTML(r.Headers):
				evs = append(evs, evidence("marker-reflected", r, marker))
				found = true
			case strings.Contains(s, marker):
				escaped++
				found = true
			}
			if found {
				break
			}
		}
		if !found {
			missing++
		}
	}
	if wrote {
		if err := neutralize(ctx, env, method, target, hdrs, param); err != nil {
			return model.ValidationResult{}, err
		}
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, "benign tag stored via "+param+" and rendered unescaped on read-back", evs...), nil
	case escaped == MinObservations:
		return result(model.StateRefuted, "stored value is encoded on read-back"), nil
	case rejected == MinObservations:
		return result(model.StateTheorized, "write rejected"), nil
	case missing == MinObservations:
		return result(model.StateTheorized, "marker not found at any read-back location"), nil
	}
	return result(model.StateTheorized, "inconsistent read-back", evs...), nil
}

// neutralize overwrites the field with an empty value, falling back to a
// plain placeholder when the endpoint rejects empty input.
func neutralize(ctx context.Context, env Env, method, target string, hdrs map[string]string, param string) error {
	for _, v := range []string{"", "-"} {
		body, _ := json.Marshal(map[string]string{param: v})
		r, err := env.HTTP.Exchange(ctx, method, target, hdrs, body)
		if err != nil {
			return err
		}
		if is2xx(r) {
			return nil
		}
	}
	return nil
}

// readbackURLs lists where the stored value may be rendered, same host only.
func readbackURLs(env Env, sess *session, target string, w *httpx.Response) []string {
	base, err := url.Parse(target)
	if err != nil {
		return []string{target}
	}
	var out []string
	add := func(ref string) {
		u, err := base.Parse(ref)
		if err != nil || !strings.EqualFold(u.Host, base.Host) {
			return
		}
		s := u.String()
		for _, have := range out {
			if have == s {
				return
			}
		}
		out = append(out, s)
	}
	if tpl := env.Markers["stored-readback"]; tpl != "" {
		add(targetURL(env, model.Finding{Location: model.Location{PathTemplate: tpl}}, map[string]string{"id": sess.userID}))
	}
	if loc := w.Headers["Location"]; loc != "" {
		add(loc)
	}
	var obj map[string]any
	if json.Unmarshal(w.Body, &obj) == nil {
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if s, ok := obj[k].(string); ok && strings.HasPrefix(s, "/") && !strings.ContainsAny(s, " \t\n") {
				add(s)
			}
		}
	}
	add(target)
	return out
}
