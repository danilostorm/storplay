package sunshine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ServerInfo struct {
	StatusCode        int    `json:"statusCode"`
	Hostname          string `json:"hostname"`
	AppVersion        string `json:"appVersion"`
	GfeVersion        string `json:"gfeVersion"`
	UniqueID          string `json:"uniqueId"`
	HTTPSPort         int    `json:"httpsPort"`
	ExternalPort      int    `json:"externalPort"`
	PairStatus        int    `json:"pairStatus"`
	CurrentGame       int    `json:"currentGame"`
	State             string `json:"state"`
	MaxLumaPixelsHEVC string `json:"maxLumaPixelsHEVC"`
}

type serverInfoXML struct {
	XMLName           xml.Name `xml:"root"`
	StatusCode        int      `xml:"status_code,attr"`
	Hostname          string   `xml:"hostname"`
	AppVersion        string   `xml:"appversion"`
	GfeVersion        string   `xml:"GfeVersion"`
	UniqueID          string   `xml:"uniqueid"`
	HTTPSPort         int      `xml:"HttpsPort"`
	ExternalPort      int      `xml:"ExternalPort"`
	PairStatus        int      `xml:"PairStatus"`
	CurrentGame       int      `xml:"currentgame"`
	State             string   `xml:"state"`
	MaxLumaPixelsHEVC string   `xml:"MaxLumaPixelsHEVC"`
}

type Client struct {
	BaseURL  string
	UniqueID string
	Identity *Identity
	HTTP     *http.Client

	pairMu    sync.Mutex
	pairState PairingStatus

	sessionMu     sync.Mutex
	activeSession *LaunchSession
}

func New(baseURL, uniqueID string) *Client {
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		UniqueID: uniqueID,
		HTTP: &http.Client{
			Timeout: 1200 * time.Millisecond,
		},
		pairState: PairingStatus{State: "unpaired"},
	}
}

func NewWithIdentity(baseURL string, identity *Identity) *Client {
	client := New(baseURL, identity.UniqueID)
	client.Identity = identity
	return client
}

func NewUniqueID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func (c *Client) Probe(ctx context.Context) (ServerInfo, error) {
	if c.BaseURL == "" {
		return ServerInfo{}, fmt.Errorf("Sunshine base URL is empty")
	}

	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return ServerInfo{}, fmt.Errorf("parse Sunshine URL: %w", err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return ServerInfo{}, fmt.Errorf("unsupported Sunshine URL scheme %q", base.Scheme)
	}
	if base.Host == "" {
		return ServerInfo{}, fmt.Errorf("Sunshine URL is missing a host")
	}

	base.Path = strings.TrimRight(base.Path, "/") + "/serverinfo"
	query := base.Query()
	query.Set("uniqueid", c.UniqueID)
	query.Set("uuid", NewUniqueID())
	base.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return ServerInfo{}, fmt.Errorf("build Sunshine serverinfo request: %w", err)
	}
	req.Header.Set("User-Agent", "StorPlay/0.1")
	req.Header.Set("Connection", "close")

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 1200 * time.Millisecond}
	}

	resp, err := client.Do(req)
	if err != nil {
		return ServerInfo{}, fmt.Errorf("Sunshine serverinfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return ServerInfo{}, fmt.Errorf("Sunshine serverinfo HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ServerInfo{}, fmt.Errorf("read Sunshine serverinfo: %w", err)
	}

	var parsed serverInfoXML
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return ServerInfo{}, fmt.Errorf("parse Sunshine serverinfo XML: %w", err)
	}

	if parsed.StatusCode != 200 {
		return ServerInfo{}, fmt.Errorf("Sunshine serverinfo status_code=%d", parsed.StatusCode)
	}

	c.pairMu.Lock()
	if parsed.PairStatus == 1 && c.pairState.State != "waiting_for_pin" && c.pairState.State != "pairing" {
		c.pairState = PairingStatus{State: "paired"}
	} else if parsed.PairStatus == 0 && c.pairState.State == "paired" {
		c.pairState = PairingStatus{State: "unpaired"}
	}
	c.pairMu.Unlock()

	return ServerInfo{
		StatusCode:        parsed.StatusCode,
		Hostname:          parsed.Hostname,
		AppVersion:        parsed.AppVersion,
		GfeVersion:        parsed.GfeVersion,
		UniqueID:          parsed.UniqueID,
		HTTPSPort:         parsed.HTTPSPort,
		ExternalPort:      parsed.ExternalPort,
		PairStatus:        parsed.PairStatus,
		CurrentGame:       parsed.CurrentGame,
		State:             parsed.State,
		MaxLumaPixelsHEVC: parsed.MaxLumaPixelsHEVC,
	}, nil
}
