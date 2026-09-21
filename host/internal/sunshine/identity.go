package sunshine

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type Identity struct {
	UniqueID string
	CertPEM  []byte
	KeyPEM   []byte
	TLS      tls.Certificate
	Leaf     *x509.Certificate
	Private  *rsa.PrivateKey
}

type persistedIdentity struct {
	UniqueID string `json:"uniqueId"`
	CertPEM  string `json:"certPem"`
	KeyPEM   string `json:"keyPem"`
}

func LoadOrCreateIdentity() (*Identity, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config directory: %w", err)
	}
	dir := filepath.Join(base, "StorPlay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create StorPlay config directory: %w", err)
	}
	path := filepath.Join(dir, "sunshine-identity.json")

	if body, err := os.ReadFile(path); err == nil {
		var stored persistedIdentity
		if err := json.Unmarshal(body, &stored); err == nil {
			id, err := parseIdentity(stored.UniqueID, []byte(stored.CertPEM), []byte(stored.KeyPEM))
			if err == nil {
				return id, nil
			}
		}
	}

	id, err := generateIdentity()
	if err != nil {
		return nil, err
	}
	body, err := json.MarshalIndent(persistedIdentity{
		UniqueID: id.UniqueID,
		CertPEM:  string(id.CertPEM),
		KeyPEM:   string(id.KeyPEM),
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode StorPlay identity: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return nil, fmt.Errorf("persist StorPlay identity: %w", err)
	}
	return id, nil
}

func generateIdentity() (*Identity, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA identity key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "StorPlay",
			Organization: []string{"StorPlay"},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(20, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create StorPlay client certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return parseIdentity(NewUniqueID(), certPEM, keyPEM)
}

func parseIdentity(uniqueID string, certPEM, keyPEM []byte) (*Identity, error) {
	if uniqueID == "" {
		return nil, fmt.Errorf("identity unique ID is empty")
	}

	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("load StorPlay TLS identity: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return nil, fmt.Errorf("StorPlay TLS identity has no certificate")
	}

	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse StorPlay certificate: %w", err)
	}

	private, ok := pair.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("StorPlay private key is not RSA")
	}

	pair.Leaf = leaf
	return &Identity{
		UniqueID: uniqueID,
		CertPEM:  certPEM,
		KeyPEM:   keyPEM,
		TLS:      pair,
		Leaf:     leaf,
		Private:  private,
	}, nil
}
