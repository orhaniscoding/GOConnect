package controllerclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type NetworkSettings struct {
	Mode               string    `json:"mode"`
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

func FetchNetworkSettings(controllerURL, networkID string) (*NetworkSettings, error) {
	url := fmt.Sprintf("%s/api/v1/networks/%s/settings", controllerURL, networkID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var ns NetworkSettings
	if err := json.NewDecoder(resp.Body).Decode(&ns); err != nil {
		return nil, err
	}
	return &ns, nil
}

func FetchMembershipPreferences(controllerURL, networkID, nodeID string) (*MembershipPreferences, error) {
	url := fmt.Sprintf("%s/api/v1/networks/%s/me/preferences", controllerURL, networkID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Node-ID", nodeID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var mp MembershipPreferences
	if err := json.NewDecoder(resp.Body).Decode(&mp); err != nil {
		return nil, err
	}
	return &mp, nil
}
