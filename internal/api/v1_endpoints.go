//go:build windows
// +build windows

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	v10 "github.com/go-playground/validator/v10"
)

// EffectivePolicy is derived view of settings + preferences.
type EffectivePolicy struct {
	Policy             string   `json:"policy"`
	Reason             string   `json:"reason"`
	NetworkMTU         int      `json:"network_mtu"`
	NetworkPort        int      `json:"network_port"`
	AllowInternet      bool     `json:"allow_internet"`
	EncryptionRequired bool     `json:"encryption_required"`
	RelayFallback      bool     `json:"relay_fallback"`
	BroadcastAllowed   bool     `json:"broadcast_allowed"`
	IPv6Allowed        bool     `json:"ipv6_allowed"`
	IdleDisconnectMin  int      `json:"idle_disconnect_minutes"`
	DefaultDNS         []string `json:"default_dns,omitempty"`
}

// GET/PUT /api/v1/networks/{networkId}/me/preferences
func (a *API) handleMemberPreferences(w http.ResponseWriter, r *http.Request) (int, any) {
	nid, ok := NetworkIDFromContext(r.Context())
	if !ok {
		return errPayload(404, "network_not_found", "network not found")
	}
	k := nid + "/me"
	switch r.Method {
	case http.MethodGet:
		a.netMu.RLock()
		prefs, ok := a.memberPreferences[k]
		a.netMu.RUnlock()
		if !ok {
			return errPayload(404, "prefs_not_found", "preferences not found")
		}
		return 200, prefs
	case http.MethodPut:
		var in MemberPreferencesState
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			return errPayload(400, "bad_json", "invalid json")
		}
		// validate payload if enabled
		if err := a.validatePayload(in); err != nil {
			details := err.Error()
			if verrs, ok := err.(v10.ValidationErrors); ok {
				arr := make([]string, 0, len(verrs))
				for _, fe := range verrs {
					arr = append(arr, fe.Namespace()+": "+fe.Tag())
				}
				return errPayloadWithDetails(400, "invalid_payload", "validation failed", arr)
			}
			return errPayloadWithDetails(400, "invalid_payload", "validation failed", details)
		}
		if in.Nickname == "" {
			in.Nickname = "me"
		}
		updated, err := a.updateMemberPreferences(k, &in)
		if err != nil {
			if errors.Is(err, errConflict) {
				return errPayload(409, "version_conflict", "version mismatch")
			}
			return errPayload(400, "update_failed", err.Error())
		}
		_ = a.saveAll() // best-effort persistence
		return 200, updated
	default:
		return 405, nil
	}
}

// GET /api/v1/networks/{networkId}/effective?node=me
func (a *API) handleEffectivePolicy(w http.ResponseWriter, r *http.Request) (int, any) {
	nid, ok := NetworkIDFromContext(r.Context())
	if !ok {
		return errPayload(404, "network_not_found", "network not found")
	}
	a.netMu.RLock()
	settings := a.networkSettings[nid]
	prefs := a.memberPreferences[nid+"/me"]
	a.netMu.RUnlock()
	if settings == nil || prefs == nil {
		return errPayload(404, "not_found", "settings or prefs missing")
	}
	policy := "allow_all"
	reason := "baseline allow"
	if !prefs.AllowInternet {
		policy = "restricted_no_internet"
		reason = "member disabled internet access"
	}
	if settings.RestrictNewMembers {
		reason += "; restricted_new_members"
	}
	if settings.RequireEncryption {
		reason += "; encryption required"
	}
	return 200, EffectivePolicy{
		Policy:             policy,
		Reason:             reason,
		NetworkMTU:         settings.MTU,
		NetworkPort:        settings.Port,
		AllowInternet:      prefs.AllowInternet,
		EncryptionRequired: settings.RequireEncryption,
		RelayFallback:      settings.AllowRelayFallback,
		BroadcastAllowed:   settings.AllowBroadcast,
		IPv6Allowed:        settings.AllowIPv6,
		IdleDisconnectMin:  settings.IdleDisconnectMin,
		DefaultDNS:         settings.DefaultDNS,
	}
}

// GET/PUT /api/v1/networks/{networkId}/settings
func (a *API) handleNetworkSettings(w http.ResponseWriter, r *http.Request) (int, any) {
	nid, ok := NetworkIDFromContext(r.Context())
	if !ok {
		return errPayload(404, "network_not_found", "network not found")
	}
	switch r.Method {
	case http.MethodGet:
		a.netMu.RLock()
		ns, ok := a.networkSettings[nid]
		a.netMu.RUnlock()
		if !ok || ns == nil {
			return errPayload(404, "settings_not_found", "settings not found")
		}
		return 200, ns
	case http.MethodPut:
		var in NetworkSettingsState
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			return errPayload(400, "bad_json", "invalid json")
		}
		if err := a.validatePayload(in); err != nil {
			details := err.Error()
			if verrs, ok := err.(v10.ValidationErrors); ok {
				arr := make([]string, 0, len(verrs))
				for _, fe := range verrs {
					arr = append(arr, fe.Namespace()+": "+fe.Tag())
				}
				return errPayloadWithDetails(400, "invalid_payload", "validation failed", arr)
			}
			return errPayloadWithDetails(400, "invalid_payload", "validation failed", details)
		}
		updated, err := a.updateNetworkSettings(nid, &in)
		if err != nil {
			if errors.Is(err, errConflict) {
				return errPayload(409, "version_conflict", "version mismatch")
			}
			return errPayload(400, "update_failed", err.Error())
		}
		_ = a.saveAll() // best-effort persistence
		return 200, updated
	default:
		return 405, nil
	}
}

// --- internal helpers ---
var errConflict = errors.New("version conflict")

func (a *API) updateNetworkSettings(nid string, in *NetworkSettingsState) (*NetworkSettingsState, error) {
	a.netMu.Lock()
	defer a.netMu.Unlock()
	cur, ok := a.networkSettings[nid]
	if !ok || cur == nil {
		in.Version = 1
		if in.MTU == 0 {
			in.MTU = a.cfg.MTU
		}
		if in.Port == 0 {
			in.Port = a.cfg.Port
		}
		a.networkSettings[nid] = in
		return in, nil
	}
	// optimistic concurrency: client must send same Version
	if in.Version != cur.Version {
		return nil, errConflict
	}
	if in.MTU != 0 {
		cur.MTU = in.MTU
	}
	if in.Port != 0 {
		cur.Port = in.Port
	}
	cur.AllowAll = in.AllowAll
	if in.Mode != "" {
		cur.Mode = in.Mode
	}
	cur.AllowFileShare = in.AllowFileShare
	cur.AllowServiceDisc = in.AllowServiceDisc
	cur.AllowPeerPing = in.AllowPeerPing
	cur.AllowRelayFallback = in.AllowRelayFallback
	cur.AllowBroadcast = in.AllowBroadcast
	cur.AllowIPv6 = in.AllowIPv6
	cur.AllowChat = in.AllowChat
	if in.MTUOverride >= 0 {
		cur.MTUOverride = in.MTUOverride
	}
	if in.DefaultDNS != nil {
		cur.DefaultDNS = in.DefaultDNS
	}
	if in.GameProfile != "" {
		cur.GameProfile = in.GameProfile
	}
	cur.RequireEncryption = in.RequireEncryption
	cur.RestrictNewMembers = in.RestrictNewMembers
	if in.IdleDisconnectMin >= 0 {
		cur.IdleDisconnectMin = in.IdleDisconnectMin
	}
	cur.Version++
	return cur, nil
}

func (a *API) updateMemberPreferences(k string, in *MemberPreferencesState) (*MemberPreferencesState, error) {
	a.netMu.Lock()
	defer a.netMu.Unlock()
	cur, ok := a.memberPreferences[k]
	if !ok || cur == nil {
		in.Version = 1
		a.memberPreferences[k] = in
		return in, nil
	}
	if in.Version != cur.Version {
		return nil, errConflict
	}
	cur.AllowInternet = in.AllowInternet
	if in.Nickname != "" {
		cur.Nickname = in.Nickname
	}
	cur.LocalShareEnabled = in.LocalShareEnabled
	cur.AdvertiseServices = in.AdvertiseServices
	cur.AllowIncomingP2P = in.AllowIncomingP2P
	cur.ChatEnabled = in.ChatEnabled
	if in.Alias != "" {
		cur.Alias = in.Alias
	}
	if in.Notes != "" {
		cur.Notes = in.Notes
	}
	cur.Version++
	return cur, nil
}

// optional parse helpers (future use)
func parseInt(str string) int {
	if str == "" {
		return 0
	}
	if v, _ := strconv.Atoi(str); v > 0 {
		return v
	}
	return 0
}
