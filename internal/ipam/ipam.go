package ipam

import (
	"fmt"
	"hash/crc32"
)

type Allocator struct{}

func New() *Allocator { return &Allocator{} }

func (a *Allocator) AddressFor(id string) string {
	if id == "" {
		return ""
	}
	sum := crc32.ChecksumIEEE([]byte(id))
	second := byte(64 + (sum % 64))
	third := byte((sum >> 8) & 0xff)
	host := byte(2 + (sum % 250))
	return fmt.Sprintf("100.%d.%d.%d/32", second, third, host)
}
