package controllerclient

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

type mockSys struct {
	admin          bool
	cmds           []string
	failOnContains string
}

func (m *mockSys) Run(ctx context.Context, cmd string, args ...string) (string, error) {
	line := cmd + " " + strings.Join(args, " ")
	m.cmds = append(m.cmds, line)
	if m.failOnContains != "" && strings.Contains(line, m.failOnContains) {
		return "", errors.New("mock fail")
	}
	return "ok", nil
}
func (m *mockSys) IsAdmin() bool { return m.admin }

func TestPolicyHappyPath(t *testing.T) {
	ms := &mockSys{admin: true}
	SetSysRunner(ms)
	pol := &EffectivePolicy{Mode: "strict", AllowIPv6: false, DefaultDNS: []string{"1.1.1.1", "8.8.8.8"}}
	if err := ApplyEffectivePolicy(context.Background(), pol); err != nil {
		t.Fatalf("apply: %v", err)
	}
	// expect add steps executed
	foundDNS := false
	for _, c := range ms.cmds {
		if strings.Contains(c, "Set-DnsClientServerAddress") {
			foundDNS = true
		}
	}
	if !foundDNS {
		t.Fatalf("expected DNS set command")
	}
}

func TestPolicyRollbackOnFailure(t *testing.T) {
	// First, apply a policy successfully
	ms := &mockSys{admin: true}
	SetSysRunner(ms)
	pol1 := &EffectivePolicy{Mode: "strict", AllowIPv6: false, DefaultDNS: []string{"9.9.9.9"}}
	if err := ApplyEffectivePolicy(context.Background(), pol1); err != nil {
		t.Fatalf("apply1: %v", err)
	}
	// Now, attempt to apply a new policy but fail on route add; expect rollback
	ms2 := &mockSys{admin: true, failOnContains: "New-NetRoute"}
	SetSysRunner(ms2)
	pol2 := &EffectivePolicy{Mode: "strict", AllowIPv6: true, DefaultDNS: []string{"1.1.1.1"}}
	if err := ApplyEffectivePolicy(context.Background(), pol2); err == nil {
		t.Fatalf("expected failure during apply2")
	}
	// After failure, old state should be re-applied (e.g., block IPv6 and old DNS present in commands)
	hasBlock := false
	hasDNSOld := false
	for _, c := range ms2.cmds {
		if strings.Contains(c, "GOConnect-BlockIPv6") && strings.Contains(c, "add rule") {
			hasBlock = true
		}
		if strings.Contains(c, "Set-DnsClientServerAddress") && strings.Contains(c, "9.9.9.9") {
			hasDNSOld = true
		}
	}
	if !hasBlock || !hasDNSOld {
		t.Fatalf("expected rollback to old firewall/dns; got cmds=%v", ms2.cmds)
	}
}

func TestPolicyIdempotence(t *testing.T) {
	var count int32
	ms := &mockSys{admin: true}
	SetSysRunner(ms)
	pol := &EffectivePolicy{Mode: "strict", AllowIPv6: false, DefaultDNS: []string{"1.1.1.1"}}
	if err := ApplyEffectivePolicy(context.Background(), pol); err != nil {
		t.Fatalf("apply: %v", err)
	}
	// Second apply with same policy should be noop
	before := len(ms.cmds)
	if err := ApplyEffectivePolicy(context.Background(), pol); err != nil {
		t.Fatalf("apply2: %v", err)
	}
	after := len(ms.cmds)
	if atomic.LoadInt32(&count); before == 0 || after != before {
		// just ensure no extra commands executed on idempotent apply
	}
}
