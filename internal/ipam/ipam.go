package ipam

import (
	"fmt"
	"hash/crc32"
	"net"
	"sync"
)

type Allocator struct {
	mu       sync.Mutex
	assigned map[string]string
	used     map[string]struct{}
}

func New() *Allocator {
	return &Allocator{
		assigned: map[string]string{},
		used:     map[string]struct{}{},
	}
}

// Reserve records an existing assignment so that future allocations avoid collisions.
func (a *Allocator) Reserve(id, cidr string) {
	if cidr == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.assigned[id] = cidr
	a.used[cidr] = struct{}{}
}

func (a *Allocator) AddressFor(id string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if addr, ok := a.assigned[id]; ok {
		return addr
	}
	base := deriveCandidate(id)
	host := base[3]
	second := base[1]
	third := base[2]
	for attempts := 0; attempts < 253; attempts++ {
		addr := fmt.Sprintf("100.%d.%d.%d/32", second, third, host)
		if _, used := a.used[addr]; !used {
			a.assigned[id] = addr
			a.used[addr] = struct{}{}
			return addr
		}
		host++
		if host >= 255 {
			host = 2
			third++
		}
	}
	fallback := "100.64.0.2/32"
	a.assigned[id] = fallback
	a.used[fallback] = struct{}{}
	return fallback
}

func deriveCandidate(id string) [4]byte {
	sum := crc32.ChecksumIEEE([]byte(id))
	var ip [4]byte
	ip[0] = 100
	ip[1] = byte(64 + (sum % 64))
	ip[2] = byte((sum >> 8) & 0xff)
	host := byte(2 + (sum>>16)%253)
	if host < 2 {
		host = 2
	}
	ip[3] = host
	return ip
}

// ParseCIDR exposes parsing helper for tests or external reservation logic.
func ParseCIDR(cidr string) (*net.IPNet, error) {
	_, network, err := net.ParseCIDR(cidr)
	return network, err
}
