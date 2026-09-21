package media

import (
	"testing"

	"github.com/pion/webrtc/v4"
)

func TestRelayAddsVideoAndAudioTracks(t *testing.T) {
	relay, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("NewPeerConnection() error = %v", err)
	}
	defer pc.Close()

	if err := relay.AddToPeerConnection(pc); err != nil {
		t.Fatalf("AddToPeerConnection() error = %v", err)
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer() error = %v", err)
	}

	if offer.SDP == "" {
		t.Fatal("expected non-empty SDP offer")
	}

	// Two outbound media senders should exist: video + audio.
	if got := len(pc.GetSenders()); got != 2 {
		t.Fatalf("senders = %d, want 2", got)
	}
}
