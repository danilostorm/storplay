package input

import (
	"log"

	"github.com/danilostorm/storplay/host/internal/protocol"
)

// Sink is the boundary between WebRTC input and operating-system input
// injection. Keeping this interface small lets us add Windows SendInput,
// ViGEm/Virtual Gamepad, and Linux uinput backends independently.
type Sink interface {
	Handle(protocol.InputMessage) error
	ReleaseAll() error
}

type LoggingSink struct{}

func (LoggingSink) Handle(message protocol.InputMessage) error {
	switch message.Type {
	case "mouse_move":
		// Mouse move events are intentionally not logged individually: pointer
		// lock can generate hundreds per second.
		return nil
	case "gamepad":
		return nil
	default:
		log.Printf("input: type=%s code=%q key=%q button=%d down=%v", message.Type, message.Code, message.Key, message.Button, message.Down)
		return nil
	}
}

func (LoggingSink) ReleaseAll() error {
	log.Printf("input: release_all")
	return nil
}
