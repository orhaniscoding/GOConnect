//go:build !windows

package security

// Non-Windows builds fall back to identity operations so the agent can run in
// development environments without DPAPI.
func Protect(data []byte) ([]byte, error)   { return append([]byte(nil), data...), nil }
func Unprotect(data []byte) ([]byte, error) { return append([]byte(nil), data...), nil }
