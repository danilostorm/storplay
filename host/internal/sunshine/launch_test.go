package sunshine

import (
	"encoding/xml"
	"testing"
)

func TestParseLaunchResponse(t *testing.T) {
	const body = `<root status_code="200"><sessionUrl0>rtsp://127.0.0.1:48010</sessionUrl0><gamesession>1</gamesession></root>`
	var parsed launchXML
	if err := xml.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("xml.Unmarshal() error = %v", err)
	}
	if parsed.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", parsed.StatusCode)
	}
	if parsed.SessionURL != "rtsp://127.0.0.1:48010" {
		t.Fatalf("SessionURL = %q", parsed.SessionURL)
	}
	if parsed.GameSession != 1 {
		t.Fatalf("GameSession = %d, want 1", parsed.GameSession)
	}
}
