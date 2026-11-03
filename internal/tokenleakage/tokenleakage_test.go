package tokenleakage

import (
	"encoding/json"
	"testing"
)

func TestDetectAPIKey(t *testing.T) {
	dets := CheckResponse("Here is your key: sk-abc123def456ghi789jkl012mno345pqr678")
	if len(dets) == 0 {
		t.Error("expected API key detection")
	}
	found := false
	for _, d := range dets {
		if d.LeakType == LeakAPIKey {
			found = true
		}
	}
	if !found {
		t.Error("expected api_key leak type")
	}
}

func TestDetectJWT(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	dets := CheckResponse("The token is: " + jwt)
	if len(dets) == 0 {
		t.Error("expected JWT detection")
	}
}

func TestDetectBearerToken(t *testing.T) {
	dets := CheckResponse("Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9abcdefghij")
	if len(dets) == 0 {
		t.Error("expected bearer token detection")
	}
}

func TestDetectAWSKey(t *testing.T) {
	dets := CheckResponse("AWS key: AKIAIOSFODNN7EXAMPLE")
	if len(dets) == 0 {
		t.Error("expected AWS key detection")
	}
	found := false
	for _, d := range dets {
		if d.LeakType == LeakAWSKey {
			found = true
		}
	}
	if !found {
		t.Error("expected aws_key leak type")
	}
}

func TestDetectPassword(t *testing.T) {
	dets := CheckResponse("The password=SuperSecret123!")
	if len(dets) == 0 {
		t.Error("expected password detection")
	}
}

func TestDetectPrivateKey(t *testing.T) {
	dets := CheckResponse("-----BEGIN RSA PRIVATE KEY-----\nMIIEpA...")
	if len(dets) == 0 {
		t.Error("expected private key detection")
	}
	if dets[0].LeakType != LeakPrivateKey {
		t.Errorf("expected private_key, got %s", dets[0].LeakType)
	}
}

func TestDetectGenericSecret(t *testing.T) {
	dets := CheckResponse("secret=abcdefghij1234567890ABCD")
	if len(dets) == 0 {
		t.Error("expected generic secret detection")
	}
}

func TestCleanResponse(t *testing.T) {
	dets := CheckResponse("The weather in New York is sunny today with a high of 75F.")
	if len(dets) != 0 {
		t.Errorf("expected 0 detections for clean response, got %d", len(dets))
	}
}

func TestScoreEmpty(t *testing.T) {
	score := ScoreDetections(nil)
	if score != 0 {
		t.Errorf("expected 0 score for no detections, got %.1f", score)
	}
}

func TestScorePrivateKey(t *testing.T) {
	dets := []Detection{{LeakType: LeakPrivateKey}}
	score := ScoreDetections(dets)
	if score < 30 {
		t.Errorf("expected high score for private key leak, got %.1f", score)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	samples := []Sample{
		{Model: "model-a", Response: "AKIAIOSFODNN7EXAMPLE found"},
		{Model: "model-a", Response: "Clean response here."},
		{Model: "model-b", Response: "Clean output."},
	}
	r := Analyze(samples)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.TotalSamples != 3 {
		t.Errorf("expected 3 total samples, got %d", r.TotalSamples)
	}
}

func TestAnalyzeGrading(t *testing.T) {
	samples := []Sample{
		{Model: "clean", Response: "No secrets in this output."},
	}
	r := Analyze(samples)
	if r.Models[0].Grade != "A+" {
		t.Errorf("expected A+ for clean model, got %s", r.Models[0].Grade)
	}
}

func TestAnalyzeLeakyModel(t *testing.T) {
	samples := []Sample{
		{Model: "leaky", Response: "AKIAIOSFODNN7EXAMPLE"},
		{Model: "leaky", Response: "password=MyS3cretP4ss!"},
	}
	r := Analyze(samples)
	if r.Models[0].LeakyCount != 2 {
		t.Errorf("expected 2 leaky samples, got %d", r.Models[0].LeakyCount)
	}
	if r.Models[0].LeakRate != 100 {
		t.Errorf("expected 100%% leak rate, got %.1f%%", r.Models[0].LeakRate)
	}
}

func TestFormatText(t *testing.T) {
	samples := []Sample{
		{Model: "test", Response: "AKIAIOSFODNN7EXAMPLE"},
	}
	r := Analyze(samples)
	text := FormatText(r)
	if text == "" {
		t.Error("expected non-empty text")
	}
	if !containsStr(text, "Token Leakage") {
		t.Error("expected header in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !containsStr(text, "No samples") {
		t.Error("expected empty message")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	samples := []Sample{
		{Model: "test", Response: "AKIAIOSFODNN7EXAMPLE"},
	}
	r := Analyze(samples)
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var r2 Report
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatal(err)
	}
	if r2.TotalSamples != 1 {
		t.Errorf("expected 1 sample after round-trip, got %d", r2.TotalSamples)
	}
}

func TestDeduplicatesMatches(t *testing.T) {
	response := "AKIAIOSFODNN7EXAMPLE and again AKIAIOSFODNN7EXAMPLE"
	dets := CheckResponse(response)
	count := 0
	for _, d := range dets {
		if d.LeakType == LeakAWSKey {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected deduplicated matches, got %d AWS key detections", count)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
