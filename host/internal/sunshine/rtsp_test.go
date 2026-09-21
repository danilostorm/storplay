package sunshine

import "testing"

func TestParseServerPort(t *testing.T) {
	tests := map[string]int{
		"unicast;server_port=48000-48001;source=127.0.0.1": 48000,
		"server_port=47998":                              47998,
		"":                                               0,
	}
	for input, want := range tests {
		if got := parseServerPort(input); got != want {
			t.Fatalf("parseServerPort(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseRTSPCodecs(t *testing.T) {
	sdp := "m=video 0 RTP/AVP 96\r\na=rtpmap:96 H264/90000\r\na=rtpmap:97 OPUS/48000/2\r\n"
	codecs := parseRTSPCodecs(sdp)
	if len(codecs) != 2 || codecs[0] != "H264" || codecs[1] != "OPUS" {
		t.Fatalf("codecs = %#v", codecs)
	}
}

func TestParseSDPAttribute(t *testing.T) {
	sdp := "v=0\r\na=x-ss-general.featureFlags:123 \r\n"
	if got := parseSDPAttribute(sdp, "x-ss-general.featureFlags"); got != "123" {
		t.Fatalf("feature flags = %q", got)
	}
}
