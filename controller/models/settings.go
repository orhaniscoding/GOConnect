package models

import "time"

// Ağ başı ayarlar (owner tarafından belirlenir)
type NetworkSettings struct {
	Mode               string    `json:"mode"` // generic|game_lan|fileshare|custom|full
	AllowAll           bool      `json:"allow_all"`
	AllowFileShare     bool      `json:"allow_file_share"`
	AllowServiceDisc   bool      `json:"allow_service_discovery"`
	AllowPeerPing      bool      `json:"allow_peer_ping"`
	AllowQuicDirect    bool      `json:"allow_quic_direct"`
	AllowRelayFallback bool      `json:"allow_relay_fallback"`
	AllowBroadcast     bool      `json:"allow_broadcast"`
	AllowIPv6          bool      `json:"allow_ipv6"`
	MTUOverride        int       `json:"mtu_override,omitempty"`
	DefaultDNS         []string  `json:"default_dns,omitempty"`
	GameProfile        string    `json:"game_profile,omitempty"`
	RequireEncryption  bool      `json:"require_encryption"`
	RestrictNewMembers bool      `json:"restrict_new_members"`
	IdleDisconnectMin  int       `json:"idle_disconnect_minutes,omitempty"`
	Version            int       `json:"version"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Üye bazında kişisel tercihler
type MembershipPreferences struct {
	NodeID            string    `json:"node_id"`
	NetworkID         string    `json:"network_id"`
	LocalShareEnabled bool      `json:"local_share_enabled"`
	AdvertiseServices bool      `json:"advertise_services"`
	AllowIncomingP2P  bool      `json:"allow_incoming_p2p"`
	Alias             string    `json:"alias"`
	Notes             string    `json:"notes"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Birleşik (etkili) policy çıktısı
type EffectivePolicy struct {
	FileShare          bool `json:"file_share"`
	ServiceDiscovery   bool `json:"service_discovery"`
	PeerPing           bool `json:"peer_ping"`
	QuicDirect         bool `json:"quic_direct"`
	RelayFallback      bool `json:"relay_fallback"`
	Broadcast          bool `json:"broadcast"`
	IPv6               bool `json:"ipv6"`
	EncryptionRequired bool `json:"encryption_required"`
}
