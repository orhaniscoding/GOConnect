package controllerclient

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// EffectivePolicy represents the merged policy for a node in a network.
type EffectivePolicy struct {
	Mode               string
	AllowAll           bool
	AllowFileShare     bool
	AllowServiceDisc   bool
	AllowPeerPing      bool
	AllowQuicDirect    bool
	AllowRelayFallback bool
	AllowBroadcast     bool
	AllowIPv6          bool
	MTU                int
	DefaultDNS         []string
	GameProfile        string
	RequireEncryption  bool
	IdleDisconnectMin  int
}

// Sys abstracts command execution for OS changes to allow mocking in tests.
type Sys interface {
	Run(ctx context.Context, cmd string, args ...string) (stdout string, err error)
	IsAdmin() bool
}

// global state for last applied policy and the system runner
var (
	muApplied     sync.Mutex
	appliedPolicy *EffectivePolicy
	sysRunner     Sys
	defaultIface  = "GOConnect"
)

// SetSysRunner allows tests to inject a mock system runner.
func SetSysRunner(s Sys) { sysRunner = s }

// ApplyEffectivePolicy applies the given policy to Windows using netsh/powershell with rollback.
func ApplyEffectivePolicy(ctx context.Context, pol *EffectivePolicy) error {
	if pol == nil {
		return errors.New("nil policy")
	}
	muApplied.Lock()
	defer muApplied.Unlock()

	if sysRunner == nil {
		sysRunner = &noopSys{}
	}
	if !sysRunner.IsAdmin() {
		return errors.New("admin_required: policy application needs Administrator")
	}

	// Interface alias override via env for local dev/testing
	if v := strings.TrimSpace(os.Getenv("GOCONNECT_IFACE_ALIAS")); v != "" {
		defaultIface = v
	}

	old := appliedPolicy
	if old != nil && equalPoliciesOnAppliedFields(old, pol) {
		// idempotent no-op
		log.Printf("policy action=noop reason=equal iface=%s", defaultIface)
		return nil
	}

	// Plan: first remove old artifacts, then add new
	plan := buildPlan(old, pol, defaultIface)

	// Execute with rollback
	var done []step
	for i, st := range plan {
		if err := st.do(ctx, sysRunner); err != nil {
			log.Printf("policy action=%s status=error step=%d err=%v", st.name, i, err)
			// rollback
			for j := len(done) - 1; j >= 0; j-- {
				_ = done[j].undo(ctx, sysRunner)
			}
			// Re-apply old fully to be safe
			if old != nil {
				_ = applyAll(ctx, sysRunner, buildAddNew(old, defaultIface))
			}
			return fmt.Errorf("apply failed at step %s: %w", st.name, err)
		}
		done = append(done, st)
	}
	appliedPolicy = clonePolicy(pol)
	return nil
}

// step is an atomic reversible action
type step struct {
	name string
	do   func(context.Context, Sys) error
	undo func(context.Context, Sys) error
}

func applyAll(ctx context.Context, s Sys, steps []step) error {
	for _, st := range steps {
		if err := st.do(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// buildPlan creates remove(old) + add(new) action list
func buildPlan(old, new *EffectivePolicy, iface string) []step {
	out := make([]step, 0, 8)
	out = append(out, buildRemoveOld(old, iface)...)
	out = append(out, buildAddNew(new, iface)...)
	return out
}

func buildRemoveOld(old *EffectivePolicy, iface string) []step {
	if old == nil {
		return nil
	}
	var steps []step
	// Firewall IPv6 rule removal if previously blocked (AllowIPv6 was false)
	if !old.AllowIPv6 {
		steps = append(steps, step{
			name: "fw_delete_block_ipv6",
			do: func(ctx context.Context, s Sys) error {
				_, err := s.Run(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", "name=GOConnect-BlockIPv6")
				return err
			},
			undo: func(ctx context.Context, s Sys) error {
				// re-add block in undo of removal
				_, err := s.Run(ctx, "netsh", "advfirewall", "firewall", "add", "rule", "name=GOConnect-BlockIPv6", "dir=out", "action=block", "protocol=IPv6")
				return err
			},
		})
	}
	// Route for strict mode removal
	if strings.EqualFold(old.Mode, "strict") {
		steps = append(steps, step{
			name: "route_delete_cgnat",
			do: func(ctx context.Context, s Sys) error {
				// Remove route for 100.64.0.0/10
				_, err := s.Run(ctx, "powershell", "-NoProfile", "-Command", "Remove-NetRoute -ErrorAction SilentlyContinue -DestinationPrefix '100.64.0.0/10' -InterfaceAlias '"+iface+"'")
				return err
			},
			undo: func(ctx context.Context, s Sys) error {
				// Re-add route
				_, err := s.Run(ctx, "powershell", "-NoProfile", "-Command", "New-NetRoute -DestinationPrefix '100.64.0.0/10' -InterfaceAlias '"+iface+"' -NextHop 0.0.0.0 -RouteMetric 10")
				return err
			},
		})
	}
	// DNS reset to DHCP if old had overrides
	if len(old.DefaultDNS) > 0 {
		steps = append(steps, step{
			name: "dns_reset_old",
			do: func(ctx context.Context, s Sys) error {
				_, err := s.Run(ctx, "powershell", "-NoProfile", "-Command", "Set-DnsClientServerAddress -InterfaceAlias '"+iface+"' -ResetServerAddresses")
				return err
			},
			undo: func(ctx context.Context, s Sys) error {
				// Re-apply old DNS list
				args := "Set-DnsClientServerAddress -InterfaceAlias '" + iface + "' -ServerAddresses ('" + strings.Join(old.DefaultDNS, "','") + "')"
				_, err := s.Run(ctx, "powershell", "-NoProfile", "-Command", args)
				return err
			},
		})
	}
	return steps
}

func buildAddNew(new *EffectivePolicy, iface string) []step {
	var steps []step
	if new == nil {
		return steps
	}
	// Firewall: if AllowIPv6 is false, add block rule
	if !new.AllowIPv6 {
		steps = append(steps, step{
			name: "fw_add_block_ipv6",
			do: func(ctx context.Context, s Sys) error {
				_, err := s.Run(ctx, "netsh", "advfirewall", "firewall", "add", "rule", "name=GOConnect-BlockIPv6", "dir=out", "action=block", "protocol=IPv6")
				return err
			},
			undo: func(ctx context.Context, s Sys) error {
				_, err := s.Run(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", "name=GOConnect-BlockIPv6")
				return err
			},
		})
	}
	// Route: strict mode add
	if strings.EqualFold(new.Mode, "strict") {
		steps = append(steps, step{
			name: "route_add_cgnat",
			do: func(ctx context.Context, s Sys) error {
				_, err := s.Run(ctx, "powershell", "-NoProfile", "-Command", "New-NetRoute -DestinationPrefix '100.64.0.0/10' -InterfaceAlias '"+iface+"' -NextHop 0.0.0.0 -RouteMetric 10")
				return err
			},
			undo: func(ctx context.Context, s Sys) error {
				_, err := s.Run(ctx, "powershell", "-NoProfile", "-Command", "Remove-NetRoute -DestinationPrefix '100.64.0.0/10' -InterfaceAlias '"+iface+"' -Confirm:$false")
				return err
			},
		})
	}
	// DNS: override if provided
	if len(new.DefaultDNS) > 0 {
		steps = append(steps, step{
			name: "dns_set",
			do: func(ctx context.Context, s Sys) error {
				args := "Set-DnsClientServerAddress -InterfaceAlias '" + iface + "' -ServerAddresses ('" + strings.Join(new.DefaultDNS, "','") + "')"
				_, err := s.Run(ctx, "powershell", "-NoProfile", "-Command", args)
				return err
			},
			undo: func(ctx context.Context, s Sys) error {
				_, err := s.Run(ctx, "powershell", "-NoProfile", "-Command", "Set-DnsClientServerAddress -InterfaceAlias '"+iface+"' -ResetServerAddresses")
				return err
			},
		})
	}
	return steps
}

func equalPoliciesOnAppliedFields(a, b *EffectivePolicy) bool {
	if a == nil || b == nil {
		return false
	}
	if !strings.EqualFold(a.Mode, b.Mode) {
		return false
	}
	if a.AllowIPv6 != b.AllowIPv6 {
		return false
	}
	if len(a.DefaultDNS) != len(b.DefaultDNS) {
		return false
	}
	for i := range a.DefaultDNS {
		if a.DefaultDNS[i] != b.DefaultDNS[i] {
			return false
		}
	}
	return true
}

func clonePolicy(p *EffectivePolicy) *EffectivePolicy {
	if p == nil {
		return nil
	}
	cp := *p
	if p.DefaultDNS != nil {
		cp.DefaultDNS = append([]string(nil), p.DefaultDNS...)
	}
	return &cp
}

// noopSys is a default placeholder that pretends to be admin but does nothing.
type noopSys struct{}

func (n *noopSys) Run(ctx context.Context, cmd string, args ...string) (string, error) {
	return "", nil
}
func (n *noopSys) IsAdmin() bool { return true }
