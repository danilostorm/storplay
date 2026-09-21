package sunshine

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

type MediaProbe struct {
	State            string    `json:"state"`
	PingMode         string    `json:"pingMode,omitempty"`
	AudioPackets     int       `json:"audioPackets"`
	VideoPackets     int       `json:"videoPackets"`
	AudioBytes       int64     `json:"audioBytes"`
	VideoBytes       int64     `json:"videoBytes"`
	FirstAudioPacket int       `json:"firstAudioPacketBytes,omitempty"`
	FirstVideoPacket int       `json:"firstVideoPacketBytes,omitempty"`
	AudioLocalPort   int       `json:"audioLocalPort,omitempty"`
	VideoLocalPort   int       `json:"videoLocalPort,omitempty"`
	StartedAt        time.Time `json:"startedAt,omitempty"`
	FinishedAt       time.Time `json:"finishedAt,omitempty"`
	Error            string    `json:"error,omitempty"`
}

type udpProbeResult struct {
	packets   int
	bytes     int64
	firstSize int
	err       error
}

func (c *Client) ProbeMedia(ctx context.Context) (MediaProbe, error) {
	active := c.ActiveSession()
	if active == nil || active.SessionURL == "" {
		return MediaProbe{}, fmt.Errorf("no active Sunshine GameStream session")
	}

	rtsp := c.RTSPStatus()
	if rtsp == nil || rtsp.State != "setup_complete" || !rtsp.PingPayloadReady {
		probe, probeErr := c.ProbeRTSP(ctx)
		rtsp = &probe
		if probeErr != nil {
			return MediaProbe{}, fmt.Errorf("RTSP setup is not ready: %w", probeErr)
		}
	}

	parsed, err := url.Parse(active.SessionURL)
	if err != nil {
		return MediaProbe{}, fmt.Errorf("parse RTSP session URL: %w", err)
	}
	host := parsed.Hostname()
	if host == "" {
		return MediaProbe{}, fmt.Errorf("RTSP session URL has no host")
	}

	network := "udp4"
	bindIP := net.IPv4zero
	if strings.Contains(host, ":") {
		network = "udp6"
		bindIP = net.IPv6unspecified
	}

	audioConn, err := net.ListenUDP(network, &net.UDPAddr{IP: bindIP, Port: 0})
	if err != nil {
		return MediaProbe{}, fmt.Errorf("bind audio UDP socket: %w", err)
	}
	defer audioConn.Close()

	videoConn, err := net.ListenUDP(network, &net.UDPAddr{IP: bindIP, Port: 0})
	if err != nil {
		return MediaProbe{}, fmt.Errorf("bind video UDP socket: %w", err)
	}
	defer videoConn.Close()

	audioRemote, err := net.ResolveUDPAddr(network, net.JoinHostPort(host, fmt.Sprintf("%d", rtsp.AudioPort)))
	if err != nil {
		return MediaProbe{}, fmt.Errorf("resolve Sunshine audio endpoint: %w", err)
	}
	videoRemote, err := net.ResolveUDPAddr(network, net.JoinHostPort(host, fmt.Sprintf("%d", rtsp.VideoPort)))
	if err != nil {
		return MediaProbe{}, fmt.Errorf("resolve Sunshine video endpoint: %w", err)
	}

	pingMode := "legacy"
	if len(rtsp.audioPingPayload) == 16 && len(rtsp.videoPingPayload) == 16 {
		pingMode = "sunshine-v2"
	}

	result := MediaProbe{
		State:          "starting",
		PingMode:       pingMode,
		AudioLocalPort: audioConn.LocalAddr().(*net.UDPAddr).Port,
		VideoLocalPort: videoConn.LocalAddr().(*net.UDPAddr).Port,
		StartedAt:      time.Now(),
	}
	c.setMediaProbe(result)

	rtspClient, err := newRTSPClient(active.SessionURL)
	if err != nil {
		return c.mediaFailure(result, err)
	}

	sdp := buildDiagnosticSDP(active, rtsp.VideoPort, pingMode == "sunshine-v2")
	announce, err := rtspClient.request(ctx, "ANNOUNCE", "streamid=control/13/0", map[string]string{
		"Session":      rtsp.SessionID,
		"Content-type": "application/sdp",
	}, []byte(sdp))
	if err != nil {
		return c.mediaFailure(result, fmt.Errorf("ANNOUNCE: %w", err))
	}
	if announce.StatusCode != 200 {
		return c.mediaFailure(result, fmt.Errorf("ANNOUNCE: RTSP status %d %s", announce.StatusCode, announce.Status))
	}

	// Sunshine creates the streaming workers during ANNOUNCE. Modern Moonlight
	// identifies the session using the 16-byte X-SS-Ping-Payload returned by
	// SETUP followed by a big-endian sequence number. Use that exact format
	// whenever Sunshine advertised it; keep the legacy 4-byte PING only as a
	// compatibility fallback for older servers.
	pingCtx, cancelPings := context.WithCancel(ctx)
	defer cancelPings()
	go sendGameStreamPings(pingCtx, audioConn, audioRemote, rtsp.audioPingPayload)
	go sendGameStreamPings(pingCtx, videoConn, videoRemote, rtsp.videoPingPayload)

	// Give the ping goroutines a chance to establish the return endpoints before
	// PLAY. Modern Sunshine starts capture from ANNOUNCE, but PLAY is still sent
	// for GameStream compatibility.
	select {
	case <-ctx.Done():
		return c.mediaFailure(result, ctx.Err())
	case <-time.After(120 * time.Millisecond):
	}

	play, err := rtspClient.request(ctx, "PLAY", "/", map[string]string{
		"Session": rtsp.SessionID,
	}, nil)
	if err != nil {
		return c.mediaFailure(result, fmt.Errorf("PLAY: %w", err))
	}
	if play.StatusCode != 200 {
		return c.mediaFailure(result, fmt.Errorf("PLAY: RTSP status %d %s", play.StatusCode, play.Status))
	}

	result.State = "receiving"
	c.setMediaProbe(result)

	const captureWindow = 8 * time.Second
	deadline := time.Now().Add(captureWindow)
	_ = audioConn.SetReadDeadline(deadline)
	_ = videoConn.SetReadDeadline(deadline)

	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan struct {
		kind string
		data udpProbeResult
	}, 2)

	go func() {
		defer wg.Done()
		results <- struct {
			kind string
			data udpProbeResult
		}{"audio", collectUDP(audioConn)}
	}()
	go func() {
		defer wg.Done()
		results <- struct {
			kind string
			data udpProbeResult
		}{"video", collectUDP(videoConn)}
	}()

	wg.Wait()
	close(results)
	cancelPings()

	for item := range results {
		switch item.kind {
		case "audio":
			result.AudioPackets = item.data.packets
			result.AudioBytes = item.data.bytes
			result.FirstAudioPacket = item.data.firstSize
		case "video":
			result.VideoPackets = item.data.packets
			result.VideoBytes = item.data.bytes
			result.FirstVideoPacket = item.data.firstSize
		}
		if item.data.err != nil && !isExpectedUDPTimeout(item.data.err) {
			result.Error = item.kind + ": " + item.data.err.Error()
		}
	}

	result.FinishedAt = time.Now()
	if result.AudioPackets == 0 && result.VideoPackets == 0 {
		return c.mediaFailure(result, fmt.Errorf("Sunshine sent no audio or video UDP packets during the diagnostic window"))
	}
	if result.VideoPackets == 0 {
		return c.mediaFailure(result, fmt.Errorf("audio arrived (%d packets) but no video UDP packets were received", result.AudioPackets))
	}

	result.State = "packets_received"
	result.Error = ""
	c.setMediaProbe(result)
	return result, nil
}

func (c *Client) MediaStatus() *MediaProbe {
	c.mediaMu.Lock()
	defer c.mediaMu.Unlock()
	if c.mediaProbe == nil {
		return nil
	}
	copy := *c.mediaProbe
	return &copy
}

func (c *Client) setMediaProbe(probe MediaProbe) {
	c.mediaMu.Lock()
	copy := probe
	c.mediaProbe = &copy
	c.mediaMu.Unlock()
}

func (c *Client) mediaFailure(probe MediaProbe, err error) (MediaProbe, error) {
	probe.State = "error"
	probe.Error = err.Error()
	probe.FinishedAt = time.Now()
	c.setMediaProbe(probe)
	return probe, err
}

func buildDiagnosticSDP(active *LaunchSession, videoPort int, modernPing bool) string {
	width := active.Width
	if width <= 0 {
		width = 1920
	}
	height := active.Height
	if height <= 0 {
		height = 1080
	}
	fps := active.FPS
	if fps <= 0 {
		fps = 60
	}
	if videoPort <= 0 {
		videoPort = 47998
	}

	mlFeatureFlags := 0
	if modernPing {
		// ML_FF_FEC_STATUS | ML_FF_SESSION_ID_V1
		mlFeatureFlags = 3
	}

	// These are the minimum GameStream attributes Sunshine requires in
	// cmd_announce(), with H.264/stereo/no-media-encryption selected for the
	// first transport diagnostic.
	lines := []string{
		"v=0",
		"o=android 0 14 IN IPv4 127.0.0.1",
		"s=NVIDIA Streaming Client",
		"a=x-nv-audio.surround.numChannels:2 ",
		"a=x-nv-audio.surround.channelMask:3 ",
		"a=x-nv-audio.surround.AudioQuality:0 ",
		"a=x-nv-general.useReliableUdp:13 ",
		"a=x-nv-video[0].packetSize:1024 ",
		fmt.Sprintf("a=x-nv-video[0].clientViewportWd:%d ", width),
		fmt.Sprintf("a=x-nv-video[0].clientViewportHt:%d ", height),
		fmt.Sprintf("a=x-nv-video[0].maxFPS:%d ", fps),
		"a=x-nv-vqos[0].bw.maximumBitrateKbps:20000 ",
		"a=x-nv-video[0].videoEncoderSlicesPerFrame:1 ",
		"a=x-nv-video[0].maxNumReferenceFrames:1 ",
		"a=x-nv-vqos[0].bitStreamFormat:0 ",
		"a=x-nv-vqos[0].qosTrafficType:0 ",
		"a=x-nv-aqos.qosTrafficType:0 ",
		fmt.Sprintf("a=x-ml-general.featureFlags:%d ", mlFeatureFlags),
		"a=x-ml-video.configuredBitrateKbps:20000 ",
		"a=x-ss-general.encryptionEnabled:0 ",
		"t=0 0",
		fmt.Sprintf("m=video %d  ", videoPort),
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

func sendGameStreamPings(ctx context.Context, conn *net.UDPConn, remote *net.UDPAddr, payload string) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var sequence uint32
	send := func() {
		if len(payload) == 16 {
			sequence++
			packet := make([]byte, 20)
			copy(packet[:16], []byte(payload))
			binary.BigEndian.PutUint32(packet[16:], sequence)
			_, _ = conn.WriteToUDP(packet, remote)
			return
		}

		_, _ = conn.WriteToUDP([]byte{'P', 'I', 'N', 'G'}, remote)
	}

	send()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send()
		}
	}
}

func collectUDP(conn *net.UDPConn) udpProbeResult {
	buffer := make([]byte, 64*1024)
	var result udpProbeResult

	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			result.err = err
			return result
		}
		if n <= 0 {
			continue
		}
		result.packets++
		result.bytes += int64(n)
		if result.firstSize == 0 {
			result.firstSize = n
		}
	}
}

func isExpectedUDPTimeout(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
