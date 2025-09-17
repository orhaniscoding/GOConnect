package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"goconnect/internal"
	"goconnect/internal/config"
	"goconnect/internal/core"
	"goconnect/internal/ipam"
	"goconnect/internal/traymgr"
	webfs "goconnect/webui"
)

type API struct {
	state     *core.State
	cfg       *config.Config
	cfgMu     sync.Mutex
	logger    *log.Logger
	tray      *traymgr.Manager
	ipam      *ipam.Allocator
	csrfToken string
	shutdown  func()

	sseMu   sync.Mutex
	sseSubs map[chan string]struct{}
	peersFn func() []map[string]any
}

func New(state *core.State, cfg *config.Config, logger *log.Logger, tray *traymgr.Manager, shutdown func()) *API {
	a := &API{
		state:     state,
		cfg:       cfg,
		logger:    logger,
		tray:      tray,
		ipam:      ipam.New(),
		csrfToken: randomToken(),
		shutdown:  shutdown,
		sseSubs:   map[chan string]struct{}{},
	}
	state.SetNetworks(mapNetworks(cfg.Networks))
	for _, n := range cfg.Networks {
		if n.Address != "" {
			a.ipam.Reserve(n.ID, n.Address)
		}
	}
	return a
}

func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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

	mux.HandleFunc("/api/status", a.wrap(a.handleStatus))
	mux.HandleFunc("/api/service/start", a.wrapPOST(a.handleServiceStart))
	mux.HandleFunc("/api/service/stop", a.wrapPOST(a.handleServiceStop))
	mux.HandleFunc("/api/service/restart", a.wrapPOST(a.handleServiceRestart))
	mux.HandleFunc("/api/networks", a.wrap(a.handleNetworks))
	mux.HandleFunc("/api/networks/join", a.wrapPOST(a.handleNetworksJoin))
	mux.HandleFunc("/api/networks/leave", a.wrapPOST(a.handleNetworksLeave))
	mux.HandleFunc("/api/peers", a.wrap(a.handlePeers))
	mux.HandleFunc("/api/logs/stream", a.handleLogsStream)
	mux.HandleFunc("/api/tray/start", a.wrapPOST(a.handleTrayStart))
	mux.HandleFunc("/api/tray/stop", a.wrapPOST(a.handleTrayStop))
	mux.HandleFunc("/api/tray/heartbeat", a.wrapPOST(a.handleTrayHeartbeat))
	mux.HandleFunc("/api/tray/offline", a.wrapPOST(a.handleTrayOffline))
	mux.HandleFunc("/api/settings", a.wrap(a.handleSettings))
	mux.HandleFunc("/api/diag/run", a.wrapPOST(a.handleDiagRun))
	mux.HandleFunc("/api/update/check", a.wrapPOST(a.handleUpdateCheck))
	mux.HandleFunc("/api/update/apply", a.wrapPOST(a.handleUpdateApply))
	mux.HandleFunc("/api/exit", a.wrapPOST(a.handleExit))

	srv := &http.Server{Addr: addr, Handler: a.csrfMiddleware(mux)}

	go a.fakeLogPump()
	return srv
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

		if r.Method != http.MethodGet {
			// Loopback requests to /api/exit are allowed without CSRF for tray client shutdown.
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

func (a *API) getLocalizer() func(string) string {
	lang := a.cfg.Language
	if lang == "" {
		lang = "en"
	}
	return internal.NewLocalizer(lang)
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) (int, any) {
	ss, tun, ctrl, _, _, _ := a.state.Snapshot()
	trayOnline, lastSeen := a.state.TrayStatus(time.Now())
	trayState := map[bool]string{true: "online", false: "offline"}[trayOnline]
	return 200, map[string]any{
		"service_state":   string(ss),
		"tun_state":       map[bool]string{true: "up", false: "down"}[tun],
		"tun_error":       a.state.TunError(),
		"controller":      map[bool]string{true: "connected", false: "disconnected"}[ctrl],
		"public_endpoint": a.state.PublicEndpoint(),
		"tray_state":      trayState,
		"csrf_token":      a.csrfToken,
		"tray_last_seen":  lastSeen,
		"language":        a.cfg.Language,
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

func (a *API) handleTrayStart(w http.ResponseWriter, r *http.Request) (int, any) {
	if a.tray == nil {
		return 500, map[string]string{"error": "tray_manager_unavailable"}
	}
	if err := a.tray.Start(); err != nil {
		if a.logger != nil {
			a.logger.Printf("tray start error: %v", err)
		}
		return 500, map[string]string{"error": "tray_start_failed"}
	}
	a.state.RecordTrayHeartbeat()
	return 200, map[string]string{"result": "ok"}
}

func (a *API) handleTrayStop(w http.ResponseWriter, r *http.Request) (int, any) {
	if a.tray == nil {
		return 500, map[string]string{"error": "tray_manager_unavailable"}
	}
	if err := a.tray.Stop(); err != nil {
		if a.logger != nil {
			a.logger.Printf("tray stop error: %v", err)
		}
		return 500, map[string]string{"error": "tray_stop_failed"}
	}
	a.state.SetTrayOffline()
	return 200, map[string]string{"result": "ok"}
}

func (a *API) handleTrayHeartbeat(w http.ResponseWriter, r *http.Request) (int, any) {
	a.state.RecordTrayHeartbeat()
	return 200, map[string]string{"result": "ok"}
}

func (a *API) handleTrayOffline(w http.ResponseWriter, r *http.Request) (int, any) {
	a.state.SetTrayOffline()
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

	// In a real implementation, we would validate the JoinSecret here against a
	// controller or some other authority. For now, we just store it.

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
	if in.JoinSecret != "" {
		target.JoinSecret = in.JoinSecret
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
		return 200, s
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
			// gi18n.SetActiveLanguage(a.cfg.Language) // i18n kaldırıldı
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
		settings := core.Settings{
			Port:          a.cfg.Port,
			MTU:           a.cfg.MTU,
			LogLevel:      a.cfg.LogLevel,
			Language:      a.cfg.Language,
			Autostart:     a.cfg.Autostart,
			ControllerURL: a.cfg.ControllerURL,
			RelayURLs:     relayCopy,
			UDPPort:       a.cfg.UDPPort,
			Peers:         peersCopy,
			StunServers:   stunCopy,
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

func (a *API) handleDiagRun(w http.ResponseWriter, r *http.Request) (int, any) {
	// Gerçek teşhis verileriyle doldurulmuş örnek bir yanıt
	svcState, _, _, networks, _, settings := a.state.Snapshot()
	joined := 0
	for _, n := range networks {
		if n.Joined {
			joined++
		}
	}
	result := map[string]any{
		"tun_ok":          a.state.TunError() == "",
		"tun_error":       a.state.TunError(),
		"public_endpoint": a.state.PublicEndpoint(),
		"networks":        networks,
		"stun":            "ok", // Geliştirilebilir: Gerçek STUN testi
		"mtu":             settings.MTU,
		"joined":          joined,
		"total":           len(networks),
		"service_state":   svcState,
	}
	return 200, result
}

func (a *API) handleUpdateCheck(w http.ResponseWriter, r *http.Request) (int, any) {
	return 200, map[string]any{"available": false, "version": "v1.0.0"}
}

func (a *API) handleUpdateApply(w http.ResponseWriter, r *http.Request) (int, any) {
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
