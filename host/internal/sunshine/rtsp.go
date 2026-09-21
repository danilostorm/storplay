package sunshine

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RTSPProbe struct {
	State       string    `json:"state"`
	SessionURL  string    `json:"sessionUrl,omitempty"`
	SessionID   string    `json:"sessionId,omitempty"`
	AudioPort   int       `json:"audioPort,omitempty"`
	VideoPort   int       `json:"videoPort,omitempty"`
	ControlPort int       `json:"controlPort,omitempty"`
	Codecs      []string  `json:"codecs,omitempty"`
	FeatureFlags string   `json:"featureFlags,omitempty"`
	ProbedAt    time.Time `json:"probedAt,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type rtspResponse struct {
	StatusCode int
	Status     string
	Headers    map[string]string
	Body       []byte
}

type rtspClient struct {
	sessionURL string
	hostHeader string
	address    string

	mu   sync.Mutex
	cseq int
}

func newRTSPClient(sessionURL string) (*rtspClient, error) {
	u, err := url.Parse(sessionURL)
	if err != nil {
		return nil, fmt.Errorf("parse RTSP session URL: %w", err)
	}
	if u.Scheme != "rtsp" {
		return nil, fmt.Errorf("unsupported RTSP scheme %q (expected rtsp)", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("RTSP URL is missing a host")
	}
	port := u.Port()
	if port == "" {
		port = "554"
	}
	return &rtspClient{
		sessionURL: sessionURL,
		hostHeader: u.Hostname(),
		address:    net.JoinHostPort(u.Hostname(), port),
		cseq:       1,
	}, nil
}

func (c *Client) ProbeRTSP(ctx context.Context) (RTSPProbe, error) {
	active := c.ActiveSession()
	if active == nil || active.SessionURL == "" {
		return RTSPProbe{}, fmt.Errorf("no active Sunshine GameStream session")
	}

	probe := RTSPProbe{
		State:      "probing",
		SessionURL: active.SessionURL,
	}

	rtsp, err := newRTSPClient(active.SessionURL)
	if err != nil {
		probe.State = "error"
		probe.Error = err.Error()
		return probe, err
	}

	options, err := rtsp.request(ctx, "OPTIONS", active.SessionURL, nil, nil)
	if err != nil {
		return probeFailure(probe, "OPTIONS", err)
	}
	if options.StatusCode != 200 {
		return probeFailure(probe, "OPTIONS", fmt.Errorf("RTSP status %d %s", options.StatusCode, options.Status))
	}

	describe, err := rtsp.request(ctx, "DESCRIBE", active.SessionURL, map[string]string{
		"Accept":            "application/sdp",
		"If-Modified-Since": "Thu, 01 Jan 1970 00:00:00 GMT",
	}, nil)
	if err != nil {
		return probeFailure(probe, "DESCRIBE", err)
	}
	if describe.StatusCode != 200 {
		return probeFailure(probe, "DESCRIBE", fmt.Errorf("RTSP status %d %s", describe.StatusCode, describe.Status))
	}

	sdp := string(describe.Body)
	probe.Codecs = parseRTSPCodecs(sdp)
	probe.FeatureFlags = parseSDPAttribute(sdp, "x-ss-general.featureFlags")

	audio, err := rtsp.request(ctx, "SETUP", "streamid=audio/0/0", map[string]string{
		"Transport":         "unicast;X-GS-ClientPort=50000-50001",
		"If-Modified-Since": "Thu, 01 Jan 1970 00:00:00 GMT",
	}, nil)
	if err != nil {
		return probeFailure(probe, "SETUP audio", err)
	}
	if audio.StatusCode != 200 {
		return probeFailure(probe, "SETUP audio", fmt.Errorf("RTSP status %d %s", audio.StatusCode, audio.Status))
	}

	sessionID := strings.TrimSpace(strings.Split(audio.Headers["session"], ";")[0])
	if sessionID == "" {
		return probeFailure(probe, "SETUP audio", fmt.Errorf("Sunshine response did not include Session header"))
	}
	probe.SessionID = sessionID
	probe.AudioPort = parseServerPort(audio.Headers["transport"])

	sessionHeaders := map[string]string{
		"Session":           sessionID,
		"Transport":         "unicast;X-GS-ClientPort=50000-50001",
		"If-Modified-Since": "Thu, 01 Jan 1970 00:00:00 GMT",
	}

	video, err := rtsp.request(ctx, "SETUP", "streamid=video/0/0", sessionHeaders, nil)
	if err != nil {
		return probeFailure(probe, "SETUP video", err)
	}
	if video.StatusCode != 200 {
		return probeFailure(probe, "SETUP video", fmt.Errorf("RTSP status %d %s", video.StatusCode, video.Status))
	}
	probe.VideoPort = parseServerPort(video.Headers["transport"])

	control, err := rtsp.request(ctx, "SETUP", "streamid=control/13/0", sessionHeaders, nil)
	if err != nil {
		return probeFailure(probe, "SETUP control", err)
	}
	if control.StatusCode != 200 {
		return probeFailure(probe, "SETUP control", fmt.Errorf("RTSP status %d %s", control.StatusCode, control.Status))
	}
	probe.ControlPort = parseServerPort(control.Headers["transport"])

	// Sunshine normally advertises well-known defaults if a Transport response
	// omits server_port. Preserve those as diagnostic fallbacks.
	if probe.AudioPort == 0 {
		probe.AudioPort = 48000
	}
	if probe.VideoPort == 0 {
		probe.VideoPort = 47998
	}
	if probe.ControlPort == 0 {
		probe.ControlPort = 47999
	}

	probe.State = "setup_complete"
	probe.ProbedAt = time.Now()

	c.rtspMu.Lock()
	c.rtspProbe = &probe
	c.rtspMu.Unlock()

	return probe, nil
}

func (c *Client) RTSPStatus() *RTSPProbe {
	c.rtspMu.Lock()
	defer c.rtspMu.Unlock()
	if c.rtspProbe == nil {
		return nil
	}
	copy := *c.rtspProbe
	copy.Codecs = append([]string(nil), c.rtspProbe.Codecs...)
	return &copy
}

func (c *rtspClient) request(ctx context.Context, method, target string, headers map[string]string, body []byte) (rtspResponse, error) {
	c.mu.Lock()
	cseq := c.cseq
	c.cseq++
	c.mu.Unlock()

	dialer := &net.Dialer{Timeout: 4 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", c.address)
	if err != nil {
		return rtspResponse{}, fmt.Errorf("connect %s: %w", c.address, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(6 * time.Second))

	var request strings.Builder
	fmt.Fprintf(&request, "%s %s RTSP/1.0\r\n", method, target)
	fmt.Fprintf(&request, "CSeq: %d\r\n", cseq)
	fmt.Fprintf(&request, "X-GS-ClientVersion: 14\r\n")
	fmt.Fprintf(&request, "Host: %s\r\n", c.hostHeader)
	for key, value := range headers {
		fmt.Fprintf(&request, "%s: %s\r\n", key, value)
	}
	if len(body) > 0 {
		fmt.Fprintf(&request, "Content-Length: %d\r\n", len(body))
	}
	request.WriteString("\r\n")
	if len(body) > 0 {
		request.Write(body)
	}

	if _, err := io.WriteString(conn, request.String()); err != nil {
		return rtspResponse{}, fmt.Errorf("send %s: %w", method, err)
	}

	reader := bufio.NewReaderSize(conn, 64*1024)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return rtspResponse{}, fmt.Errorf("read %s status: %w", method, err)
	}
	statusLine = strings.TrimSpace(statusLine)
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return rtspResponse{}, fmt.Errorf("malformed RTSP status line %q", statusLine)
	}
	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return rtspResponse{}, fmt.Errorf("malformed RTSP status code %q", parts[1])
	}
	status := ""
	if len(parts) == 3 {
		status = parts[2]
	}

	response := rtspResponse{
		StatusCode: code,
		Status:     status,
		Headers:    map[string]string{},
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return rtspResponse{}, fmt.Errorf("read %s headers: %w", method, err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		response.Headers[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}

	if rawLength := response.Headers["content-length"]; rawLength != "" {
		length, err := strconv.Atoi(rawLength)
		if err != nil || length < 0 || length > 2*1024*1024 {
			return rtspResponse{}, fmt.Errorf("invalid RTSP Content-Length %q", rawLength)
		}
		if length > 0 {
			response.Body = make([]byte, length)
			if _, err := io.ReadFull(reader, response.Body); err != nil {
				return rtspResponse{}, fmt.Errorf("read %s body: %w", method, err)
			}
		}
	} else {
		// Current Sunshine DESCRIBE responses have Content-Length. Keep this
		// bounded fallback for compatibility with simpler RTSP responses.
		response.Body, _ = io.ReadAll(io.LimitReader(reader, 2*1024*1024))
	}

	return response, nil
}

func parseServerPort(transport string) int {
	const marker = "server_port="
	index := strings.Index(strings.ToLower(transport), marker)
	if index < 0 {
		return 0
	}
	value := transport[index+len(marker):]
	if end := strings.IndexAny(value, "-;"); end >= 0 {
		value = value[:end]
	}
	port, _ := strconv.Atoi(strings.TrimSpace(value))
	if port < 1 || port > 65535 {
		return 0
	}
	return port
}

func parseRTSPCodecs(sdp string) []string {
	upper := strings.ToUpper(sdp)
	codecs := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, codec := range []string{"H264", "H265", "HEVC", "AV1", "OPUS"} {
		if strings.Contains(upper, codec) && !seen[codec] {
			seen[codec] = true
			codecs = append(codecs, codec)
		}
	}
	return codecs
}

func parseSDPAttribute(sdp, name string) string {
	prefix := "a=" + name + ":"
	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func probeFailure(probe RTSPProbe, stage string, err error) (RTSPProbe, error) {
	wrapped := fmt.Errorf("%s: %w", stage, err)
	probe.State = "error"
	probe.Error = wrapped.Error()
	probe.ProbedAt = time.Now()
	return probe, wrapped
}
