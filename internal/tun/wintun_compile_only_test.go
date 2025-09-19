//go:build windows && wintun

package tun

import "testing"

// This test only verifies that the wintun-backed implementation compiles and can be referenced.
// It does not attempt to bring up the interface or require administrative privileges.
func TestWintunCompileOnly(t *testing.T) {
	// Construct and ensure methods are callable without panicking when not up.
	d := New()
	if d == nil {
		t.Fatal("New() returned nil")
	}
	if d.IsUp() { // should be false initially
		t.Fatal("device unexpectedly up")
	}
}
