package media

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"time"

	"github.com/pion/webrtc/v4"
)

// A tiny 320x180 baseline-H.264 IDR access unit generated specifically for the
// StorPlay development diagnostic. Repeating it produces a static blue frame.
//
// This is not a production encoder and never ships user content. Its only job
// is to prove: source -> StorPlay media relay -> RTP -> browser decoder.
const diagnosticH264Base64 = "AAAAAWdCwB7cFBn58BEAAAMAAQAAAwACjxYvgAAAAAFozg/IAAABBgX//1bcRem95tlIt5Ys2CDZI+7veDI2NCAtIGNvcmUgMTY0IHIzMTA4IDMxZTE5ZjkgLSBILjI2NC9NUEVHLTQgQVZDIGNvZGVjIC0gQ29weWxlZnQgMjAwMy0yMDIzIC0gaHR0cDovL3d3dy52aWRlb2xhbi5vcmcveDI2NC5odG1sIC0gb3B0aW9uczogY2FiYWM9MCByZWY9MSBkZWJsb2NrPTA6MDowIGFuYWx5c2U9MDowIG1lPWRpYSBzdWJtZT0wIHBzeT0xIHBzeV9yZD0xLjAwOjAuMDAgbWl4ZWRfcmVmPTAgbWVfcmFuZ2U9MTYgY2hyb21hX21lPTEgdHJlbGxpcz0wIDh4OGRjdD0wIGNxbT0wIGRlYWR6b25lPTIxLDExIGZhc3RfcHNraXA9MSBjaHJvbWFfcXBfb2Zmc2V0PTAgdGhyZWFkcz0zIGxvb2thaGVhZF90aHJlYWRzPTMgc2xpY2VkX3RocmVhZHM9MSBzbGljZXM9MyBucj0wIGRlY2ltYXRlPTEgaW50ZXJsYWNlZD0wIGJsdXJheV9jb21wYXQ9MCBjb25zdHJhaW5lZF9pbnRyYT0wIGJmcmFtZXM9MCB3ZWlnaHRwPTAga2V5aW50PTEga2V5aW50X21pbj0xIHNjZW5lY3V0PTAgaW50cmFfcmVmcmVzaD0wIHJjPWNyZiBtYnRyZWU9MCBjcmY9MjMuMCBxY29tcD0wLjYwIHFwbWluPTAgcXBtYXg9NjkgcXBzdGVwPTQgaXBfcmF0aW89MS40MCBhcT0wAIAAAAFliIQ6EYoAAjFxwABDyjgACAXJycnJycnJycnJycnJycnJycnJ1111111111111111111111111111111111111111111111111111111111114AAAAWUCiIhDoRigACMXHAAEPKOAAIBcnJycnJycnJycnJycnJycnJycnXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXgAAAWUBQiIQ6EYoAAjFxwABDyjgACAXJycnJycnJycnJycnJycnJycnJ1111111111111111111111111111111111111111111111111111111111114A="

type DiagnosticPattern struct {
	frame []byte
}

func NewDiagnosticPattern() (*DiagnosticPattern, error) {
	frame, err := base64.StdEncoding.DecodeString(diagnosticH264Base64)
	if err != nil {
		return nil, err
	}
	return &DiagnosticPattern{frame: frame}, nil
}

func (p *DiagnosticPattern) Start(ctx context.Context, sink Sink) error {
	if sink == nil {
		return errors.New("diagnostic pattern sink is nil")
	}

	// Five keyframes per second is intentionally wasteful but tiny at 320x180.
	// It makes joining/reloading deterministic without requiring a PLI handshake.
	const frameDuration = 200 * time.Millisecond
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	send := func() {
		err := sink.PushVideo(p.frame, frameDuration)
		if err == nil || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, webrtc.ErrNoBind) {
			return
		}
		log.Printf("media: diagnostic frame: %v", err)
	}

	send()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			send()
		}
	}
}

func (p *DiagnosticPattern) RequestKeyframe() {
	// Every diagnostic frame is already an IDR.
}

func (p *DiagnosticPattern) Close() error {
	return nil
}
