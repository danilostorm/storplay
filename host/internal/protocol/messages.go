package protocol

import "github.com/pion/webrtc/v4"

// SignalMessage is the small JSON protocol exchanged over the signaling
// WebSocket. SDP and ICE remain standard WebRTC structures.
type SignalMessage struct {
	Type      string                    `json:"type"`
	SDP       *webrtc.SessionDescription `json:"sdp,omitempty"`
	Candidate *webrtc.ICECandidateInit   `json:"candidate,omitempty"`
	Message   string                    `json:"message,omitempty"`
	Reason    string                    `json:"reason,omitempty"`
}

// InputMessage is deliberately transport-neutral. The browser currently sends
// JSON while the protocol is young; high-frequency input can move to a compact
// binary representation without changing the WebRTC session architecture.
type InputMessage struct {
	Type      string    `json:"type"`
	Code      string    `json:"code,omitempty"`
	Key       string    `json:"key,omitempty"`
	Down      bool      `json:"down,omitempty"`
	Repeat    bool      `json:"repeat,omitempty"`
	DX        float64   `json:"dx,omitempty"`
	DY        float64   `json:"dy,omitempty"`
	Button    int       `json:"button,omitempty"`
	Index     int       `json:"index,omitempty"`
	Buttons   []float64 `json:"buttons,omitempty"`
	Axes      []float64 `json:"axes,omitempty"`
	Timestamp float64   `json:"timestamp,omitempty"`
}

func (m InputMessage) Valid() bool {
	switch m.Type {
	case "key", "mouse_move", "mouse_button", "mouse_wheel", "gamepad", "release_all":
		return true
	default:
		return false
	}
}
