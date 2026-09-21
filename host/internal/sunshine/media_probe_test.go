package sunshine

import (
	"strings"
	"testing"
)

func TestBuildDiagnosticSDPModernPing(t *testing.T) {
	active := &LaunchSession{Width: 1920, Height: 1080, FPS: 60}
	sdp := buildDiagnosticSDP(active, 47998, true)
	required := []string{
		"x-nv-audio.surround.numChannels:2",
		"x-nv-video[0].clientViewportWd:1920",
		"x-nv-video[0].clientViewportHt:1080",
		"x-nv-video[0].maxFPS:60",
		"x-nv-vqos[0].bw.maximumBitrateKbps:20000",
		"x-nv-general.useReliableUdp:13",
		"x-ml-general.featureFlags:3",
		"m=video 47998",
	}
	for _, want := range required {
		if !strings.Contains(sdp, want) {
			t.Fatalf("SDP missing %q\n%s", want, sdp)
		}
	}
}

func TestBuildDiagnosticSDPLegacyPing(t *testing.T) {
	active := &LaunchSession{Width: 1280, Height: 720, FPS: 60}
	sdp := buildDiagnosticSDP(active, 47998, false)
	if !strings.Contains(sdp, "x-ml-general.featureFlags:0") {
		t.Fatalf("legacy SDP should disable session-ID ping feature\n%s", sdp)
	}
}
