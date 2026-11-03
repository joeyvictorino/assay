package policy

import (
	"testing"
)

func TestSearchByKeyword(t *testing.T) {
	yaml, _ := GenerateTemplate("strict")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	results := Search(eng.Policy(), SearchQuery{Keyword: "recon"})
	if len(results) == 0 {
		t.Fatal("expected results for keyword 'recon'")
	}
	for _, r := range results {
		found := false
		if r.MatchField == "agent" || r.MatchField == "id" || r.MatchField == "rationale" {
			found = true
		}
		if !found {
			t.Errorf("unexpected match field %q for keyword 'recon'", r.MatchField)
		}
	}
}

func TestSearchByEffect(t *testing.T) {
	yaml, _ := GenerateTemplate("strict")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	results := Search(eng.Policy(), SearchQuery{Effect: "deny"})
	for _, r := range results {
		if string(r.Rule.Effect) != "deny" {
			t.Errorf("expected deny effect, got %s", r.Rule.Effect)
		}
	}
	if len(results) != 3 {
		t.Errorf("strict template has 3 deny rules, got %d", len(results))
	}
}

func TestSearchByAgent(t *testing.T) {
	yaml, _ := GenerateTemplate("standard")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	results := Search(eng.Policy(), SearchQuery{Agent: "probe"})
	if len(results) == 0 {
		t.Fatal("expected results for agent 'probe'")
	}
	for _, r := range results {
		found := false
		for _, a := range r.Rule.Match.Agents {
			if a == "probe" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rule %s should mention agent 'probe'", r.Rule.ID)
		}
	}
}

func TestSearchByCapability(t *testing.T) {
	yaml, _ := GenerateTemplate("strict")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	results := Search(eng.Policy(), SearchQuery{Capability: "auth.login"})
	if len(results) == 0 {
		t.Fatal("expected results for capability 'auth.login'")
	}
	for _, r := range results {
		found := false
		for _, c := range r.Rule.Match.Capabilities {
			if c == "auth.login" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rule %s should match capability 'auth.login'", r.Rule.ID)
		}
	}
}

func TestSearchCombinedFilters(t *testing.T) {
	yaml, _ := GenerateTemplate("strict")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	results := Search(eng.Policy(), SearchQuery{
		Effect: "deny",
		Agent:  "recon",
	})
	if len(results) != 1 {
		t.Errorf("expected 1 deny rule for recon, got %d", len(results))
	}
	if len(results) > 0 && results[0].Rule.ID != "deny-recon-write" {
		t.Errorf("expected deny-recon-write, got %s", results[0].Rule.ID)
	}
}

func TestSearchNoMatches(t *testing.T) {
	yaml, _ := GenerateTemplate("minimal")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	results := Search(eng.Policy(), SearchQuery{Keyword: "nonexistent-xyz"})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchAllRules(t *testing.T) {
	yaml, _ := GenerateTemplate("standard")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	results := Search(eng.Policy(), SearchQuery{})
	if len(results) != len(eng.Policy().Rules) {
		t.Errorf("empty query should return all %d rules, got %d", len(eng.Policy().Rules), len(results))
	}
}

func TestFormatSearchText(t *testing.T) {
	yaml, _ := GenerateTemplate("strict")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	results := Search(eng.Policy(), SearchQuery{Effect: "allow"})
	text := FormatSearchText(results)
	if text == "" {
		t.Error("text should not be empty")
	}
	if len(results) > 0 {
		if !containsSubstr(text, "rule(s) matched") {
			t.Error("text should contain match count")
		}
	}
}

func TestFormatSearchTextEmpty(t *testing.T) {
	text := FormatSearchText(nil)
	if !containsSubstr(text, "No matching rules") {
		t.Error("empty results should say no matching rules")
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
