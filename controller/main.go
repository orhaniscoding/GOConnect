package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"controller/models"
	"controller/store"
)

var s = store.NewInMemoryStore()

func main() {
	http.HandleFunc("/api/v1/networks/", dispatchNetworkRoutes)
	log.Println("Controller API listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// dispatchNetworkRoutes routes:
// GET/PUT /api/v1/networks/{id}/settings
// GET/PUT /api/v1/networks/{id}/me/preferences (header X-Node-ID)
// GET     /api/v1/networks/{id}/effective?node=NODEID
func dispatchNetworkRoutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "networks" {
		http.NotFound(w, r)
		return
	}
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}
	networkID := parts[3]
	// /api/v1/networks/{id}/settings
	if len(parts) == 5 && parts[4] == "settings" {
		handleNetworkSettings(w, r, networkID)
		return
	}
	// /api/v1/networks/{id}/me/preferences
	if len(parts) == 7 && parts[4] == "me" && parts[5] == "preferences" {
		nodeID := r.Header.Get("X-Node-ID")
		if nodeID == "" {
			http.Error(w, "missing node id", 400)
			return
		}
		handleMembershipPreferencesV1(w, r, networkID, nodeID)
		return
	}
	// /api/v1/networks/{id}/effective
	if len(parts) == 5 && parts[4] == "effective" && r.Method == http.MethodGet {
		nodeID := r.URL.Query().Get("node")
		if nodeID == "" {
			http.Error(w, "missing node query", 400)
			return
		}
		handleEffectiveV1(w, r, networkID, nodeID)
		return
	}
	http.NotFound(w, r)
}

// GET/PUT /api/v1/netprefs/{networkID}/{nodeID}
func handleMembershipPreferencesV1(w http.ResponseWriter, r *http.Request, networkID, nodeID string) {
	switch r.Method {
	case http.MethodGet:
		prefs, ok := s.GetMembershipPreferences(networkID, nodeID)
		if !ok {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prefs)
	case http.MethodPut:
		var mp models.MembershipPreferences
		if err := json.NewDecoder(r.Body).Decode(&mp); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		mp.UpdatedAt = time.Now().UTC()
		s.SetMembershipPreferences(networkID, nodeID, &mp)
		w.WriteHeader(200)
		w.Write([]byte(`{"result":"ok"}`))
	default:
		http.Error(w, "method not allowed", 405)
	}
}

// GET /api/v1/effective/{networkID}/{nodeID}
func handleEffectiveV1(w http.ResponseWriter, r *http.Request, networkID, nodeID string) {
	ns, ok1 := s.GetNetworkSettings(networkID)
	mp, ok2 := s.GetMembershipPreferences(networkID, nodeID)
	if !ok1 || !ok2 {
		http.Error(w, "not found", 404)
		return
	}
	eff := models.ComputeEffectivePolicy(*ns, *mp)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eff)
}

// GET /api/v1/networks/{id}/settings
// PUT /api/v1/networks/{id}/settings
func handleNetworkSettings(w http.ResponseWriter, r *http.Request, networkID string) {
	switch r.Method {
	case http.MethodGet:
		settings, ok := s.GetNetworkSettings(networkID)
		if !ok {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	case http.MethodPut:
		var ns models.NetworkSettings
		if err := json.NewDecoder(r.Body).Decode(&ns); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		// Optimistic lock: version kontrolü
		old, exists := s.GetNetworkSettings(networkID)
		if exists && ns.Version != old.Version {
			http.Error(w, "version conflict", 409)
			return
		}
		// Preset uygula (mode)
		if ns.Mode == "full" {
			ns.AllowAll = true
		}
		if ns.Mode == "game_lan" {
			ns.AllowAll = false
			ns.AllowBroadcast = true
			ns.AllowQuicDirect = true
			ns.AllowRelayFallback = false
			ns.AllowFileShare = false
			ns.AllowServiceDisc = false
		}
		if ns.Mode == "fileshare" {
			ns.AllowAll = false
			ns.AllowFileShare = true
			ns.AllowServiceDisc = true
			ns.AllowBroadcast = false
		}
		// Validation
		if ns.MTUOverride != 0 && (ns.MTUOverride < 576 || ns.MTUOverride > 9000) {
			http.Error(w, "invalid mtu", 400)
			return
		}
		if len(ns.DefaultDNS) > 3 {
			http.Error(w, "too many dns", 400)
			return
		}
		if ns.GameProfile != "" && ns.GameProfile != "minecraft" && ns.GameProfile != "valheim" && ns.GameProfile != "rust" {
			http.Error(w, "invalid game_profile", 400)
			return
		}
		if ns.IdleDisconnectMin != 0 && (ns.IdleDisconnectMin < 5 || ns.IdleDisconnectMin > 4320) {
			http.Error(w, "invalid idle_disconnect_minutes", 400)
			return
		}
		ns.Version++
		ns.UpdatedAt = time.Now().UTC()
		s.SetNetworkSettings(networkID, &ns)
		w.WriteHeader(200)
		w.Write([]byte(`{"result":"ok"}`))
	default:
		http.Error(w, "method not allowed", 405)
	}
}
