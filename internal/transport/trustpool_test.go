package transport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// generate a self-signed ECDSA cert and return PEM bytes
func genSelfSignedPEM(t *testing.T) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "inline-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	b := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if len(b) == 0 {
		t.Fatalf("pem encode failed")
	}
	return b
}

func TestTrustedPeerCertsMergeIntoPool(t *testing.T) {
	sec := t.TempDir()
	// Build baseline configs
	identity, ca, err := loadOrCreateManagerIdentity(sec)
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	_, cli0, err := newTLSConfigs(identity, ca, sec, nil)
	if err != nil {
		t.Fatalf("tls0: %v", err)
	}
	base := len(cli0.RootCAs.Subjects())

	// Now include an inline PEM entry
	inline := string(genSelfSignedPEM(t))
	_, cli1, err := newTLSConfigs(identity, ca, sec, []string{inline})
	if err != nil {
		t.Fatalf("tls1: %v", err)
	}
	got := len(cli1.RootCAs.Subjects())
	if got != base+1 {
		t.Fatalf("expected subjects %d, got %d", base+1, got)
	}
}
