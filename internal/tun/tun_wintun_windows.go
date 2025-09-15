//go:build windows && wintun

package tun

import (
    "fmt"
    "os/exec"

    wgTun "golang.zx2c4.com/wireguard/tun"
)

// Real Wintun-backed implementation (enabled with -tags=wintun). Requires admin and wintun.dll available.

type device struct {
    ifName string
    mtu    int
    up     bool
    dev    wgTun.Device
}

func New() Device { return &device{ifName: "GOConnect", mtu: 1280} }

func (d *device) Up() error {
    if d.up {
        return nil
    }
    // Create or open TUN device
    t, err := wgTun.CreateTUN(d.ifName, d.mtu)
    if err != nil {
        return err
    }
    d.dev = t
    // Set IPv4 address via netsh (admin required). Example: 100.64.0.2/32
    _ = exec.Command("netsh", "interface", "ipv4", "set", "address", fmt.Sprintf("name=%s", d.ifName), "static", "100.64.0.2", "255.255.255.255").Run()
    d.up = true
    return nil
}

func (d *device) Down() error {
    if d.dev != nil {
        _ = d.dev.Close()
        d.dev = nil
    }
    d.up = false
    return nil
}

func (d *device) IsUp() bool { return d.up }

