package controllerclient

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

// ApplyEffectivePolicy applies the given policy to the agent's state/network runtime.
// This is a skeleton; actual implementation will depend on the agent's runtime and network stack.
func ApplyEffectivePolicy(policy *EffectivePolicy) error {
	// TODO: Integrate with agent's runtime/network stack.
	// Example: Update firewall, routing, DNS, file sharing, etc.
	return nil
}
