//go:build !wintun

package tun

// TODO v1.1: Integrate golang.zx2c4.com/wireguard/tun (Wintun) and implement packet loopback test.

type Device interface {
    Up() error
    Down() error
    IsUp() bool
}

type stub struct{ up bool }

func New() Device { return &stub{} }
func (s *stub) Up() error   { s.up = true; return nil }
func (s *stub) Down() error { s.up = false; return nil }
func (s *stub) IsUp() bool  { return s.up }
