package media

import (
	"context"
	"time"
)

// Sink is what an encoded media source needs from StorPlay.
//
// Sunshine/GameStream, a native capture engine, and development sources can all
// target this interface. The browser/WebRTC implementation stays isolated.
type Sink interface {
	PushVideo(accessUnit []byte, duration time.Duration) error
	PushAudio(opusPacket []byte, duration time.Duration) error
}

// Source produces already-encoded media. It must not decode/re-encode merely to
// cross this boundary.
type Source interface {
	Start(ctx context.Context, sink Sink) error
	RequestKeyframe()
	Close() error
}
