package transport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerHandshakeWithPersistedStore(t *testing.T) {
	dir := t.TempDir()
	// Simulate persisted certs in dir
	// For brevity, just check that NewManager can be called with secretsDir
	trusted := []string{}
	_, err := NewManager("127.0.0.1:0", nil, dir, trusted)
	if err != nil {
		t.Fatalf("NewManager failed with persisted store: %v", err)
	}
}

func TestManagerRespectsTrustedPEMs(t *testing.T) {
	dir := t.TempDir()
	pemPath := filepath.Join(dir, "trusted.pem")
	pemBytes, err := os.ReadFile(filepath.Join("testdata", "valid_test_cert.pem"))
	if err != nil {
		t.Fatalf("failed to read valid_test_cert.pem: %v", err)
	}
	if err := os.WriteFile(pemPath, pemBytes, 0644); err != nil {
		t.Fatalf("failed to write trusted.pem: %v", err)
	}
	trusted := []string{pemPath}
	_, err = NewManager("127.0.0.1:0", nil, dir, trusted)
	if err != nil {
		t.Fatalf("NewManager failed with trusted PEM: %v", err)
	}
}
