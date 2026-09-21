package sunshine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeParsesServerInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/serverinfo" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("uniqueid") != "storplay-test" {
			t.Fatalf("missing uniqueid query")
		}
		if r.URL.Query().Get("uuid") == "" {
			t.Fatalf("missing uuid query")
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<root status_code="200">
			<hostname>Gaming-PC</hostname>
			<appversion>2026.920.1</appversion>
			<GfeVersion>3.26.0.0</GfeVersion>
			<uniqueid>sunshine-id</uniqueid>
			<HttpsPort>47984</HttpsPort>
			<ExternalPort>47989</ExternalPort>
			<PairStatus>0</PairStatus>
			<currentgame>0</currentgame>
			<state>SUNSHINE_SERVER_FREE</state>
			<MaxLumaPixelsHEVC>1869449984</MaxLumaPixelsHEVC>
		</root>`))
	}))
	defer server.Close()

	client := New(server.URL, "storplay-test")
	info, err := client.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}

	if info.Hostname != "Gaming-PC" {
		t.Fatalf("Hostname = %q", info.Hostname)
	}
	if info.HTTPSPort != 47984 {
		t.Fatalf("HTTPSPort = %d", info.HTTPSPort)
	}
	if info.State != "SUNSHINE_SERVER_FREE" {
		t.Fatalf("State = %q", info.State)
	}
}

func TestProbeRejectsBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<root status_code="401"><hostname>nope</hostname></root>`))
	}))
	defer server.Close()

	_, err := New(server.URL, "id").Probe(context.Background())
	if err == nil || !strings.Contains(err.Error(), "status_code=401") {
		t.Fatalf("expected status_code error, got %v", err)
	}
}
