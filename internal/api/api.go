package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"goconnect/internal/api/mw"
	"goconnect/internal/config"
	"goconnect/internal/core"
	"goconnect/internal/diag"
	gi18n "goconnect/internal/i18n"
	"goconnect/internal/ipam"
	"goconnect/internal/metrics"

	webfs "goconnect/webui"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"goconnect/internal/updater"

	v10 "github.com/go-playground/validator/v10"
)

type API struct {
	state     *core.State
	cfg       *config.Config
	cfgMu     sync.Mutex
	logger    *log.Logger
	ipam      *ipam.Allocator
	csrfToken string
	shutdown  func()
	startTime time.Time
	validate  *v10.Validate

	controllerToken string

	sseMu   sync.Mutex
	sseSubs map[chan string]struct{}
	peersFn func() []map[string]any

	// v1 per-network data (in-memory for now)
	netMu             sync.RWMutex
	networkSettings   map[string]*NetworkSettingsState
	memberPreferences map[string]*MemberPreferencesState       // key: networkID+"/"+member (only "me" supported now)
	chatHistories     map[string][]ChatMessage                 // key: networkID
	chatSubs          map[string]map[chan ChatMessage]struct{} // networkID -> subscribers
	chatLast          map[string]time.Time                     // rate limit last send per network+user
}

// ChatMessage represents a single chat line within a network.
type ChatMessage struct {
	ID        string    `json:"id"`
	NetworkID string    `json:"network_id"`
	From      string    `json:"from"`
	Text      string    `json:"text"`
	At        time.Time `json:"at"`
}

// errorResponse standard form
type errorResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

func errPayload(code int, key, msg string) (int, any) {
	if msg == "" {
		msg = key
	}
	return code, errorResponse{Code: key, Message: msg}
}

func errPayloadWithDetails(code int, key, msg string, details any) (int, any) {
	if msg == "" {
		msg = key
	}
	return code, errorResponse{Code: key, Message: msg, Details: details}
}

// context keys and helpers
type ctxKey int

const (
	ctxKeyNetworkID ctxKey = iota
)

func WithNetworkID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyNetworkID, id)
}

func NetworkIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyNetworkID).(string)
	return v, ok
}

// NetworkSettingsState holds versioned settings for a network.
type NetworkSettingsState struct {
	Version            int      `json:"version" validate:"gte=0"`
	MTU                int      `json:"mtu" validate:"omitempty,min=576,max=9000"`
	Port               int      `json:"port" validate:"omitempty,min=1,max=65535"`
	AllowAll           bool     `json:"allow_all"`
	Mode               string   `json:"mode" validate:"omitempty,oneof=default strict gaming custom"`
	AllowFileShare     bool     `json:"allow_file_share"`
	AllowServiceDisc   bool     `json:"allow_service_discovery"`
	AllowPeerPing      bool     `json:"allow_peer_ping"`
	AllowRelayFallback bool     `json:"allow_relay_fallback"`
	AllowBroadcast     bool     `json:"allow_broadcast"`
	AllowIPv6          bool     `json:"allow_ipv6"`
	AllowChat          bool     `json:"allow_chat"`
	MTUOverride        int      `json:"mtu_override" validate:"gte=0"`
	DefaultDNS         []string `json:"default_dns" validate:"dive,hostname|ip"`
	GameProfile        string   `json:"game_profile" validate:"omitempty,oneof=none low latency balanced"`
	RequireEncryption  bool     `json:"require_encryption"`
	RestrictNewMembers bool     `json:"restrict_new_members"`
	IdleDisconnectMin  int      `json:"idle_disconnect_minutes" validate:"gte=0,lte=10080"`
}

// MemberPreferencesState holds versioned member preferences.
type MemberPreferencesState struct {
	Version           int    `json:"version" validate:"gte=0"`
	AllowInternet     bool   `json:"allow_internet"`
	Nickname          string `json:"nickname" validate:"omitempty,min=1,max=64"`
	LocalShareEnabled bool   `json:"local_share_enabled"`
	AdvertiseServices bool   `json:"advertise_services"`
	AllowIncomingP2P  bool   `json:"allow_incoming_p2p"`
	ChatEnabled       bool   `json:"chat_enabled"`
	Alias             string `json:"alias" validate:"omitempty,max=64"`
	Notes             string `json:"notes" validate:"omitempty,max=256"`
}

func New(state *core.State, cfg *config.Config, logger *log.Logger, shutdown func()) *API {
	a := &API{
		state:             state,
		cfg:               cfg,
		logger:            logger,
		ipam:              ipam.New(),
		csrfToken:         randomToken(),
		shutdown:          shutdown,
		sseSubs:           map[chan string]struct{}{},
		networkSettings:   map[string]*NetworkSettingsState{},
		memberPreferences: map[string]*MemberPreferencesState{},
		chatHistories:     map[string][]ChatMessage{},
		chatSubs:          map[string]map[chan ChatMessage]struct{}{},
		chatLast:          map[string]time.Time{},
		startTime:         time.Now(),
	}
	a.validate = newValidator()
	// Load controller token from ProgramData secrets if present
	if _, _, secretsDir := config.Paths(); secretsDir != "" {
		p := filepath.Join(secretsDir, "controller_token.txt")
		if b, err := os.ReadFile(p); err == nil {
			a.controllerToken = strings.TrimSpace(string(b))
		}
	}
	// attempt load persisted state (best-effort)
	_ = a.loadAllOnce()
	state.SetNetworks(mapNetworks(cfg.Networks))
	for _, n := range cfg.Networks {
		if n.Address != "" {
			a.ipam.Reserve(n.ID, n.Address)
		}
		// seed default settings with sensible baseline
		a.networkSettings[n.ID] = &NetworkSettingsState{Version: 1, MTU: cfg.MTU, Port: cfg.Port, Mode: "default", AllowAll: true}
		// seed default member prefs for "me"
		a.memberPreferences[n.ID+"/me"] = &MemberPreferencesState{Version: 1, AllowInternet: true, Nickname: "me"}
	}
	return a
}

// Controller sync loop: ControllerURL varsa join ve snapshot çek
func (a *API) StartControllerSync() {
	url := a.cfg.ControllerURL
	if url == "" {
		return
	}
	go func() {
		client := &http.Client{}
		for _, n := range a.cfg.Networks {
			// Join isteği
			joinBody := map[string]any{
				"nickname":    n.Name,
				"chatEnabled": true,
				"joinSecret":  n.JoinSecret,
			}
			b, _ := json.Marshal(joinBody)
			req, _ := http.NewRequest("POST", url+"/api/controller/networks/"+n.ID+"/join", strings.NewReader(string(b)))
			req.Header.Set("Content-Type", "application/json")
			if a.controllerToken != "" {
				req.Header.Set("Authorization", "Bearer "+a.controllerToken)
			}
			resp, err := client.Do(req)
			if err != nil {
				a.logger.Printf("Controller join hatası: %v", err)
				continue
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				a.logger.Printf("Controller join başarısız: %s", resp.Status)
				continue
			}
			var member struct {
				NodeID string `json:"nodeId"`
				IP     string `json:"ip"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&member)
			a.logger.Printf("Controller'dan IP alındı: %s", member.IP)
			if a.state != nil {
				tun := a.state.TunDevice()
				if tun != nil {
					if err := tun.SetAddress(member.IP); err != nil {
						a.logger.Printf("TUN IP atanamadı: %v", err)
					} else {
						a.logger.Printf("TUN arayüzüne IP atandı: %s", member.IP)
					}
				}
			}
		}
		// Periyodik snapshot çek
		for {
			for _, n := range a.cfg.Networks {
				req, _ := http.NewRequest("GET", url+"/api/controller/networks/"+n.ID+"/snapshot", nil)
				if a.controllerToken != "" {
					req.Header.Set("Authorization", "Bearer "+a.controllerToken)
				}
				resp, err := client.Do(req)
				if err != nil {
					a.logger.Printf("Controller snapshot hatası: %v", err)
					continue
				}
				var snap struct {
					Members []any `json:"members"`
					Chats   []any `json:"chats"`
				}
				_ = json.NewDecoder(resp.Body).Decode(&snap)
				resp.Body.Close()
				// TODO: Üyeler ve chat listesini local state'e uygula
				a.logger.Printf("Controller snapshot: %d üye, %d mesaj", len(snap.Members), len(snap.Chats))
			}
			time.Sleep(5 * time.Second)
		}
	}()
}

func (a *API) Serve(addr string, webDir string) *http.Server {
	mux := http.NewServeMux()

	var handler http.Handler
	if webDir != "" {
		handler = http.FileServer(http.Dir(webDir))
	} else {
		handler = http.FileServer(http.FS(webfs.FS))
	}
	mux.Handle("/", http.StripPrefix("/", handler))

	// Build middleware chain: auth -> rate limit -> handler. Exempt GET /api/status from auth/rate for convenience.
	chain := mw.Chain(
		mw.BearerAuth(func() string { return strings.TrimSpace(a.cfg.Api.BearerToken) }, a.logger),
		mw.RateLimit(a.cfg.Api.RateLimit.RPS, a.cfg.Api.RateLimit.Burst, a.logger),
	)

	mux.HandleFunc("/api/status", a.wrap(a.handleStatus))
	mux.Handle("/api/service/start", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleServiceStart)(w, r) })))
	mux.Handle("/api/service/stop", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleServiceStop)(w, r) })))
	mux.Handle("/api/service/restart", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleServiceRestart)(w, r) })))
	mux.Handle("/api/networks", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrap(a.handleNetworks)(w, r) })))
	mux.Handle("/api/networks/join", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleNetworksJoin)(w, r) })))
	mux.Handle("/api/networks/leave", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleNetworksLeave)(w, r) })))
	mux.Handle("/api/peers", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrap(a.handlePeers)(w, r) })))
	mux.HandleFunc("/api/logs/stream", a.handleLogsStream) // SSE left unauthenticated for now
	mux.Handle("/api/settings", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrap(a.handleSettings)(w, r) })))
	// Controller token management endpoints (agent-side)
	mux.Handle("/api/controller/token/status", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrap(a.handleControllerTokenStatus)(w, r) })))
	mux.Handle("/api/controller/token/set", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleControllerTokenSet)(w, r) })))
	mux.Handle("/api/controller/token/clear", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleControllerTokenClear)(w, r) })))
	// Token management endpoints
	mux.Handle("/api/token/regenerate", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleTokenRegenerate)(w, r) })))
	mux.Handle("/api/token/clear", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleTokenClear)(w, r) })))
	// Optional metrics: if enabled in config, expose a Prometheus-style endpoint at /metrics (no auth middleware)
	if a.cfg != nil && a.cfg.Metrics.Enabled {
		mux.Handle("/metrics", metrics.Handler())
	}
	// Controller proxy: forward /api/controller/* to configured ControllerURL
	mux.Handle("/api/controller/", chain(http.HandlerFunc(a.handleControllerProxy)))
	// --- v1 endpoints for webui integration ---
	mux.Handle("/api/v1/networks/", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route: /api/v1/networks/{networkId}/settings, /me/preferences, /effective
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/networks/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		networkID := parts[0]
		ctx := WithNetworkID(r.Context(), networkID)
		r = r.WithContext(ctx)
		if parts[1] == "settings" && len(parts) == 2 {
			a.wrap(a.handleNetworkSettings)(w, r)
			return
		}
		if parts[1] == "me" && len(parts) == 3 && parts[2] == "preferences" {
			a.wrap(a.handleMemberPreferences)(w, r)
			return
		}
		if parts[1] == "effective" && len(parts) == 2 {
			a.wrap(a.handleEffectivePolicy)(w, r)
			return
		}
		// Chat endpoints: /api/v1/networks/{id}/chat/messages and /chat/stream
		if parts[1] == "chat" && len(parts) >= 3 {
			if parts[2] == "messages" {
				a.handleChatMessages(w, r)
				return
			}
			if parts[2] == "stream" {
				a.handleChatStream(w, r)
				return
			}
		}
		http.NotFound(w, r)
	})))
	// --- end v1 endpoints ---
	mux.Handle("/api/diag/run", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleDiagRun)(w, r) })))
	mux.Handle("/api/update/check", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleUpdateCheck)(w, r) })))
	mux.Handle("/api/update/apply", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleUpdateApply)(w, r) })))
	mux.Handle("/api/exit", chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.wrapPOST(a.handleExit)(w, r) })))

	srv := &http.Server{Addr: addr, Handler: a.csrfMiddleware(mux)}

	go a.fakeLogPump()
	return srv
}

// --- Validation helpers ---
func newValidator() *v10.Validate {
	v := v10.New()
	return v
}

func (a *API) validatePayload(v any) error {
	if a.cfg != nil && !a.cfg.Api.Validation {
		return nil
	}
	if a.validate == nil {
		a.validate = newValidator()
	}
	return a.validate.Struct(v)
}

func (a *API) SetPeersFn(f func() []map[string]any) { a.peersFn = f }

func (a *API) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "goc_csrf",
			Value:    a.csrfToken,
			HttpOnly: true, // Prevent JS access
			Secure:   r.TLS != nil,
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
		})

		// Also provide bearer token via HttpOnly cookie for Web UI convenience.
		// This allows the UI to authenticate without exposing the token to JS.
		// When token is empty, clear the cookie.
		tok := strings.TrimSpace(a.cfg.Api.BearerToken)
		if tok != "" {
			http.SetCookie(w, &http.Cookie{
				Name:     "goc_bearer",
				Value:    tok,
				HttpOnly: true,
				Secure:   r.TLS != nil,
				Path:     "/",
				SameSite: http.SameSiteStrictMode,
			})
		} else {
			http.SetCookie(w, &http.Cookie{
				Name:     "goc_bearer",
				Value:    "",
				HttpOnly: true,
				Secure:   r.TLS != nil,
				Path:     "/",
				SameSite: http.SameSiteStrictMode,
				MaxAge:   -1, // delete
			})
		}

		if r.Method != http.MethodGet {
			// Loopback requests to /api/exit are allowed without CSRF (local management convenience).
			if r.URL.Path == "/api/exit" && isLoopback(r.RemoteAddr) {
				next.ServeHTTP(w, r)
				return
			}
			tok := r.Header.Get("X-CSRF-Token")
			if tok == "" {
				http.Error(w, "missing csrf token", http.StatusForbidden)
				return
			}
			if tok != a.csrfToken {
				http.Error(w, "invalid csrf token", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopback(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (a *API) wrap(h func(http.ResponseWriter, *http.Request) (int, any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code, payload := h(w, r)
		w.Header().Set("Content-Type", "application/json")
		// Normalize error payloads to unified shape when status >= 400
		if code >= 400 && payload != nil {
			switch v := payload.(type) {
			case map[string]string:
				if e, ok := v["error"]; ok {
					payload = errorResponse{Code: e, Message: v["message"]}
				}
			case map[string]any:
				if e, ok := v["error"]; ok {
					msg := ""
					if m, ok := v["message"].(string); ok {
						msg = m
					}
					payload = errorResponse{Code: fmt.Sprint(e), Message: msg}
				}
			}
		}
		w.WriteHeader(code)
		if payload != nil {
			_ = json.NewEncoder(w).Encode(payload)
		}
	}
}

func (a *API) wrapPOST(h func(http.ResponseWriter, *http.Request) (int, any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		a.wrap(h)(w, r)
	}
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) (int, any) {
	ss, tun, ctrl, _, _, _ := a.state.Snapshot()
	return 200, map[string]any{
		"service_state":   string(ss),
		"tun_state":       map[bool]string{true: "up", false: "down"}[tun],
		"tun_error":       a.state.TunError(),
		"controller":      map[bool]string{true: "connected", false: "disconnected"}[ctrl],
		"public_endpoint": a.state.PublicEndpoint(),
		"csrf_token":      a.csrfToken,
		"i18n":            a.cfg.Language, // frontend watches this key for language changes
		"bearer_set":      strings.TrimSpace(a.cfg.Api.BearerToken) != "",
	}
}

func (a *API) handleServiceStart(w http.ResponseWriter, r *http.Request) (int, any) {
	a.state.Start()
	a.log("service.start")
	return 200, map[string]string{"result": "ok"}
}

func (a *API) handleServiceStop(w http.ResponseWriter, r *http.Request) (int, any) {
	a.state.Stop()
	a.log("service.stop")
	return 200, map[string]string{"result": "ok"}
}

func (a *API) handleServiceRestart(w http.ResponseWriter, r *http.Request) (int, any) {
	a.state.Restart()
	a.log("service.restart")
	return 200, map[string]string{"result": "ok"}
}

func (a *API) handleNetworks(w http.ResponseWriter, r *http.Request) (int, any) {
	_, _, _, nets, _, _ := a.state.Snapshot()
	if nets == nil {
		nets = []core.Network{}
	}
	return 200, map[string]any{"networks": nets}
}

func (a *API) handleNetworksJoin(w http.ResponseWriter, r *http.Request) (int, any) {
	var in struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		JoinSecret  string `json:"join_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		return 400, map[string]string{"error": "bad_json"}
	}
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		return 400, map[string]string{"error": "missing_id"}
	}

	a.cfgMu.Lock()
	var target *config.Network
	for i := range a.cfg.Networks {
		if strings.EqualFold(a.cfg.Networks[i].ID, in.ID) {
			target = &a.cfg.Networks[i]
			break
		}
	}
	if target == nil {
		a.cfg.Networks = append(a.cfg.Networks, config.Network{ID: in.ID})
		target = &a.cfg.Networks[len(a.cfg.Networks)-1]
	}
	target.ID = in.ID
	if in.Name != "" {
		target.Name = in.Name
	} else if target.Name == "" {
		target.Name = in.ID
	}
	if in.Description != "" {
		target.Description = in.Description
	}
	// Enforce join secret validation: if a secret is already declared for this network,
	// require a non-empty match. If not declared, accept and optionally set it when provided.
	if strings.TrimSpace(target.JoinSecret) != "" {
		if strings.TrimSpace(in.JoinSecret) == "" {
			a.cfgMu.Unlock()
			return 400, map[string]string{"error": "missing_join_secret"}
		}
		if in.JoinSecret != target.JoinSecret {
			a.cfgMu.Unlock()
			return 403, map[string]string{"error": "invalid_join_secret"}
		}
	} else if strings.TrimSpace(in.JoinSecret) != "" {
		// Set the secret for future validations if provided during first join
		target.JoinSecret = strings.TrimSpace(in.JoinSecret)
	}
	if target.Address == "" {
		target.Address = a.ipam.AddressFor(target.ID)
	}
	target.Joined = true

	if err := config.Save(a.cfg); err != nil {
		a.cfgMu.Unlock()
		return 500, map[string]string{"error": "save_failed"}
	}
	netCopy := *target
	networks := append([]config.Network(nil), a.cfg.Networks...)
	a.cfgMu.Unlock()

	a.state.SetNetworks(mapNetworks(networks))
	a.log("net.join:" + netCopy.ID)
	return 200, map[string]any{"result": "ok", "network": netCopy}
}

func (a *API) handleNetworksLeave(w http.ResponseWriter, r *http.Request) (int, any) {
	var in struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		return 400, map[string]string{"error": "bad_json"}
	}
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		return 400, map[string]string{"error": "missing_id"}
	}

	a.cfgMu.Lock()
	var target *config.Network
	for i := range a.cfg.Networks {
		if strings.EqualFold(a.cfg.Networks[i].ID, in.ID) {
			target = &a.cfg.Networks[i]
			break
		}
	}
	if target == nil {
		a.cfgMu.Unlock()
		return 404, map[string]string{"error": "not_found"}
	}
	target.Joined = false
	if err := config.Save(a.cfg); err != nil {
		a.cfgMu.Unlock()
		return 500, map[string]string{"error": "save_failed"}
	}
	netCopy := *target
	networks := append([]config.Network(nil), a.cfg.Networks...)
	a.cfgMu.Unlock()

	a.state.SetNetworks(mapNetworks(networks))
	a.log("net.leave:" + netCopy.ID)
	return 200, map[string]any{"result": "ok", "network": netCopy}
}

func (a *API) handlePeers(w http.ResponseWriter, r *http.Request) (int, any) {
	if a.peersFn != nil {
		return 200, map[string]any{"peers": a.peersFn()}
	}
	_, _, _, _, _, s := a.state.Snapshot()
	out := []map[string]any{}
	for _, addr := range s.Peers {
		out = append(out, map[string]any{
			"ID":       addr,
			"Relay":    false,
			"P2P":      true,
			"RTTms":    0,
			"LastSeen": time.Now(),
		})
	}
	return 200, map[string]any{"peers": out}
}

func (a *API) handleSettings(w http.ResponseWriter, r *http.Request) (int, any) {
	switch r.Method {
	case http.MethodGet:
		_, _, _, _, _, s := a.state.Snapshot()
		// include obfuscated info whether bearer is set (do not leak token)
		return 200, map[string]any{
			"port":               s.Port,
			"mtu":                s.MTU,
			"log_level":          s.LogLevel,
			"language":           s.Language,
			"autostart":          s.Autostart,
			"controller_url":     s.ControllerURL,
			"relay_urls":         s.RelayURLs,
			"udp_port":           s.UDPPort,
			"peers":              s.Peers,
			"stun_servers":       s.StunServers,
			"trusted_peer_certs": s.TrustedPeerCerts,
			"bearer_set":         strings.TrimSpace(a.cfg.Api.BearerToken) != "",
		}
	case http.MethodPut:
		var in struct {
			Port             int      `json:"port"`
			MTU              int      `json:"mtu"`
			LogLevel         string   `json:"log_level"`
			Language         string   `json:"language"`
			Autostart        bool     `json:"autostart"`
			ControllerURL    string   `json:"controller_url"`
			RelayURLs        []string `json:"relay_urls"`
			UDPPort          int      `json:"udp_port"`
			Peers            []string `json:"peers"`
			StunServers      []string `json:"stun_servers"`
			TrustedPeerCerts []string `json:"trusted_peer_certs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			return 400, map[string]string{"error": "bad_json"}
		}

		a.cfgMu.Lock()
		a.cfg.Port = in.Port
		a.cfg.MTU = in.MTU
		a.cfg.LogLevel = in.LogLevel
		if in.Language != "" {
			a.cfg.Language = strings.ToLower(in.Language)
			gi18n.SetActiveLanguage(a.cfg.Language)
		}
		a.cfg.Autostart = in.Autostart
		a.cfg.ControllerURL = in.ControllerURL
		a.cfg.RelayURLs = in.RelayURLs
		if in.UDPPort != 0 {
			a.cfg.UDPPort = in.UDPPort
		}
		if in.Peers != nil {
			a.cfg.Peers = in.Peers
		}
		if in.StunServers != nil {
			a.cfg.StunServers = in.StunServers
		}
		if in.TrustedPeerCerts != nil {
			a.cfg.TrustedPeerCerts = in.TrustedPeerCerts
		}
		relayCopy := append([]string(nil), a.cfg.RelayURLs...)
		peersCopy := append([]string(nil), a.cfg.Peers...)
		stunCopy := append([]string(nil), a.cfg.StunServers...)
		trustedCopy := append([]string(nil), a.cfg.TrustedPeerCerts...)
		settings := core.Settings{
			Port:             a.cfg.Port,
			MTU:              a.cfg.MTU,
			LogLevel:         a.cfg.LogLevel,
			Language:         a.cfg.Language,
			Autostart:        a.cfg.Autostart,
			ControllerURL:    a.cfg.ControllerURL,
			RelayURLs:        relayCopy,
			UDPPort:          a.cfg.UDPPort,
			Peers:            peersCopy,
			StunServers:      stunCopy,
			TrustedPeerCerts: trustedCopy,
		}
		if err := config.Save(a.cfg); err != nil {
			a.cfgMu.Unlock()
			return 500, map[string]string{"error": "save_failed"}
		}
		a.cfgMu.Unlock()

		a.state.SetSettings(settings)
		a.log("settings.update")
		return 200, map[string]string{"result": "ok"}
	default:
		return 405, map[string]string{"error": "method_not_allowed"}
	}
}

// POST /api/token/regenerate - creates a new random bearer token and persists it
func (a *API) handleTokenRegenerate(w http.ResponseWriter, r *http.Request) (int, any) {
	if r.Method != http.MethodPost {
		return 405, map[string]string{"error": "method_not_allowed"}
	}
	a.cfgMu.Lock()
	a.cfg.Api.BearerToken = config.GenerateBearerToken()
	if err := config.Save(a.cfg); err != nil {
		a.cfgMu.Unlock()
		return 500, map[string]string{"error": "save_failed"}
	}
	a.cfgMu.Unlock()
	// Update state snapshot to keep UI consistent
	_, _, _, _, _, s := a.state.Snapshot()
	a.state.SetSettings(s)
	return 200, map[string]any{"result": "ok", "bearer_set": true}
}

// POST /api/token/clear - clears the bearer token (disables auth)
func (a *API) handleTokenClear(w http.ResponseWriter, r *http.Request) (int, any) {
	if r.Method != http.MethodPost {
		return 405, map[string]string{"error": "method_not_allowed"}
	}
	a.cfgMu.Lock()
	a.cfg.Api.BearerToken = ""
	if err := config.Save(a.cfg); err != nil {
		a.cfgMu.Unlock()
		return 500, map[string]string{"error": "save_failed"}
	}
	a.cfgMu.Unlock()
	_, _, _, _, _, s := a.state.Snapshot()
	a.state.SetSettings(s)
	return 200, map[string]any{"result": "ok", "bearer_set": false}
}

func (a *API) handleDiagRun(w http.ResponseWriter, r *http.Request) (int, any) {
	dr := diag.Run(a.cfg)
	svcState, _, _, networks, _, settings := a.state.Snapshot()
	// Back-compat top-level fields used by Web UI
	result := map[string]any{
		"tun_ok":          a.state.TunError() == "",
		"tun_error":       a.state.TunError(),
		"public_endpoint": dr.PublicEP,
		"networks":        networks,
		"mtu":             settings.MTU,
		"service_state":   svcState,
		"diag":            dr, // detailed object for future UI
	}
	return 200, result
}

func (a *API) handleUpdateCheck(w http.ResponseWriter, r *http.Request) (int, any) {
	res, err := updater.Check()
	if err != nil {
		return 500, map[string]any{"error": "update_check_failed", "details": err.Error()}
	}
	return 200, res
}

func (a *API) handleUpdateApply(w http.ResponseWriter, r *http.Request) (int, any) {
	if err := updater.Apply(); err != nil {
		return 500, map[string]any{"error": "update_apply_failed", "details": err.Error()}
	}
	return 200, map[string]string{"result": "ok"}
}

// --- Agent-side Controller Token Management ---
// GET  /api/controller/token/status -> { set: bool }
func (a *API) handleControllerTokenStatus(w http.ResponseWriter, r *http.Request) (int, any) {
	return 200, map[string]any{"set": strings.TrimSpace(a.controllerToken) != ""}
}

// POST /api/controller/token/set { token: string }
func (a *API) handleControllerTokenSet(w http.ResponseWriter, r *http.Request) (int, any) {
	var in struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Token) == "" {
		return 400, map[string]string{"error": "bad_json"}
	}
	_, _, secrets := config.Paths()
	if err := os.MkdirAll(secrets, 0o755); err != nil {
		return 500, map[string]string{"error": "secrets_dir"}
	}
	p := filepath.Join(secrets, "controller_token.txt")
	if err := os.WriteFile(p, []byte(strings.TrimSpace(in.Token)), 0o600); err != nil {
		return 500, map[string]string{"error": "write_failed"}
	}
	a.controllerToken = strings.TrimSpace(in.Token)
	return 200, map[string]string{"result": "ok"}
}

// POST /api/controller/token/clear
func (a *API) handleControllerTokenClear(w http.ResponseWriter, r *http.Request) (int, any) {
	_, _, secrets := config.Paths()
	p := filepath.Join(secrets, "controller_token.txt")
	_ = os.Remove(p)
	a.controllerToken = ""
	return 200, map[string]string{"result": "ok"}
}

func (a *API) handleExit(w http.ResponseWriter, r *http.Request) (int, any) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		if a.shutdown != nil {
			a.shutdown()
		}
		time.Sleep(400 * time.Millisecond)
		os.Exit(0)
	}()
	return 200, map[string]string{"result": "exiting"}
}

// handleMetrics returns a simple JSON metrics payload (not Prometheus format yet).
func (a *API) handleMetrics(w http.ResponseWriter, r *http.Request) (int, any) {
	uptime := time.Since(a.startTime).Seconds()
	a.netMu.RLock()
	netCount := len(a.networkSettings)
	prefCount := len(a.memberPreferences)
	a.netMu.RUnlock()
	a.sseMu.Lock()
	subs := len(a.sseSubs)
	a.sseMu.Unlock()
	svcState, tunUp, ctrlUp, networks, peers, settings := a.state.Snapshot()
	joined := 0
	for _, n := range networks {
		if n.Joined {
			joined++
		}
	}
	return 200, map[string]any{
		"uptime_seconds":             uptime,
		"service_state":              svcState,
		"tun_up":                     tunUp,
		"controller_up":              ctrlUp,
		"networks_joined":            joined,
		"networks_total":             len(networks),
		"configured_peers":           len(peers),
		"sse_subscribers":            subs,
		"network_settings_objects":   netCount,
		"member_preferences_objects": prefCount,
		"mtu":                        settings.MTU,
	}
}

func (a *API) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 16)
	a.sseMu.Lock()
	a.sseSubs[ch] = struct{}{}
	a.sseMu.Unlock()
	defer func() {
		a.sseMu.Lock()
		delete(a.sseSubs, ch)
		a.sseMu.Unlock()
		close(ch)
	}()

	fmt.Fprintf(w, "retry: 3000\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case line := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", line)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

func (a *API) fakeLogPump() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	i := 0
	for range ticker.C {
		i++
		a.log(fmt.Sprintf("event_%d", i))
	}
}

func (a *API) log(event string) {
	if a.logger != nil {
		a.logger.Println(event)
	}
	a.sseMu.Lock()
	for ch := range a.sseSubs {
		select {
		case ch <- event:
		default:
		}
	}
	a.sseMu.Unlock()
}

// handleControllerProxy forwards requests under /api/controller/ to the configured controller URL.
func (a *API) handleControllerProxy(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimSpace(a.cfg.ControllerURL)
	if base == "" {
		http.Error(w, "controller not configured", http.StatusBadRequest)
		return
	}
	// Build target URL
	suffix := strings.TrimPrefix(r.URL.Path, "/api/controller")
	if !strings.HasPrefix(suffix, "/") {
		suffix = "/" + suffix
	}
	target := strings.TrimRight(base, "/") + suffix
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	// Prepare new request
	var body io.Reader
	if r.Body != nil {
		// Read and buffer to allow reuse errors
		b, _ := io.ReadAll(r.Body)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(r.Method, target, body)
	if err != nil {
		http.Error(w, "proxy request failed", http.StatusBadGateway)
		return
	}
	// Copy minimal headers
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if a.controllerToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.controllerToken)
	}
	if owner := r.Header.Get("X-Owner-Token"); owner != "" {
		req.Header.Set("X-Owner-Token", owner)
	}
	// Do request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "controller unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	// Copy response
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// addChatMessage appends a message to a network chat history and broadcasts to subscribers.
func (a *API) addChatMessage(networkID, from, text string) ChatMessage {
	msg := ChatMessage{
		ID:        randomToken(),
		NetworkID: networkID,
		From:      from,
		Text:      strings.TrimSpace(text),
		At:        time.Now().UTC(),
	}
	a.netMu.Lock()
	a.chatHistories[networkID] = append(a.chatHistories[networkID], msg)
	// Cap history to last 200 messages per network to avoid unbounded growth.
	if len(a.chatHistories[networkID]) > 200 {
		a.chatHistories[networkID] = a.chatHistories[networkID][len(a.chatHistories[networkID])-200:]
	}
	subs := a.chatSubs[networkID]
	a.netMu.Unlock()
	for ch := range subs {
		select {
		case ch <- msg:
		default:
		}
	}
	return msg
}

// subscribeChat registers a channel to receive chat messages for a network.
func (a *API) subscribeChat(networkID string) chan ChatMessage {
	ch := make(chan ChatMessage, 32)
	a.netMu.Lock()
	if a.chatSubs[networkID] == nil {
		a.chatSubs[networkID] = map[chan ChatMessage]struct{}{}
	}
	a.chatSubs[networkID][ch] = struct{}{}
	a.netMu.Unlock()
	return ch
}

// unsubscribeChat removes a chat subscription.
func (a *API) unsubscribeChat(networkID string, ch chan ChatMessage) {
	a.netMu.Lock()
	if m := a.chatSubs[networkID]; m != nil {
		delete(m, ch)
	}
	a.netMu.Unlock()
	close(ch)
}

// handleChatMessages supports listing and sending chat messages with optional since filter and rate limiting.
func (a *API) handleChatMessages(w http.ResponseWriter, r *http.Request) {
	nid, ok := NetworkIDFromContext(r.Context())
	if !ok {
		http.Error(w, "network not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		sinceParam := r.URL.Query().Get("since")
		var since time.Time
		if sinceParam != "" {
			if t, err := time.Parse(time.RFC3339, sinceParam); err == nil {
				since = t
			}
		}
		a.netMu.RLock()
		msgs := a.chatHistories[nid]
		filtered := make([]ChatMessage, 0, len(msgs))
		if !since.IsZero() {
			for _, m := range msgs {
				if m.At.After(since) || m.At.Equal(since) {
					filtered = append(filtered, m)
				}
			}
		} else {
			filtered = append(filtered, msgs...)
		}
		out := append([]ChatMessage(nil), filtered...)
		a.netMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"messages": out})
	case http.MethodPost:
		// Basic enablement checks
		a.netMu.RLock()
		ns := a.networkSettings[nid]
		prefs := a.memberPreferences[nid+"/me"]
		a.netMu.RUnlock()
		if ns == nil || prefs == nil || !ns.AllowChat || !prefs.ChatEnabled {
			http.Error(w, "chat not enabled", http.StatusForbidden)
			return
		}
		var in struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Text) == "" {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		// Rate limit: one message per 2 seconds per (network,user)
		key := nid + "|" + prefs.Nickname
		now := time.Now()
		a.netMu.Lock()
		last := a.chatLast[key]
		if !last.IsZero() && now.Sub(last) < 2*time.Second {
			a.netMu.Unlock()
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		a.chatLast[key] = now
		a.netMu.Unlock()
		msg := a.addChatMessage(nid, prefs.Nickname, in.Text)
		_ = a.saveAll() // best-effort persistence
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(msg)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleChatStream streams new chat messages via SSE.
func (a *API) handleChatStream(w http.ResponseWriter, r *http.Request) {
	nid, ok := NetworkIDFromContext(r.Context())
	if !ok {
		http.Error(w, "network not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := a.subscribeChat(nid)
	// Send last messages initially and a heartbeat to transition EventSource to open
	a.netMu.RLock()
	history := a.chatHistories[nid]
	a.netMu.RUnlock()
	if flusher, ok := w.(http.Flusher); ok {
		for _, m := range history {
			b, _ := json.Marshal(m)
			fmt.Fprintf(w, "data: %s\n\n", string(b))
		}
		// initial heartbeat comment
		fmt.Fprintf(w, ": ping\n\n")
		flusher.Flush()
	}
	ctx := r.Context()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.unsubscribeChat(nid, ch)
			return
		case msg := <-ch:
			if flusher, ok := w.(http.Flusher); ok {
				b, _ := json.Marshal(msg)
				fmt.Fprintf(w, "data: %s\n\n", string(b))
				flusher.Flush()
			}
		case <-ticker.C:
			if flusher, ok := w.(http.Flusher); ok {
				fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			}
		}
	}
}

func ResolveWebDir() string {
	if v := os.Getenv("GOCONNECT_WEB_DIR"); v != "" {
		if st, err := os.Stat(v); err == nil && st.IsDir() {
			return v
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "webui")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidate := filepath.Join(exeDir, "webui")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		candidate2 := filepath.Join(exeDir, "..", "webui")
		if st, err := os.Stat(candidate2); err == nil && st.IsDir() {
			return candidate2
		}
	}
	return ""
}

func mapNetworks(in []config.Network) []core.Network {
	out := make([]core.Network, 0, len(in))
	for _, n := range in {
		out = append(out, core.Network{
			ID:          n.ID,
			Name:        n.Name,
			Description: n.Description,
			Joined:      n.Joined,
			Address:     n.Address,
		})
	}
	return out

}

// Benzersiz token üretici (ID ve CSRF için)
func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
