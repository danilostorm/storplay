package media

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/pion/rtcp"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4"
)

// Relay owns the browser-facing WebRTC media tracks.
//
// The source side intentionally knows nothing about WebRTC. A Sunshine adapter,
// native capture engine, test source, or future cloud source only has to push
// encoded access units/packets into this relay.
type Relay struct {
	videoTrack *webrtc.TrackLocalStaticSample
	audioTrack *webrtc.TrackLocalStaticSample

	videoSender *webrtc.RTPSender
	audioSender *webrtc.RTPSender

	mu     sync.RWMutex
	closed bool

	OnKeyframeRequest func()
}

func New() (*Relay, error) {
	video, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeH264,
			ClockRate: 90000,
			RTCPFeedback: []webrtc.RTCPFeedback{
				{Type: "nack"},
				{Type: "nack", Parameter: "pli"},
				{Type: "ccm", Parameter: "fir"},
			},
		},
		"video",
		"storplay",
	)
	if err != nil {
		return nil, fmt.Errorf("create H264 video track: %w", err)
	}

	audio, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
			SDPFmtpLine: "minptime=10;useinbandfec=1;stereo=1;sprop-stereo=1",
		},
		"audio",
		"storplay",
	)
	if err != nil {
		return nil, fmt.Errorf("create Opus audio track: %w", err)
	}

	return &Relay{
		videoTrack: video,
		audioTrack: audio,
	}, nil
}

// AddToPeerConnection must run before CreateOffer(), so the SDP contains
// send-only video and audio media sections.
func (r *Relay) AddToPeerConnection(pc *webrtc.PeerConnection) error {
	if pc == nil {
		return errors.New("peer connection is nil")
	}

	videoSender, err := pc.AddTrack(r.videoTrack)
	if err != nil {
		return fmt.Errorf("add H264 track: %w", err)
	}
	r.videoSender = videoSender

	audioSender, err := pc.AddTrack(r.audioTrack)
	if err != nil {
		_ = pc.RemoveTrack(videoSender)
		return fmt.Errorf("add Opus track: %w", err)
	}
	r.audioSender = audioSender

	go r.readVideoRTCP(videoSender)
	go drainRTCP("audio", audioSender)

	return nil
}

// PushVideo writes one complete encoded H.264 access unit.
//
// The preferred input format is Annex-B (00 00 00 01 NAL...). Pion's H.264
// payloader packetizes it into RTP without decoding/re-encoding.
//
// duration should normally be 1/fps (for example time.Second/60).
func (r *Relay) PushVideo(accessUnit []byte, duration time.Duration) error {
	r.mu.RLock()
	closed := r.closed
	r.mu.RUnlock()
	if closed {
		return io.ErrClosedPipe
	}
	if len(accessUnit) == 0 {
		return nil
	}
	if duration <= 0 {
		duration = time.Second / 60
	}

	return r.videoTrack.WriteSample(pionmedia.Sample{
		Data:     accessUnit,
		Duration: duration,
	})
}

// PushAudio writes one encoded Opus packet.
//
// Sunshine commonly negotiates 5 ms or 10 ms Opus frames. The source adapter
// should pass the negotiated duration so RTP timestamps advance cleanly.
func (r *Relay) PushAudio(opusPacket []byte, duration time.Duration) error {
	r.mu.RLock()
	closed := r.closed
	r.mu.RUnlock()
	if closed {
		return io.ErrClosedPipe
	}
	if len(opusPacket) == 0 {
		return nil
	}
	if duration <= 0 {
		duration = 5 * time.Millisecond
	}

	return r.audioTrack.WriteSample(pionmedia.Sample{
		Data:     opusPacket,
		Duration: duration,
	})
}

func (r *Relay) Close() {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
}

func (r *Relay) readVideoRTCP(sender *webrtc.RTPSender) {
	for {
		packets, _, err := sender.ReadRTCP()
		if err != nil {
			if !errors.Is(err, io.ErrClosedPipe) {
				log.Printf("media: video RTCP ended: %v", err)
			}
			return
		}

		for _, packet := range packets {
			switch packet.(type) {
			case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
				r.mu.RLock()
				callback := r.OnKeyframeRequest
				closed := r.closed
				r.mu.RUnlock()
				if !closed && callback != nil {
					callback()
				}
			}
		}
	}
}

func drainRTCP(name string, sender *webrtc.RTPSender) {
	for {
		if _, _, err := sender.ReadRTCP(); err != nil {
			if !errors.Is(err, io.ErrClosedPipe) {
				log.Printf("media: %s RTCP ended: %v", name, err)
			}
			return
		}
	}
}
