//go:build !windows || !wintun

package tun

import "context"

// TODO v1.x: Integrate golang.zx2c4.com/wireguard/tun (Wintun) and implement packet loopback test.

type stub struct{ up bool }

func New() Device                                      { return &stub{} }
func (s *stub) Up() error                              { s.up = true; return nil }
func (s *stub) Down() error                            { s.up = false; return nil }
func (s *stub) IsUp() bool                             { return s.up }
func (s *stub) LoopbackTest(ctx context.Context) error { return nil }
func (s *stub) SetAddress(ip string) error             { return nil }
func (s *stub) Read(b []byte) (int, error)             { return 0, nil }
func (s *stub) Write(b []byte) (int, error)            { return len(b), nil }
