package validate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/model"
)

func sortStrings(s []string) { sort.Strings(s) }

// ---- idor -----------------------------------------------------------------

type idor struct{}

func (idor) Class() model.Class { return model.ClassIDOR }

// Check logs in as two lab accounts and has each read the other's record.
// Reads only; both directions must succeed.
func (idor) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	auth, ok := env.Lab.(labs.Authenticator)
	if !ok {
		return result(model.StateTheorized, "lab cannot issue sessions"), nil
	}
	a, b, ok := pickTwo(env.Lab.Accounts())
	if !ok {
		return result(model.StateTheorized, "lab lists fewer than two accounts"), nil
	}
	hdrA, idA, err := auth.Login(ctx, a)
	if err != nil {
		return model.ValidationResult{}, err
	}
	hdrB, idB, err := auth.Login(ctx, b)
	if err != nil {
		return model.ValidationResult{}, err
	}
	if idA == idB {
		return result(model.StateTheorized, "accounts resolve to the same id"), nil
	}
	var evs []model.Evidence
	denied := 0
	for _, step := range []struct {
		hdr    map[string]string
		victim labs.Account
		id     string
	}{{hdrA, b, idB}, {hdrB, a, idA}} {
		target := targetURL(env, f, map[string]string{"id": step.id})
		r, err := env.HTTP.Exchange(ctx, methodOf(f), target, step.hdr, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		switch {
		case r.Status == 200 && mentionsAccount(r.Body, step.victim, step.id):
			evs = append(evs, evidence("auth-bypass", r, "cross-account read of id "+step.id))
		case r.Status == 401 || r.Status == 403 || r.Status == 404:
			denied++
		}
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, "each account read the other's record", evs...), nil
	case denied == MinObservations:
		return result(model.StateRefuted, "cross-account reads denied"), nil
	}
	return result(model.StateTheorized, "cross-account read inconclusive", evs...), nil
}

// pickTwo prefers two non-admin accounts, then any two distinct ones.
func pickTwo(accts []labs.Account) (labs.Account, labs.Account, bool) {
	var users, all []labs.Account
	seen := map[string]bool{}
	for _, a := range accts {
		if a.Username == "" || seen[a.Username] {
			continue
		}
		seen[a.Username] = true
		all = append(all, a)
		if !strings.EqualFold(a.Role, "admin") {
			users = append(users, a)
		}
	}
	if len(users) >= 2 {
		return users[0], users[1], true
	}
	if len(all) >= 2 {
		return all[0], all[1], true
	}
	return labs.Account{}, labs.Account{}, false
}

// mentionsAccount reports whether a record body belongs to the victim: it
// names the username/email or carries the id as a JSON field.
func mentionsAccount(body []byte, victim labs.Account, id string) bool {
	s := string(body)
	if strings.Contains(s, victim.Username) {
		return true
	}
	var obj map[string]any
	if json.Unmarshal(body, &obj) == nil {
		if v, ok := obj["id"]; ok && fmt.Sprint(v) == id {
			return true
		}
	}
	return false
}

// ---- default-creds --------------------------------------------------------

type defaultCreds struct{}

func (defaultCreds) Class() model.Class { return model.ClassDefaultCreds }

// Check logs in with the lab's documented default (role "admin") twice.
// The password never appears in evidence or notes.
func (defaultCreds) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	auth, ok := env.Lab.(labs.Authenticator)
	if !ok {
		return result(model.StateTheorized, "lab cannot issue sessions"), nil
	}
	var acct *labs.Account
	for _, a := range env.Lab.Accounts() {
		if strings.EqualFold(a.Role, "admin") {
			a := a
			acct = &a
			break
		}
	}
	if acct == nil {
		return result(model.StateTheorized, "lab lists no default (admin) credential"), nil
	}
	var evs []model.Evidence
	failed := 0
	for i := 0; i < MinObservations; i++ {
		hdrs, id, err := auth.Login(ctx, *acct)
		if errors.Is(err, labs.ErrLoginRejected) {
			failed++
			continue
		}
		if err != nil {
			return model.ValidationResult{}, err // gate denial or transport: inconclusive
		}
		if len(hdrs) == 0 {
			failed++
			continue
		}
		evs = append(evs, model.Evidence{Kind: "auth-bypass", Marker: fmt.Sprintf("login:%s:id=%s", acct.Username, id)})
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, "documented default credential accepted for "+acct.Username, evs...), nil
	case failed == MinObservations:
		return result(model.StateRefuted, "default credential rejected"), nil
	}
	return result(model.StateTheorized, "login intermittently accepted", evs...), nil
}
