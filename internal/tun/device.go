package tun

import "context"

// Device abstracts TUN lifecycle primitives used by the service.
type Device interface {
	Up() error
	Down() error
	IsUp() bool
	LoopbackTest(ctx context.Context) error
	SetAddress(ip string) error
	Read([]byte) (int, error)
	Write([]byte) (int, error)
}
