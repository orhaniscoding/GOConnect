package diag

import (
	"net"
	"time"

	"goconnect/internal/config"
	tr "goconnect/internal/transport"
)

// Result captures diagnostic outcomes.
type Result struct {
	// STUN
	STUNOK     bool   `json:"stun_ok"`
	PublicEP   string `json:"public_endpoint"`
	STUNServer string `json:"stun_server"`
	STUNRTTms  int    `json:"stun_rtt_ms"`
	// MTU
	MTUOK     bool   `json:"mtu_ok"`
	MTU       int    `json:"mtu"`
	MTUSource string `json:"mtu_source"`
	// Meta
	Errors     []string `json:"errors"`
	DurationMs int      `json:"duration_ms"`
}

// Run executes diagnostics using configured STUN servers and MTU limits.
func Run(cfg *config.Config) *Result {
	start := time.Now()
	res := &Result{Errors: []string{}}

	// STUN Probe
	if len(cfg.StunServers) > 0 {
		server, ep, rtt, err := tr.ProbePublicEndpoint(cfg.StunServers)
		if err == nil {
			res.STUNOK = true
			res.PublicEP = ep
			res.STUNServer = server
			res.STUNRTTms = int(rtt / time.Millisecond)
		} else {
			res.STUNOK = false
			res.Errors = append(res.Errors, "stun: "+err.Error())
		}
	} else {
		res.Errors = append(res.Errors, "stun: no servers configured")
	}

	// MTU estimation: prefer active interface MTU; fallback to config.MTU
	mtu, src := detectMTU(cfg.MTU)
	res.MTU = mtu
	res.MTUOK = mtu > 0
	res.MTUSource = src

	res.DurationMs = int(time.Since(start) / time.Millisecond)
	return res
}

func detectMTU(defaultMTU int) (int, string) {
	ifaces, _ := net.Interfaces()
	best := 0
	name := ""
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		if iface.MTU > best {
			best = iface.MTU
			name = iface.Name
		}
	}
	if best > 0 {
		return best, "interface:" + name
	}
	if defaultMTU > 0 {
		return defaultMTU, "config"
	}
	return 0, "unknown"
}
