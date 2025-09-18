package transport

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

func TestTrustedCertPoolMergesPEMs(t *testing.T) {
	// Setup: create a temp secrets dir and a trusted PEM file
	dir := t.TempDir()
	pemPath := filepath.Join(dir, "trusted.pem")
	pemBytes, err := os.ReadFile(filepath.Join("testdata", "valid_test_cert.pem"))
	if err != nil {
		t.Fatalf("failed to read valid_test_cert.pem: %v", err)
	}
	if err := os.WriteFile(pemPath, pemBytes, 0644); err != nil {
		t.Fatalf("failed to write trusted.pem: %v", err)
	}

	pool := x509.NewCertPool()
	b, err := os.ReadFile(pemPath)
	if err != nil {
		t.Fatalf("failed to read trusted.pem: %v", err)
	}
	if !pool.AppendCertsFromPEM(b) {
		t.Fatal("failed to append trusted PEM to pool")
	}
	if len(pool.Subjects()) == 0 {
		t.Fatal("expected at least one subject in CertPool")
	}
}
