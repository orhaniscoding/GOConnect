
//go:build windows && wintun

package tun
// Read reads a packet from the TUN device.
func (d *device) Read(b []byte) (int, error) {
	if d.dev == nil {
		return 0, fmt.Errorf("device not up")
	}
	return d.dev.Read(b, 0)
}

// Write writes a packet to the TUN device.
func (d *device) Write(b []byte) (int, error) {
	if d.dev == nil {
		return 0, fmt.Errorf("device not up")
	}
	return d.dev.Write(b, 0)
}

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"

	wgTun "golang.zx2c4.com/wireguard/tun"
)

// Real Wintun-backed implementation (enabled with -tags=wintun).

// SetAddress: Controller'dan gelen IP'yi arayüze uygula
func (d *device) SetAddress(ip string) error {
	if d.ifName == "" {
		d.ifName = "GOConnect"
	}
	cmd := exec.Command("netsh", "interface", "ipv4", "set", "address",
		fmt.Sprintf("name=%s", d.ifName), "static", ip, "255.255.255.255")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("netsh set address: %w", err)
	}
	return nil
}
//go:build windows && wintun

package tun

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"

	wgTun "golang.zx2c4.com/wireguard/tun"
)

// Real Wintun-backed implementation (enabled with -tags=wintun).
type device struct {
	ifName string
	mtu    int
	dev    wgTun.Device
}

func New() Device { return &device{ifName: "GOConnect", mtu: 1280} }

func (d *device) Up() error {
	if d.dev != nil {
		return nil
	}
	dev, err := wgTun.CreateTUN(d.ifName, d.mtu)
	if err != nil {
		return err
	}
	d.dev = dev
	if err := configureInterface(d.ifName); err != nil {
		_ = d.dev.Close()
		d.dev = nil
		return err
	}
	// allow interface to settle before loopback tests
	time.Sleep(250 * time.Millisecond)
	return nil
}

func (d *device) Down() error {
	if d.dev != nil {
		_ = d.dev.Close()
		d.dev = nil
	}
	return nil
}

func (d *device) IsUp() bool { return d.dev != nil }

func (d *device) LoopbackTest(ctx context.Context) error {
	if d.dev == nil {
		return fmt.Errorf("device not ready")
	}
	testIP := net.ParseIP("100.64.0.2")
	if testIP == nil {
		return fmt.Errorf("invalid test ip")
	}
	listenAddr := &net.UDPAddr{IP: testIP, Port: 43000}
	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	}

	sender, err := net.DialUDP("udp4", &net.UDPAddr{IP: testIP, Port: 0}, listenAddr)
	if err != nil {
		return fmt.Errorf("dial udp: %w", err)
	}
	defer sender.Close()

	payload := []byte("goconnect-loopback")
	if _, err := sender.Write(payload); err != nil {
		return fmt.Errorf("write udp: %w", err)
	}

	buf := make([]byte, len(payload))
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return fmt.Errorf("read udp: %w", err)
	}
	if n != len(payload) || !bytes.Equal(buf[:n], payload) {
		return fmt.Errorf("loopback payload mismatch")
	}
	return nil
}

func configureInterface(name string) error {
	// Assign point-to-point address (controller will push routes later on).
	cmd := exec.Command("netsh", "interface", "ipv4", "set", "address",
		fmt.Sprintf("name=%s", name), "static", "100.64.0.2", "255.255.255.255")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("netsh set address: %w", err)
	}
	return nil
}
