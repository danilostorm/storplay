package sunshine

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PairingStatus struct {
	State string `json:"state"`
	PIN   string `json:"pin,omitempty"`
	Error string `json:"error,omitempty"`
}

type pairXML struct {
	XMLName           xml.Name `xml:"root"`
	StatusCode        int      `xml:"status_code,attr"`
	Paired            int      `xml:"paired"`
	PlainCert         string   `xml:"plaincert"`
	ChallengeResponse string   `xml:"challengeresponse"`
	PairingSecret     string   `xml:"pairingsecret"`
}

type App struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	HDRSupported bool   `json:"hdrSupported"`
}

type appListXML struct {
	XMLName xml.Name `xml:"root"`
	Apps    []struct {
		ID             int    `xml:"ID"`
		Title          string `xml:"AppTitle"`
		IsHDRSupported int    `xml:"IsHdrSupported"`
	} `xml:"App"`
	StatusCode int `xml:"status_code,attr"`
}

func (c *Client) StartPairing() (PairingStatus, error) {
	if c.Identity == nil {
		return PairingStatus{}, fmt.Errorf("Sunshine client identity is not configured")
	}

	c.pairMu.Lock()
	if c.pairState.State == "waiting_for_pin" || c.pairState.State == "pairing" {
		status := c.pairState
		c.pairMu.Unlock()
		return status, nil
	}
	pin, err := randomPIN()
	if err != nil {
		c.pairMu.Unlock()
		return PairingStatus{}, err
	}
	c.pairState = PairingStatus{State: "waiting_for_pin", PIN: pin}
	c.pairMu.Unlock()

	go c.runPairing(pin)
	return PairingStatus{State: "waiting_for_pin", PIN: pin}, nil
}

func (c *Client) PairingStatus() PairingStatus {
	c.pairMu.Lock()
	defer c.pairMu.Unlock()
	return c.pairState
}

func (c *Client) setPairState(state PairingStatus) {
	c.pairMu.Lock()
	c.pairState = state
	c.pairMu.Unlock()
}

func (c *Client) runPairing(pin string) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	info, err := c.Probe(ctx)
	if err != nil {
		c.setPairState(PairingStatus{State: "error", Error: err.Error()})
		return
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		c.setPairState(PairingStatus{State: "error", Error: "generate pairing salt: " + err.Error()})
		return
	}

	stage1Args := url.Values{}
	stage1Args.Set("devicename", "StorPlay")
	stage1Args.Set("updateState", "1")
	stage1Args.Set("phrase", "getservercert")
	stage1Args.Set("salt", hex.EncodeToString(salt))
	stage1Args.Set("clientcert", hex.EncodeToString(c.Identity.CertPEM))

	stage1, err := c.pairRequest(ctx, false, info.HTTPSPort, stage1Args, 65*time.Second)
	if err != nil {
		c.setPairState(PairingStatus{State: "error", Error: "pairing stage 1: " + err.Error()})
		return
	}
	if stage1.Paired != 1 {
		c.setPairState(PairingStatus{State: "error", Error: "Sunshine did not accept pairing stage 1"})
		return
	}

	serverCertPEM, err := hex.DecodeString(stage1.PlainCert)
	if err != nil || len(serverCertPEM) == 0 {
		c.setPairState(PairingStatus{State: "error", Error: "Sunshine returned an invalid server certificate"})
		return
	}
	serverCert, err := parsePEMCertificate(serverCertPEM)
	if err != nil {
		c.setPairState(PairingStatus{State: "error", Error: err.Error()})
		return
	}

	c.setPairState(PairingStatus{State: "pairing", PIN: pin})

	hashLen, hashFn := pairingHash(info.AppVersion)
	aesKey := hashFn(append(append([]byte{}, salt...), []byte(pin)...))[:16]

	randomChallenge := make([]byte, 16)
	if _, err := rand.Read(randomChallenge); err != nil {
		c.setPairState(PairingStatus{State: "error", Error: "generate client challenge: " + err.Error()})
		return
	}
	encryptedChallenge, err := aesECBEncrypt(randomChallenge, aesKey)
	if err != nil {
		c.setPairState(PairingStatus{State: "error", Error: err.Error()})
		return
	}

	stage2Args := url.Values{}
	stage2Args.Set("devicename", "StorPlay")
	stage2Args.Set("updateState", "1")
	stage2Args.Set("clientchallenge", hex.EncodeToString(encryptedChallenge))

	stage2, err := c.pairRequest(ctx, false, info.HTTPSPort, stage2Args, 6*time.Second)
	if err != nil {
		c.setPairState(PairingStatus{State: "error", Error: "pairing stage 2: " + err.Error()})
		return
	}
	if stage2.Paired != 1 {
		c.setPairState(PairingStatus{State: "error", Error: "PIN was not accepted by Sunshine"})
		return
	}

	challengeCipher, err := hex.DecodeString(stage2.ChallengeResponse)
	if err != nil {
		c.setPairState(PairingStatus{State: "error", Error: "invalid Sunshine challenge response"})
		return
	}
	challengeData, err := aesECBDecrypt(challengeCipher, aesKey)
	if err != nil || len(challengeData) < hashLen+16 {
		c.setPairState(PairingStatus{State: "error", Error: "invalid decrypted Sunshine challenge"})
		return
	}
	serverResponse := append([]byte{}, challengeData[:hashLen]...)

	clientSecret := make([]byte, 16)
	if _, err := rand.Read(clientSecret); err != nil {
		c.setPairState(PairingStatus{State: "error", Error: "generate client secret: " + err.Error()})
		return
	}

	challengeResponse := make([]byte, 0, 16+len(c.Identity.Leaf.Signature)+16)
	challengeResponse = append(challengeResponse, challengeData[hashLen:hashLen+16]...)
	challengeResponse = append(challengeResponse, c.Identity.Leaf.Signature...)
	challengeResponse = append(challengeResponse, clientSecret...)
	digest := hashFn(challengeResponse)
	padded := make([]byte, 32)
	copy(padded, digest)
	encryptedResponse, err := aesECBEncrypt(padded, aesKey)
	if err != nil {
		c.setPairState(PairingStatus{State: "error", Error: err.Error()})
		return
	}

	stage3Args := url.Values{}
	stage3Args.Set("devicename", "StorPlay")
	stage3Args.Set("updateState", "1")
	stage3Args.Set("serverchallengeresp", hex.EncodeToString(encryptedResponse))

	stage3, err := c.pairRequest(ctx, false, info.HTTPSPort, stage3Args, 6*time.Second)
	if err != nil || stage3.Paired != 1 {
		if err != nil {
			c.setPairState(PairingStatus{State: "error", Error: "pairing stage 3: " + err.Error()})
		} else {
			c.setPairState(PairingStatus{State: "error", Error: "Sunshine rejected pairing stage 3"})
		}
		return
	}

	pairingSecret, err := hex.DecodeString(stage3.PairingSecret)
	if err != nil || len(pairingSecret) <= 16 {
		c.setPairState(PairingStatus{State: "error", Error: "Sunshine returned an invalid pairing secret"})
		return
	}
	serverSecret := pairingSecret[:16]
	serverSignature := pairingSecret[16:]

	serverPub, ok := serverCert.PublicKey.(*rsa.PublicKey)
	if !ok {
		c.setPairState(PairingStatus{State: "error", Error: "Sunshine server certificate is not RSA"})
		return
	}
	serverSecretHash := sha256.Sum256(serverSecret)
	if err := rsa.VerifyPKCS1v15(serverPub, crypto.SHA256, serverSecretHash[:], serverSignature); err != nil {
		c.setPairState(PairingStatus{State: "error", Error: "Sunshine server signature verification failed"})
		return
	}

	expectedData := make([]byte, 0, 16+len(serverCert.Signature)+16)
	expectedData = append(expectedData, randomChallenge...)
	expectedData = append(expectedData, serverCert.Signature...)
	expectedData = append(expectedData, serverSecret...)
	if !bytes.Equal(hashFn(expectedData), serverResponse) {
		c.setPairState(PairingStatus{State: "error", Error: "PIN verification failed"})
		return
	}

	clientSecretHash := sha256.Sum256(clientSecret)
	clientSignature, err := rsa.SignPKCS1v15(rand.Reader, c.Identity.Private, crypto.SHA256, clientSecretHash[:])
	if err != nil {
		c.setPairState(PairingStatus{State: "error", Error: "sign client pairing secret: " + err.Error()})
		return
	}
	clientPairingSecret := append(append([]byte{}, clientSecret...), clientSignature...)

	stage4Args := url.Values{}
	stage4Args.Set("devicename", "StorPlay")
	stage4Args.Set("updateState", "1")
	stage4Args.Set("clientpairingsecret", hex.EncodeToString(clientPairingSecret))
	stage4, err := c.pairRequest(ctx, false, info.HTTPSPort, stage4Args, 6*time.Second)
	if err != nil || stage4.Paired != 1 {
		if err != nil {
			c.setPairState(PairingStatus{State: "error", Error: "pairing stage 4: " + err.Error()})
		} else {
			c.setPairState(PairingStatus{State: "error", Error: "Sunshine rejected client certificate"})
		}
		return
	}

	stage5Args := url.Values{}
	stage5Args.Set("devicename", "StorPlay")
	stage5Args.Set("updateState", "1")
	stage5Args.Set("phrase", "pairchallenge")
	stage5, err := c.pairRequest(ctx, true, info.HTTPSPort, stage5Args, 6*time.Second)
	if err != nil || stage5.Paired != 1 {
		if err != nil {
			c.setPairState(PairingStatus{State: "error", Error: "pairing certificate check: " + err.Error()})
		} else {
			c.setPairState(PairingStatus{State: "error", Error: "Sunshine did not authorize StorPlay certificate"})
		}
		return
	}

	c.setPairState(PairingStatus{State: "paired"})
}

func (c *Client) pairRequest(ctx context.Context, https bool, httpsPort int, values url.Values, timeout time.Duration) (pairXML, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return pairXML{}, err
	}
	if https {
		u.Scheme = "https"
		u.Host = netJoinHostPort(u.Hostname(), httpsPort)
	}
	u.Path = "/pair"
	q := u.Query()
	q.Set("uniqueid", c.UniqueID)
	q.Set("uuid", newRequestID())
	for key, vals := range values {
		for _, val := range vals {
			q.Add(key, val)
		}
	}
	u.RawQuery = q.Encode()

	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return pairXML{}, err
	}
	req.Header.Set("User-Agent", "StorPlay/0.1")
	req.Header.Set("Connection", "close")

	client := &http.Client{Timeout: timeout}
	if https {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Sunshine uses a self-signed server certificate.
			Certificates:       []tls.Certificate{c.Identity.TLS},
			MinVersion:         tls.VersionTLS12,
		}}
	}

	resp, err := client.Do(req)
	if err != nil {
		return pairXML{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return pairXML{}, err
	}

	var parsed pairXML
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return pairXML{}, fmt.Errorf("parse Sunshine pairing XML: %w", err)
	}
	if parsed.StatusCode != 200 {
		return pairXML{}, fmt.Errorf("Sunshine pairing status_code=%d HTTP=%d", parsed.StatusCode, resp.StatusCode)
	}
	return parsed, nil
}

func (c *Client) AppList(ctx context.Context) ([]App, error) {
	if c.Identity == nil {
		return nil, fmt.Errorf("Sunshine client identity is not configured")
	}
	info, err := c.Probe(ctx)
	if err != nil {
		return nil, err
	}

	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	base.Scheme = "https"
	base.Host = netJoinHostPort(base.Hostname(), info.HTTPSPort)
	base.Path = "/applist"
	q := base.Query()
	q.Set("uniqueid", c.UniqueID)
	q.Set("uuid", newRequestID())
	base.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "StorPlay/0.1")
	req.Header.Set("Connection", "close")

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			Certificates:       []tls.Certificate{c.Identity.TLS},
			MinVersion:         tls.VersionTLS12,
		}},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Sunshine applist request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var parsed appListXML
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse Sunshine app list: %w", err)
	}
	if parsed.StatusCode != 200 {
		return nil, fmt.Errorf("Sunshine applist status_code=%d HTTP=%d", parsed.StatusCode, resp.StatusCode)
	}

	// A successful authenticated HTTPS /applist request proves Sunshine accepted
	// StorPlay's persisted client certificate. This is a stronger signal than
	// the legacy PairStatus field returned by plaintext /serverinfo.
	c.setPairState(PairingStatus{State: "paired"})

	apps := make([]App, 0, len(parsed.Apps))
	for _, item := range parsed.Apps {
		if item.ID == 0 || strings.TrimSpace(item.Title) == "" {
			continue
		}
		apps = append(apps, App{ID: item.ID, Title: item.Title, HDRSupported: item.IsHDRSupported == 1})
	}
	return apps, nil
}

func randomPIN() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", fmt.Errorf("generate pairing PIN: %w", err)
	}
	return fmt.Sprintf("%04d", n.Int64()), nil
}

func pairingHash(appVersion string) (int, func([]byte) []byte) {
	major := 0
	if first := strings.Split(strings.TrimSpace(appVersion), "."); len(first) > 0 {
		major, _ = strconv.Atoi(first[0])
	}
	if major >= 7 {
		return 32, func(data []byte) []byte {
			sum := sha256.Sum256(data)
			return sum[:]
		}
	}
	return 20, func(data []byte) []byte {
		sum := sha1.Sum(data)
		return sum[:]
	}
}

func aesECBEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(plaintext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("AES plaintext length %d is not block-aligned", len(plaintext))
	}
	out := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i += block.BlockSize() {
		block.Encrypt(out[i:i+block.BlockSize()], plaintext[i:i+block.BlockSize()])
	}
	return out, nil
}

func aesECBDecrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("AES ciphertext length %d is not block-aligned", len(ciphertext))
	}
	out := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += block.BlockSize() {
		block.Decrypt(out[i:i+block.BlockSize()], ciphertext[i:i+block.BlockSize()])
	}
	return out, nil
}

func parsePEMCertificate(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("decode Sunshine server certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse Sunshine server certificate: %w", err)
	}
	return cert, nil
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return NewUniqueID()
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func netJoinHostPort(host string, port int) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + strconv.Itoa(port)
}
