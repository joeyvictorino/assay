package validate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// ---- security-headers -----------------------------------------------------

type securityHeaders struct{}

func (securityHeaders) Class() model.Class { return model.ClassSecurityHeaders }

// requiredHeaders are the response headers whose absence is the finding.
var requiredHeaders = []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options"}

func (securityHeaders) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	target := targetURL(env, f, nil)
	var evs []model.Evidence
	var sets []string
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.Exchange(ctx, "GET", target, nil, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		var missing []string
		for _, h := range requiredHeaders {
			if r.Headers[h] == "" {
				missing = append(missing, h)
			}
		}
		sort.Strings(missing)
		sets = append(sets, strings.Join(missing, ","))
		if len(missing) > 0 {
			evs = append(evs, evidence("header-missing", r, strings.Join(missing, ",")))
		}
	}
	switch {
	case len(evs) == MinObservations && sets[0] == sets[1]:
		return result(model.StateValidated, "missing: "+sets[0], evs...), nil
	case len(evs) == 0:
		return result(model.StateRefuted, "all required security headers present"), nil
	}
	return result(model.StateTheorized, "inconsistent headers across observations", evs...), nil
}

// ---- verbose-error --------------------------------------------------------

type verboseError struct{}

func (verboseError) Class() model.Class { return model.ClassVerboseError }

var verboseIndicators = []string{
	"goroutine ", "panic:", "runtime/debug", "Traceback (most recent call last)", "Exception", "exception",
	"at org.", "at com.", "stack trace", "Stack trace", "Stack Trace", ".go:", ".java:", ".py\", line", "Fatal error", "on line ",
	"ORA-", "SQLSTATE", "syntax error", "ODBC", "mysql_", "pg_query",
}

func (verboseError) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	target := targetURL(env, f, nil)
	var evs []model.Evidence
	calm := 0
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.Exchange(ctx, methodOf(f), target, nil, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		body := string(r.Body)
		hit := ""
		for _, ind := range verboseIndicators {
			if strings.Contains(body, ind) {
				hit = ind
				break
			}
		}
		if r.Status >= 500 && hit != "" {
			evs = append(evs, evidence("http-exchange", r, strings.TrimSpace(hit)))
		} else if hit == "" {
			calm++ // an error page without engine or stack details is not verbose
		}
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, "5xx with stack or engine details", evs...), nil
	case calm == MinObservations:
		return result(model.StateRefuted, "no error details observed"), nil
	}
	return result(model.StateTheorized, "inconsistent error behavior", evs...), nil
}

// ---- cors-misconfig -------------------------------------------------------

type corsMisconfig struct{}

func (corsMisconfig) Class() model.Class { return model.ClassCORS }

func (corsMisconfig) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	target := targetURL(env, f, nil)
	origin := "https://" + newMarker() + ".invalid"
	var evs []model.Evidence
	absent := 0
	var why string
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.Exchange(ctx, "GET", target, map[string]string{"Origin": origin}, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		acao := strings.TrimSpace(r.Headers["Access-Control-Allow-Origin"])
		acac := strings.EqualFold(strings.TrimSpace(r.Headers["Access-Control-Allow-Credentials"]), "true")
		switch {
		case acao == "*" && acac:
			why = "wildcard origin with credentials"
		case acao == origin:
			why = "arbitrary origin reflected"
			if acac {
				why += " with credentials"
			}
		case acao == "null":
			why = "null origin allowed"
		case acao == "":
			absent++
			continue
		default:
			continue
		}
		evs = append(evs, evidence("http-exchange", r, origin))
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, why, evs...), nil
	case absent == MinObservations:
		return result(model.StateRefuted, "no Access-Control-Allow-Origin header"), nil
	case len(evs) == 0:
		return result(model.StateRefuted, "origin not reflected and no wildcard-with-credentials"), nil
	}
	return result(model.StateTheorized, "inconsistent CORS behavior", evs...), nil
}

// ---- info-disclosure / sensitive-file (marker presence) -------------------

type markerChecker struct{ class model.Class }

func (m markerChecker) Class() model.Class { return m.class }

func (m markerChecker) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	marker := env.Marker
	if gt := groundTruthFor(env, f); gt != nil && gt.Marker != "" {
		marker = gt.Marker
	}
	if marker == "" {
		return result(model.StateTheorized, "no marker configured for "+string(m.class)), nil
	}
	target := targetURL(env, f, nil)
	var evs []model.Evidence
	missing := 0
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.Exchange(ctx, methodOf(f), target, nil, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		if r.Status == 200 && strings.Contains(string(r.Body), marker) {
			evs = append(evs, evidence("marker-reflected", r, marker))
		} else {
			missing++
		}
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, "planted marker returned without credentials", evs...), nil
	case missing == MinObservations:
		return result(model.StateRefuted, "marker not present"), nil
	}
	return result(model.StateTheorized, "marker intermittently present", evs...), nil
}

// ---- auth-missing ---------------------------------------------------------

type authMissing struct{}

func (authMissing) Class() model.Class { return model.ClassAuthMissing }

func (authMissing) Check(ctx context.Context, f model.Finding, env Env) (model.ValidationResult, error) {
	gt := groundTruthFor(env, f)
	if gt == nil || !gt.AuthRequired {
		return result(model.StateTheorized, "ground truth does not mark this path as requiring authentication"), nil
	}
	target := targetURL(env, f, nil)
	var evs []model.Evidence
	denied := 0
	for i := 0; i < MinObservations; i++ {
		r, err := env.HTTP.Exchange(ctx, methodOf(f), target, nil, nil)
		if err != nil {
			return model.ValidationResult{}, err
		}
		switch {
		case r.Status == 200 && len(r.Body) > 0:
			evs = append(evs, evidence("auth-bypass", r, ""))
		case r.Status == 401 || r.Status == 403:
			denied++
		}
	}
	switch {
	case len(evs) == MinObservations:
		return result(model.StateValidated, fmt.Sprintf("200 without credentials on %s (ground truth %s)", f.Location.PathTemplate, gt.ID), evs...), nil
	case denied == MinObservations:
		return result(model.StateRefuted, "unauthenticated request denied"), nil
	}
	return result(model.StateTheorized, "inconsistent responses", evs...), nil
}
