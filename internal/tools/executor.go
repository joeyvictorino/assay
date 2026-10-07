package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/finding"
	"github.com/joeyvictorino/assay/internal/model"
)

// MaxBodyBytes caps the body returned to a model.
const MaxBodyBytes = 8 * 1024

// HTTPFunc performs one HTTP exchange. It is the only path to the network;
// the lead injects the gated client (internal/scope, S3).
type HTTPFunc func(ctx context.Context, method, url string, headers map[string]string, body []byte) (status int, respHeaders map[string]string, respBody []byte, err error)

// Env is what a tool may touch while executing.
type Env struct {
	HTTP HTTPFunc
	// Gate authorizes every target URL. nil fails closed.
	Gate model.Gate
	// Lab and RunID stamp reported findings.
	Lab   string
	RunID string
	// Model identifies who is reporting.
	Model model.ModelRef
	// Report receives each normalized finding.
	Report func(model.Finding)
	// Sanitize redacts text before it is returned to the model or placed
	// in a finding summary. nil means identity.
	Sanitize func(string) string
	// Delegate runs a sub-objective; nil means delegation is unavailable.
	Delegate func(ctx context.Context, agent, objective string) (string, error)
	// Score records a score_task call; nil discards it.
	Score func(score int, critique string)
	// Now supplies timestamps; nil means time.Now.
	Now func() time.Time
}

func (e Env) sanitize(s string) string {
	if e.Sanitize == nil {
		return s
	}
	return e.Sanitize(s)
}

func (e Env) now() time.Time {
	if e.Now == nil {
		return time.Now().UTC()
	}
	return e.Now()
}

// Executor runs tool calls. A non-nil error means the harness itself
// failed; a model-visible failure is a ToolResult with IsError set.
type Executor interface {
	Execute(ctx context.Context, call model.ToolCall, env Env) (model.ToolResult, error)
}

// ErrUnknownTool is wrapped in the error result for an unregistered name.
var ErrUnknownTool = errors.New("tools: unknown tool")

type executor struct{}

// NewExecutor returns the default executor for the bundled tools.
func NewExecutor() Executor { return executor{} }

func (executor) Execute(ctx context.Context, call model.ToolCall, env Env) (model.ToolResult, error) {
	args := call.Args
	if args == nil {
		args = map[string]any{}
	}
	var (
		out any
		err error
	)
	switch call.Name {
	case "http_get":
		out, err = httpGet(ctx, args, env)
	case "http_post":
		out, err = httpPost(ctx, args, env)
	case "inspect_headers":
		out, err = inspectHeaders(ctx, args, env)
	case "login_as":
		out, err = loginAs(ctx, args, env)
	case "report_finding":
		out, err = reportFinding(args, env)
	case "score_task":
		out, err = scoreTask(args, env)
	case "delegate":
		out, err = delegate(ctx, args, env)
	default:
		return errResult(call.ID, fmt.Sprintf("%v: %q", ErrUnknownTool, call.Name)), nil
	}
	if err != nil {
		return errResult(call.ID, env.sanitize(err.Error())), nil
	}
	b, merr := json.Marshal(out)
	if merr != nil {
		return model.ToolResult{}, fmt.Errorf("tools: encode result: %w", merr)
	}
	return model.ToolResult{ToolCallID: call.ID, Content: string(b)}, nil
}

func errResult(id, msg string) model.ToolResult {
	return model.ToolResult{ToolCallID: id, Content: msg, IsError: true}
}

// argErr is a model-visible argument error.
type argErr string

func (e argErr) Error() string { return string(e) }

func stringArg(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", argErr("missing argument: " + key)
	}
	s, ok := v.(string)
	if !ok {
		return "", argErr("argument " + key + " must be a string")
	}
	return s, nil
}

func stringSliceArg(args map[string]any, key string) ([]string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, nil
	}
	switch s := v.(type) {
	case []string:
		return s, nil
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			str, ok := e.(string)
			if !ok {
				return nil, argErr("argument " + key + " must be an array of strings")
			}
			out = append(out, str)
		}
		return out, nil
	}
	return nil, argErr("argument " + key + " must be an array of strings")
}

func checkURL(ctx context.Context, env Env, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return argErr("url must be absolute with scheme and host")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return argErr("url scheme must be http or https")
	}
	if env.Gate == nil {
		return argErr("gate denied: no scope gate configured")
	}
	d := env.Gate.Allow(ctx, raw)
	if d.Effect != model.EffectAllow {
		reason := d.Reason
		if reason == "" {
			reason = "out of scope"
		}
		return argErr("gate denied: " + reason)
	}
	return nil
}

// exchange is the bounded, sanitized view of an HTTP response.
type exchange struct {
	Status        int               `json:"status"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body,omitempty"`
	BodyBytes     int               `json:"body_bytes"`
	BodyTruncated bool              `json:"body_truncated,omitempty"`
}

func doHTTP(ctx context.Context, env Env, method, target string, headers map[string]string, body []byte, includeBody bool) (exchange, error) {
	if err := checkURL(ctx, env, target); err != nil {
		return exchange{}, err
	}
	if env.HTTP == nil {
		return exchange{}, errors.New("http client not configured")
	}
	status, respHeaders, respBody, err := env.HTTP(ctx, method, target, headers, body)
	if err != nil {
		return exchange{}, argErr("request failed: " + err.Error())
	}
	ex := exchange{Status: status, Headers: sanitizeHeaders(respHeaders, env), BodyBytes: len(respBody)}
	if includeBody {
		b := respBody
		if len(b) > MaxBodyBytes {
			b = b[:MaxBodyBytes]
			ex.BodyTruncated = true
		}
		ex.Body = env.sanitize(string(b))
	}
	return ex, nil
}

// sanitizeHeaders lower-cases names, strips cookie and credential values
// (keeping cookie attributes so flags remain inspectable) and runs the
// redaction hook over everything else.
func sanitizeHeaders(h map[string]string, env Env) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		lk := strings.ToLower(k)
		switch lk {
		case "set-cookie":
			out[lk] = redactCookie(v)
		case "authorization", "proxy-authorization", "x-api-key", "cookie":
			out[lk] = "[redacted]"
		default:
			out[lk] = env.sanitize(v)
		}
	}
	return out
}

func redactCookie(v string) string {
	parts := strings.Split(v, ";")
	nameVal := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
	first := nameVal[0] + "=[redacted]"
	rest := parts[1:]
	for i := range rest {
		rest[i] = strings.TrimSpace(rest[i])
	}
	if len(rest) == 0 {
		return first
	}
	return first + "; " + strings.Join(rest, "; ")
}

func httpGet(ctx context.Context, args map[string]any, env Env) (any, error) {
	target, err := stringArg(args, "url")
	if err != nil {
		return nil, err
	}
	return doHTTP(ctx, env, "GET", target, nil, nil, true)
}

func httpPost(ctx context.Context, args map[string]any, env Env) (any, error) {
	target, err := stringArg(args, "url")
	if err != nil {
		return nil, err
	}
	ct, err := stringArg(args, "content_type")
	if err != nil {
		return nil, err
	}
	body, err := stringArg(args, "body")
	if err != nil {
		return nil, err
	}
	return doHTTP(ctx, env, "POST", target, map[string]string{"Content-Type": ct}, []byte(body), true)
}

func inspectHeaders(ctx context.Context, args map[string]any, env Env) (any, error) {
	target, err := stringArg(args, "url")
	if err != nil {
		return nil, err
	}
	return doHTTP(ctx, env, "GET", target, nil, nil, false)
}

type loginResult struct {
	Status        int    `json:"status"`
	SessionCookie bool   `json:"session_cookie_set"`
	Location      string `json:"location,omitempty"`
	Body          string `json:"body,omitempty"`
	BodyTruncated bool   `json:"body_truncated,omitempty"`
}

func loginAs(ctx context.Context, args map[string]any, env Env) (any, error) {
	target, err := stringArg(args, "url")
	if err != nil {
		return nil, err
	}
	user, err := stringArg(args, "username")
	if err != nil {
		return nil, err
	}
	pass, err := stringArg(args, "password")
	if err != nil {
		return nil, err
	}
	form := url.Values{"username": {user}, "password": {pass}}.Encode()
	ex, err := doHTTP(ctx, env, "POST", target, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, []byte(form), true)
	if err != nil {
		return nil, err
	}
	_, cookie := ex.Headers["set-cookie"]
	body := ex.Body
	// Never echo the submitted secret back, whatever the server did.
	if pass != "" {
		body = strings.ReplaceAll(body, pass, "[redacted]")
	}
	return loginResult{Status: ex.Status, SessionCookie: cookie, Location: ex.Headers["location"], Body: body, BodyTruncated: ex.BodyTruncated}, nil
}

var allowedMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}

type reportResult struct {
	ID       string `json:"id"`
	DedupKey string `json:"dedup_key"`
	State    string `json:"state"`
}

func reportFinding(args map[string]any, env Env) (any, error) {
	classRaw, err := stringArg(args, "class")
	if err != nil {
		return nil, err
	}
	class, ok := finding.ValidClass(classRaw)
	if !ok {
		return nil, argErr("class must be one of the fixed taxonomy values")
	}
	method, err := stringArg(args, "method")
	if err != nil {
		return nil, err
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if !allowedMethods[method] {
		return nil, argErr("method must be one of GET, POST, PUT, DELETE, PATCH")
	}
	path, err := stringArg(args, "path")
	if err != nil {
		return nil, err
	}
	param, _ := stringArg(args, "param")
	sevRaw, err := stringArg(args, "severity")
	if err != nil {
		return nil, err
	}
	sev, ok := finding.ValidSeverity(sevRaw)
	if !ok {
		return nil, argErr("severity must be one of info, low, medium, high, critical")
	}
	summary, err := stringArg(args, "summary")
	if err != nil {
		return nil, err
	}
	ids, err := stringSliceArg(args, "evidence_tool_call_ids")
	if err != nil {
		return nil, err
	}
	sort.Strings(ids)
	f, err := finding.NewFinding(finding.Params{
		RunID: env.RunID, Lab: env.Lab, Class: class, Method: method, Path: path, Param: param,
		Severity: sev, Summary: env.sanitize(summary), Model: env.Model, ToolCallIDs: ids, Now: env.now(),
	})
	if err != nil {
		return nil, argErr(err.Error())
	}
	if env.Report != nil {
		env.Report(f)
	}
	return reportResult{ID: f.ID, DedupKey: f.DedupKey, State: string(f.State)}, nil
}

type scoreResult struct {
	Recorded bool `json:"recorded"`
	Score    int  `json:"score"`
}

func scoreTask(args map[string]any, env Env) (any, error) {
	v, ok := args["score"]
	if !ok {
		return nil, argErr("missing argument: score")
	}
	var score int
	switch n := v.(type) {
	case float64:
		if n != float64(int(n)) {
			return nil, argErr("score must be an integer")
		}
		score = int(n)
	case int:
		score = n
	case int64:
		score = int(n)
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return nil, argErr("score must be an integer")
		}
		score = int(i)
	default:
		return nil, argErr("score must be an integer")
	}
	if score < 0 || score > 10 {
		return nil, argErr("score must be between 0 and 10")
	}
	critique, err := stringArg(args, "critique")
	if err != nil {
		return nil, err
	}
	if env.Score != nil {
		env.Score(score, env.sanitize(critique))
	}
	return scoreResult{Recorded: true, Score: score}, nil
}

type delegateResult struct {
	Agent  string `json:"agent"`
	Result string `json:"result"`
}

func delegate(ctx context.Context, args map[string]any, env Env) (any, error) {
	agent, err := stringArg(args, "agent")
	if err != nil {
		return nil, err
	}
	objective, err := stringArg(args, "objective")
	if err != nil {
		return nil, err
	}
	if env.Delegate == nil {
		return nil, argErr("delegation not available in this context")
	}
	res, err := env.Delegate(ctx, agent, objective)
	if err != nil {
		return nil, argErr("delegation failed: " + err.Error())
	}
	if len(res) > MaxBodyBytes {
		res = res[:MaxBodyBytes]
	}
	return delegateResult{Agent: agent, Result: env.sanitize(res)}, nil
}
