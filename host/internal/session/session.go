package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"

	inputsink "github.com/danilostorm/storplay/host/internal/input"
	"github.com/danilostorm/storplay/host/internal/protocol"
)

type Session struct {
	conn  *websocket.Conn
	pc    *webrtc.PeerConnection
	input inputsink.Sink

	writeMu sync.Mutex
	once    sync.Once
}

func New(conn *websocket.Conn, sink inputsink.Sink, stunURL string) (*Session, error) {
	config := webrtc.Configuration{}
	if stunURL != "" {
		config.ICEServers = []webrtc.ICEServer{{URLs: []string{stunURL}}}
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("create peer connection: %w", err)
	}

	s := &Session{conn: conn, pc: pc, input: sink}

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		value := candidate.ToJSON()
		if err := s.send(protocol.SignalMessage{Type: "ice", Candidate: &value}); err != nil {
			log.Printf("session: send ICE candidate: %v", err)
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("session: peer connection state=%s", state.String())

		switch state {
		case webrtc.PeerConnectionStateFailed:
			_ = s.send(protocol.SignalMessage{Type: "error", Message: "WebRTC peer connection failed"})
		case webrtc.PeerConnectionStateClosed:
			_ = s.input.ReleaseAll()
		}
	})

	ordered := false
	maxRetransmits := uint16(0)
	channel, err := pc.CreateDataChannel("input", &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	})
	if err != nil {
		_ = pc.Close()
		return nil, fmt.Errorf("create input data channel: %w", err)
	}

	channel.OnOpen(func() {
		log.Printf("session: input DataChannel open")
	})

	channel.OnClose(func() {
		log.Printf("session: input DataChannel closed")
		_ = s.input.ReleaseAll()
	})

	channel.OnMessage(func(message webrtc.DataChannelMessage) {
		s.handleInput(message.Data)
	})

	return s, nil
}

func (s *Session) Start() error {
	offer, err := s.pc.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("create offer: %w", err)
	}

	if err := s.pc.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("set local description: %w", err)
	}

	local := s.pc.LocalDescription()
	if local == nil {
		return errors.New("local description unavailable after SetLocalDescription")
	}

	if err := s.send(protocol.SignalMessage{Type: "offer", SDP: local}); err != nil {
		return fmt.Errorf("send offer: %w", err)
	}

	for {
		var message protocol.SignalMessage
		if err := s.conn.ReadJSON(&message); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				return fmt.Errorf("signaling read: %w", err)
			}
			return nil
		}

		switch message.Type {
		case "answer":
			if message.SDP == nil {
				_ = s.send(protocol.SignalMessage{Type: "error", Message: "answer is missing SDP"})
				continue
			}
			if err := s.pc.SetRemoteDescription(*message.SDP); err != nil {
				_ = s.send(protocol.SignalMessage{Type: "error", Message: "invalid SDP answer"})
				log.Printf("session: set remote description: %v", err)
			}
		case "ice":
			if message.Candidate == nil {
				_ = s.send(protocol.SignalMessage{Type: "error", Message: "ICE message is missing candidate"})
				continue
			}
			if err := s.pc.AddICECandidate(*message.Candidate); err != nil {
				log.Printf("session: add remote ICE candidate: %v", err)
			}
		default:
			_ = s.send(protocol.SignalMessage{Type: "error", Message: "unknown signaling message"})
		}
	}
}

func (s *Session) Close() {
	s.once.Do(func() {
		_ = s.input.ReleaseAll()
		_ = s.pc.Close()
		_ = s.conn.Close()
	})
}

func (s *Session) send(message protocol.SignalMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteJSON(message)
}

func (s *Session) handleInput(data []byte) {
	if len(data) == 0 || len(data) > 64*1024 {
		return
	}

	var message protocol.InputMessage
	if err := json.Unmarshal(data, &message); err != nil {
		log.Printf("session: invalid input JSON: %v", err)
		return
	}

	if !message.Valid() {
		log.Printf("session: refused unknown input type=%q", message.Type)
		return
	}

	if message.Type == "release_all" {
		if err := s.input.ReleaseAll(); err != nil {
			log.Printf("session: release all input: %v", err)
		}
		return
	}

	if err := s.input.Handle(message); err != nil {
		log.Printf("session: input sink: %v", err)
	}
}
