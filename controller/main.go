package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"goconnect/controller/models"
	"goconnect/controller/store"
)

type ControllerConfig struct {
	StoreType string // "sqlite" or "memory"
	DataDir   string
}

func loadConfig() ControllerConfig {
	// For now, just env/config file stub; default to sqlite
	st := os.Getenv("GOCONNECT_STORE_TYPE")
	if st == "" {
		st = "sqlite"
	}
	dir := os.Getenv("GOCONNECT_DATA_DIR")
	if dir == "" {
		dir = "./data"
	}
	return ControllerConfig{
		StoreType: st,
		DataDir:   dir,
	}
}

func openStore(cfg ControllerConfig) (any, error) {
	switch cfg.StoreType {
	case "memory":
		return store.NewInMemoryStore(), nil
	case "sqlite":
		return store.NewSQLiteStore(cfg.DataDir)
	default:
		return nil, errors.New("unknown store type")
	}
}

var s store.ControllerStore

// --- Simple in-memory controller model with ownership ---
type ctrlMember struct {
	NodeID   string    `json:"nodeId"`
	IP       string    `json:"ip"`
	Nickname string    `json:"nickname"`
	JoinedAt time.Time `json:"joined_at"`
}

type ctrlNetwork struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	OwnerToken string                 `json:"-"`
	JoinSecret string                 `json:"-"`
	Members    map[string]*ctrlMember `json:"members"`
	nextOctet  int                    `json:"-"`
}

var (
	ctrlNets = map[string]*ctrlNetwork{}
)

func main() {
	cfg := loadConfig()
	st, err := openStore(cfg)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	s = st.(store.ControllerStore)
	http.HandleFunc("/api/v1/networks/", dispatchNetworkRoutes)
	http.HandleFunc("/api/controller/", dispatchControllerRoutes)
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

// --- Controller endpoints ---
// Routes:
// GET    /api/controller/networks
// POST   /api/controller/networks {id,name,joinSecret}
// DELETE /api/controller/networks/{id}
// POST   /api/controller/networks/{id}/join {nickname,chatEnabled,joinSecret}
// POST   /api/controller/networks/{id}/kick {nodeId}
// GET    /api/controller/networks/{id}/snapshot
func dispatchControllerRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/controller/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	// Collection: /networks
	if len(parts) == 1 && parts[0] == "networks" {
		switch r.Method {
		case http.MethodGet:
			out := []map[string]any{}
			for _, n := range ctrlNets {
				out = append(out, map[string]any{
					"id":      n.ID,
					"name":    n.Name,
					"members": len(n.Members),
				})
			}
			writeJSON(w, 200, map[string]any{"networks": out})
			return
		case http.MethodPost:
			var in struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				JoinSecret string `json:"joinSecret"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.ID) == "" {
				http.Error(w, "bad json", 400)
				return
			}
			if _, ok := ctrlNets[in.ID]; ok {
				http.Error(w, "conflict", 409)
				return
			}
			ctrlNets[in.ID] = &ctrlNetwork{ID: in.ID, Name: in.Name, OwnerToken: token, JoinSecret: strings.TrimSpace(in.JoinSecret), Members: map[string]*ctrlMember{}, nextOctet: 2}
			writeJSON(w, 200, map[string]any{"result": "ok"})
			return
		default:
			http.Error(w, "method not allowed", 405)
			return
		}
	}
	// Item routes: /networks/{id}/...
	if len(parts) >= 2 && parts[0] == "networks" {
		nid := parts[1]
		n := ctrlNets[nid]
		if n == nil {
			http.Error(w, "not found", 404)
			return
		}
		// DELETE /networks/{id}
		if len(parts) == 2 && r.Method == http.MethodDelete {
			if n.OwnerToken != "" && token != n.OwnerToken {
				http.Error(w, "forbidden", 403)
				return
			}
			delete(ctrlNets, nid)
			writeJSON(w, 200, map[string]any{"result": "deleted"})
			return
		}
		if len(parts) == 3 && parts[2] == "join" && r.Method == http.MethodPost {
			var in struct {
				Nickname    string `json:"nickname"`
				ChatEnabled bool   `json:"chatEnabled"`
				JoinSecret  string `json:"joinSecret"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, "bad json", 400)
				return
			}
			// Join secret validation: if set, must match; else set from first provided
			if strings.TrimSpace(n.JoinSecret) != "" {
				if strings.TrimSpace(in.JoinSecret) == "" || in.JoinSecret != n.JoinSecret {
					http.Error(w, "invalid secret", 403)
					return
				}
			} else if strings.TrimSpace(in.JoinSecret) != "" {
				n.JoinSecret = strings.TrimSpace(in.JoinSecret)
			}
			// Owner token: if empty, first joiner with token becomes owner
			if n.OwnerToken == "" && token != "" {
				n.OwnerToken = token
			}
			// Allocate member
			nodeID := randHex()
			ip := fmtIP(n.nextOctet)
			n.nextOctet++
			n.Members[nodeID] = &ctrlMember{NodeID: nodeID, IP: ip, Nickname: strings.TrimSpace(in.Nickname), JoinedAt: time.Now().UTC()}
			writeJSON(w, 200, map[string]any{"nodeId": nodeID, "ip": ip})
			return
		}
		if len(parts) == 3 && parts[2] == "kick" && r.Method == http.MethodPost {
			if n.OwnerToken != "" && token != n.OwnerToken {
				http.Error(w, "forbidden", 403)
				return
			}
			var in struct {
				NodeID string `json:"nodeId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.NodeID) == "" {
				http.Error(w, "bad json", 400)
				return
			}
			delete(n.Members, in.NodeID)
			writeJSON(w, 200, map[string]any{"result": "ok"})
			return
		}
		if len(parts) == 3 && parts[2] == "snapshot" && r.Method == http.MethodGet {
			// Return members and empty chats for now
			members := []any{}
			for _, m := range n.Members {
				members = append(members, m)
			}
			writeJSON(w, 200, map[string]any{"members": members, "chats": []any{}})
			return
		}
	}
	http.NotFound(w, r)
}

func randHex() string { b := make([]byte, 8); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func fmtIP(octet int) string { // 10.83.0.X/24
	if octet < 2 {
		octet = 2
	}
	if octet > 250 {
		octet = 2
	}
	return "10.83.0." + strconv.Itoa(octet) + "/24"
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
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
