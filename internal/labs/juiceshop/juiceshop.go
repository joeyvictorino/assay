// Package juiceshop adapts OWASP Juice Shop to the labs.Lab interface.
// Accounts come from labs/juice-shop/accounts.yaml (well-known demo users);
// labs/juice-shop/reference_findings.json is a PARTIAL hand-made reference
// list, never treated as exhaustive ground truth.
package juiceshop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"gopkg.in/yaml.v3"
)

// Name is the lab identifier.
const Name = "juice-shop"

// Lab is the Juice Shop adapter.
type Lab struct {
	baseURL      string
	accountsPath string
	refPath      string
	http         *httpx.Client

	once     sync.Once
	accounts []labs.Account
	refs     []labs.GroundTruthEntry
}

// New returns an adapter for a Juice Shop at baseURL.
func New(baseURL, accountsPath, refPath string, client *httpx.Client) *Lab {
	return &Lab{baseURL: strings.TrimRight(baseURL, "/"), accountsPath: accountsPath, refPath: refPath, http: client}
}

func (l *Lab) Name() string    { return Name }
func (l *Lab) BaseURL() string { return l.baseURL }

// Health checks /rest/admin/application-version.
func (l *Lab) Health(ctx context.Context) error {
	status, _, body, err := l.http.Do(ctx, "GET", l.baseURL+"/rest/admin/application-version", nil, nil)
	if err != nil {
		return err
	}
	if status != 200 || !strings.Contains(string(body), "version") {
		return fmt.Errorf("juice-shop: unhealthy: status %d", status)
	}
	return nil
}

// Setup is a no-op; Juice Shop seeds itself on first start.
func (l *Lab) Setup(context.Context) error { return nil }

type accountsFile struct {
	Accounts []labs.Account `yaml:"accounts"`
}

func (l *Lab) load() {
	l.once.Do(func() {
		if data, err := os.ReadFile(l.accountsPath); err == nil {
			var f accountsFile
			if yaml.Unmarshal(data, &f) == nil {
				l.accounts = f.Accounts
			}
		}
		if entries, _, err := labs.LoadGroundTruth(l.refPath); err == nil {
			l.refs = entries
		}
	})
}

// Accounts returns the committed demo accounts.
func (l *Lab) Accounts() []labs.Account {
	l.load()
	return append([]labs.Account(nil), l.accounts...)
}

// GroundTruth returns the partial reference list; never exhaustive.
func (l *Lab) GroundTruth() ([]labs.GroundTruthEntry, bool) {
	l.load()
	if len(l.refs) == 0 {
		return nil, false
	}
	return append([]labs.GroundTruthEntry(nil), l.refs...), false
}

// Login posts to /rest/user/login and returns Authorization and Cookie
// headers plus the numeric user id.
func (l *Lab) Login(ctx context.Context, acct labs.Account) (map[string]string, string, error) {
	body, _ := json.Marshal(map[string]string{"email": acct.Username, "password": acct.Password})
	status, _, resp, err := l.http.Do(ctx, "POST", l.baseURL+"/rest/user/login", map[string]string{"Content-Type": "application/json"}, body)
	if err != nil {
		return nil, "", err
	}
	if status == 401 || status == 403 {
		return nil, "", fmt.Errorf("juice-shop: login %s: status %d: %w", acct.Username, status, labs.ErrLoginRejected)
	}
	if status != 200 {
		return nil, "", fmt.Errorf("juice-shop: login %s: status %d", acct.Username, status)
	}
	var out struct {
		Authentication struct {
			Token string `json:"token"`
			Bid   int    `json:"bid"`
			Umail string `json:"umail"`
		} `json:"authentication"`
	}
	if err := json.Unmarshal(resp, &out); err != nil || out.Authentication.Token == "" {
		return nil, "", fmt.Errorf("juice-shop: login %s: bad response", acct.Username)
	}
	tok := out.Authentication.Token
	// The user id lives in the JWT payload (data.id); bid is the basket id.
	userID := jwtUserID(tok)
	if userID == "" {
		userID = strconv.Itoa(out.Authentication.Bid)
	}
	return map[string]string{
		"Authorization": "Bearer " + tok,
		"Cookie":        "token=" + tok,
	}, userID, nil
}

// jwtUserID reads data.id from an unverified JWT payload. The token is
// only used to address the lab's own API; nothing is trusted from it.
func jwtUserID(tok string) string {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return ""
	}
	var claims struct {
		Data struct {
			ID json.Number `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	return claims.Data.ID.String()
}

var _ labs.Lab = (*Lab)(nil)
var _ labs.Authenticator = (*Lab)(nil)
