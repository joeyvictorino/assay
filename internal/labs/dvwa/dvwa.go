// Package dvwa adapts Damn Vulnerable Web Application to the labs.Lab
// interface. Setup creates the database via /setup.php, logs in as the
// documented admin account and pins the security level cookie to "low".
// labs/dvwa/reference_findings.json is a PARTIAL hand-made reference list.
package dvwa

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
)

// Name is the lab identifier.
const Name = "dvwa"

var tokenRe = regexp.MustCompile(`name=['"]user_token['"]\s+value=['"]([0-9a-f]+)['"]|value=['"]([0-9a-f]+)['"]\s+name=['"]user_token['"]`)
var sessRe = regexp.MustCompile(`PHPSESSID=([^;,\s]+)`)

// Well-known DVWA seeded users (dvwa/includes/DBMS/MySQL.php). Lab use only.
var accounts = []labs.Account{
	{Role: "admin", Username: "admin", Password: "password"},
	{Role: "user", Username: "gordonb", Password: "abc123"},
	{Role: "user", Username: "1337", Password: "charley"},
	{Role: "user", Username: "pablo", Password: "letmein"},
	{Role: "user", Username: "smithy", Password: "password"},
}

// Lab is the DVWA adapter.
type Lab struct {
	baseURL string
	refPath string
	http    *httpx.Client

	mu     sync.Mutex
	cookie string // "PHPSESSID=...; security=low" after Setup

	once sync.Once
	refs []labs.GroundTruthEntry
}

// New returns an adapter for a DVWA at baseURL.
func New(baseURL, refPath string, client *httpx.Client) *Lab {
	return &Lab{baseURL: strings.TrimRight(baseURL, "/"), refPath: refPath, http: client}
}

func (l *Lab) Name() string    { return Name }
func (l *Lab) BaseURL() string { return l.baseURL }

// Health checks /login.php.
func (l *Lab) Health(ctx context.Context) error {
	status, _, body, err := l.http.Do(ctx, "GET", l.baseURL+"/login.php", nil, nil)
	if err != nil {
		return err
	}
	if status != 200 || !strings.Contains(strings.ToLower(string(body)), "login") {
		return fmt.Errorf("dvwa: unhealthy: status %d", status)
	}
	return nil
}

// Setup creates/resets the database, logs in as admin and sets security=low.
func (l *Lab) Setup(ctx context.Context) error {
	// 1. GET /setup.php for the CSRF token and a session.
	status, hdrs, body, err := l.http.Do(ctx, "GET", l.baseURL+"/setup.php", nil, nil)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("dvwa: setup.php: status %d", status)
	}
	sess := sessionFrom(hdrs)
	token := tokenFrom(body)
	if token == "" {
		return fmt.Errorf("dvwa: setup.php: no user_token in page")
	}
	// 2. POST create_db.
	form := url.Values{"create_db": {"Create / Reset Database"}, "user_token": {token}}
	status, _, _, err = l.http.DoNoFollow(ctx, "POST", l.baseURL+"/setup.php",
		map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Cookie": cookieHeader(sess, "")}, []byte(form.Encode()))
	if err != nil {
		return err
	}
	if status != 302 && status != 200 {
		return fmt.Errorf("dvwa: create_db: status %d", status)
	}
	// 3. Log in as admin and pin the security level.
	hdrsOut, _, err := l.Login(ctx, accounts[0])
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.cookie = hdrsOut["Cookie"]
	l.mu.Unlock()
	return nil
}

// Login performs the form login and returns a Cookie header carrying the
// authenticated PHPSESSID and security=low. DVWA has no numeric user id in
// the session; the username is returned as the id.
func (l *Lab) Login(ctx context.Context, acct labs.Account) (map[string]string, string, error) {
	status, hdrs, body, err := l.http.Do(ctx, "GET", l.baseURL+"/login.php", nil, nil)
	if err != nil {
		return nil, "", err
	}
	if status != 200 {
		return nil, "", fmt.Errorf("dvwa: login.php: status %d", status)
	}
	sess := sessionFrom(hdrs)
	token := tokenFrom(body)
	if token == "" || sess == "" {
		return nil, "", fmt.Errorf("dvwa: login.php: missing token or session")
	}
	form := url.Values{"username": {acct.Username}, "password": {acct.Password}, "Login": {"Login"}, "user_token": {token}}
	status, hdrs, _, err = l.http.DoNoFollow(ctx, "POST", l.baseURL+"/login.php",
		map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Cookie": cookieHeader(sess, "low")}, []byte(form.Encode()))
	if err != nil {
		return nil, "", err
	}
	loc := hdrs["Location"]
	if status != 302 || strings.Contains(loc, "login.php") {
		return nil, "", fmt.Errorf("dvwa: login %s failed (status %d, location %q)", acct.Username, status, loc)
	}
	if s := sessionFrom(hdrs); s != "" {
		sess = s
	}
	return map[string]string{"Cookie": cookieHeader(sess, "low")}, acct.Username, nil
}

// Cookie returns the session cookie established by Setup ("" before Setup).
func (l *Lab) Cookie() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cookie
}

// Accounts returns the seeded DVWA users; admin/password is the default.
func (l *Lab) Accounts() []labs.Account {
	return append([]labs.Account(nil), accounts...)
}

// GroundTruth returns the partial reference list; never exhaustive.
func (l *Lab) GroundTruth() ([]labs.GroundTruthEntry, bool) {
	l.once.Do(func() {
		if entries, _, err := labs.LoadGroundTruth(l.refPath); err == nil {
			l.refs = entries
		}
	})
	if len(l.refs) == 0 {
		return nil, false
	}
	return append([]labs.GroundTruthEntry(nil), l.refs...), false
}

func tokenFrom(body []byte) string {
	m := tokenRe.FindSubmatch(body)
	if m == nil {
		return ""
	}
	if len(m[1]) > 0 {
		return string(m[1])
	}
	return string(m[2])
}

func sessionFrom(hdrs map[string]string) string {
	m := sessRe.FindStringSubmatch(hdrs["Set-Cookie"])
	if m == nil {
		return ""
	}
	return m[1]
}

func cookieHeader(sess, security string) string {
	parts := []string{}
	if sess != "" {
		parts = append(parts, "PHPSESSID="+sess)
	}
	if security != "" {
		parts = append(parts, "security="+security)
	}
	return strings.Join(parts, "; ")
}

var _ labs.Lab = (*Lab)(nil)
var _ labs.Authenticator = (*Lab)(nil)
