package sunshine

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type LaunchConfig struct {
	AppID          int  `json:"appId"`
	Width          int  `json:"width"`
	Height         int  `json:"height"`
	FPS            int  `json:"fps"`
	BitrateKbps    int  `json:"bitrateKbps"`
	HDR            bool `json:"hdr"`
	PlayHostAudio  bool `json:"playHostAudio"`
}

type LaunchSession struct {
	AppID       int       `json:"appId"`
	SessionURL  string    `json:"sessionUrl"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	FPS         int       `json:"fps"`
	HDR         bool      `json:"hdr"`
	StartedAt   time.Time `json:"startedAt"`

	// Host-only protocol material. Never serialize these to the browser.
	RIKey   []byte `json:"-"`
	RIKeyID uint32 `json:"-"`
}

type launchXML struct {
	XMLName      xml.Name `xml:"root"`
	StatusCode   int      `xml:"status_code,attr"`
	StatusMessage string   `xml:"status_message,attr"`
	GameSession  int      `xml:"gamesession"`
	SessionURL   string   `xml:"sessionUrl0"`
	Resume       int      `xml:"resume"`
}

func (c *Client) Launch(ctx context.Context, cfg LaunchConfig) (LaunchSession, error) {
	if c.Identity == nil {
		return LaunchSession{}, fmt.Errorf("Sunshine client identity is not configured")
	}
	if cfg.AppID <= 0 {
		return LaunchSession{}, fmt.Errorf("invalid Sunshine app ID %d", cfg.AppID)
	}
	if cfg.Width <= 0 {
		cfg.Width = 1920
	}
	if cfg.Height <= 0 {
		cfg.Height = 1080
	}
	if cfg.FPS <= 0 {
		cfg.FPS = 60
	}
	if cfg.BitrateKbps <= 0 {
		cfg.BitrateKbps = 20000
	}

	info, err := c.Probe(ctx)
	if err != nil {
		return LaunchSession{}, err
	}
	if info.PairStatus != 1 {
		return LaunchSession{}, fmt.Errorf("StorPlay is not paired with Sunshine")
	}

	rikey := make([]byte, 16)
	if _, err := rand.Read(rikey); err != nil {
		return LaunchSession{}, fmt.Errorf("generate GameStream key: %w", err)
	}
	var idBytes [4]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return LaunchSession{}, fmt.Errorf("generate GameStream key ID: %w", err)
	}
	rikeyID := binary.BigEndian.Uint32(idBytes[:])

	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return LaunchSession{}, err
	}
	base.Scheme = "https"
	base.Host = netJoinHostPort(base.Hostname(), info.HTTPSPort)
	base.Path = "/launch"

	q := base.Query()
	q.Set("appid", strconv.Itoa(cfg.AppID))
	q.Set("uniqueid", c.UniqueID)
	q.Set("uuid", newRequestID())
	q.Set("mode", fmt.Sprintf("%dx%dx%d", cfg.Width, cfg.Height, cfg.FPS))
	q.Set("rikey", hex.EncodeToString(rikey))
	q.Set("rikeyid", strconv.FormatUint(uint64(rikeyID), 10))
	if cfg.PlayHostAudio {
		q.Set("localAudioPlayMode", "1")
	} else {
		q.Set("localAudioPlayMode", "0")
	}
	q.Set("sops", "1")
	q.Set("surroundAudioInfo", "196610")
	q.Set("gcmap", "0")
	if cfg.HDR {
		q.Set("hdrMode", "1")
	} else {
		q.Set("hdrMode", "0")
	}

	// StorPlay and Sunshine currently run on the same PC for this MVP. Keeping
	// RTSP control plaintext on localhost makes the first native Go adapter much
	// simpler while WebRTC still encrypts the browser-facing hop. Encrypted
	// GameStream RTSP is a later compatibility milestone.
	q.Set("corever", "0")
	base.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return LaunchSession{}, fmt.Errorf("build Sunshine launch request: %w", err)
	}
	req.Header.Set("User-Agent", "StorPlay/0.1")
	req.Header.Set("Connection", "close")

	client := c.mTLSClient(15 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return LaunchSession{}, fmt.Errorf("Sunshine launch request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return LaunchSession{}, fmt.Errorf("read Sunshine launch response: %w", err)
	}

	var parsed launchXML
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return LaunchSession{}, fmt.Errorf("parse Sunshine launch XML: %w", err)
	}
	if parsed.StatusCode != 200 {
		if parsed.StatusMessage != "" {
			return LaunchSession{}, fmt.Errorf("Sunshine launch rejected (%d): %s", parsed.StatusCode, parsed.StatusMessage)
		}
		return LaunchSession{}, fmt.Errorf("Sunshine launch rejected with status_code=%d", parsed.StatusCode)
	}
	if parsed.SessionURL == "" {
		return LaunchSession{}, fmt.Errorf("Sunshine launch response did not include sessionUrl0")
	}

	session := LaunchSession{
		AppID:      cfg.AppID,
		SessionURL: parsed.SessionURL,
		Width:      cfg.Width,
		Height:     cfg.Height,
		FPS:        cfg.FPS,
		HDR:        cfg.HDR,
		StartedAt:  time.Now(),
		RIKey:      rikey,
		RIKeyID:    rikeyID,
	}

	c.sessionMu.Lock()
	c.activeSession = &session
	c.sessionMu.Unlock()

	return session, nil
}

func (c *Client) ActiveSession() *LaunchSession {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if c.activeSession == nil {
		return nil
	}
	copy := *c.activeSession
	copy.RIKey = append([]byte(nil), c.activeSession.RIKey...)
	return &copy
}

func (c *Client) Cancel(ctx context.Context) error {
	if c.Identity == nil {
		return fmt.Errorf("Sunshine client identity is not configured")
	}
	info, err := c.Probe(ctx)
	if err != nil {
		return err
	}

	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return err
	}
	base.Scheme = "https"
	base.Host = netJoinHostPort(base.Hostname(), info.HTTPSPort)
	base.Path = "/cancel"
	q := base.Query()
	q.Set("uniqueid", c.UniqueID)
	q.Set("uuid", newRequestID())
	base.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return fmt.Errorf("build Sunshine cancel request: %w", err)
	}
	req.Header.Set("User-Agent", "StorPlay/0.1")
	req.Header.Set("Connection", "close")

	resp, err := c.mTLSClient(5 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("Sunshine cancel request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read Sunshine cancel response: %w", err)
	}

	var parsed launchXML
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("parse Sunshine cancel XML: %w", err)
	}
	if parsed.StatusCode != 200 {
		if parsed.StatusMessage != "" {
			return fmt.Errorf("Sunshine cancel rejected (%d): %s", parsed.StatusCode, parsed.StatusMessage)
		}
		return fmt.Errorf("Sunshine cancel rejected with status_code=%d", parsed.StatusCode)
	}

	c.sessionMu.Lock()
	c.activeSession = nil
	c.sessionMu.Unlock()
	return nil
}

func (c *Client) mTLSClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Sunshine uses a self-signed certificate.
			Certificates:       []tls.Certificate{c.Identity.TLS},
			MinVersion:         tls.VersionTLS12,
		}},
	}
}
