package models

// NetworkSettings ve MembershipPreferences birleşiminden efektif policy hesaplar
func ComputeEffectivePolicy(ns NetworkSettings, mp MembershipPreferences) EffectivePolicy {
	networkOn := func(flag bool) bool {
		if ns.AllowAll {
			return true
		}
		return flag
	}
	return EffectivePolicy{
		FileShare:          networkOn(ns.AllowFileShare) && mp.LocalShareEnabled,
		ServiceDiscovery:   networkOn(ns.AllowServiceDisc) && mp.AdvertiseServices,
		PeerPing:           networkOn(ns.AllowPeerPing),
		QuicDirect:         networkOn(ns.AllowQuicDirect) && mp.AllowIncomingP2P,
		RelayFallback:      networkOn(ns.AllowRelayFallback),
		Broadcast:          networkOn(ns.AllowBroadcast),
		IPv6:               networkOn(ns.AllowIPv6),
		EncryptionRequired: ns.RequireEncryption,
	}
}
