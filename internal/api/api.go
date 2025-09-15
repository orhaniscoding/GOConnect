package api

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "net"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"

    "goconnect/internal/config"
    "goconnect/internal/core"
    gi18n "goconnect/internal/i18n"
    webfs "goconnect/webui"
)

type API struct {
    state     *core.State
    cfg       *config.Config
    logger    *log.Logger
    csrfToken string
    shutdown  func()

    sseMu   sync.Mutex
    sseSubs map[chan string]struct{}
    peersFn func() []map[string]any
}

func New(state *core.State, cfg *config.Config, logger *log.Logger, shutdown func()) *API {
    return &API{
        state:  state,
        cfg:    cfg,
        logger: logger,
        sseSubs: map[chan string]struct{}{},
        csrfToken: randomToken(),
        shutdown: shutdown,
    }
}

func randomToken() string {
    b := make([]byte, 16)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}

func (a *API) Serve(addr string, webDir string) *http.Server {
    mux := http.NewServeMux()

    // Static Web UI
    var handler http.Handler
    if webDir != "" { handler = http.FileServer(http.Dir(webDir)) } else { handler = http.FileServer(http.FS(webfs.FS)) }
    mux.Handle("/", http.StripPrefix("/", handler))

    // API endpoints
    mux.HandleFunc("/api/status", a.wrap(a.handleStatus))
    mux.HandleFunc("/api/service/start", a.wrapPOST(a.handleServiceStart))
    mux.HandleFunc("/api/service/stop", a.wrapPOST(a.handleServiceStop))
    mux.HandleFunc("/api/service/restart", a.wrapPOST(a.handleServiceRestart))
    mux.HandleFunc("/api/networks", a.wrap(a.handleNetworks))
    mux.HandleFunc("/api/networks/join", a.wrapPOST(a.handleNetworksJoin))
    mux.HandleFunc("/api/networks/leave", a.wrapPOST(a.handleNetworksLeave))
    mux.HandleFunc("/api/peers", a.wrap(a.handlePeers))
    mux.HandleFunc("/api/logs/stream", a.handleLogsStream)
    mux.HandleFunc("/api/settings", a.wrap(a.handleSettings))
    mux.HandleFunc("/api/diag/run", a.wrapPOST(a.handleDiagRun))
    mux.HandleFunc("/api/update/check", a.wrapPOST(a.handleUpdateCheck))
    mux.HandleFunc("/api/update/apply", a.wrapPOST(a.handleUpdateApply))
    mux.HandleFunc("/api/exit", a.wrapPOST(a.handleExit))

    srv := &http.Server{Addr: addr, Handler: a.csrfMiddleware(mux)}

    // Start fake log generator for SSE
    go a.fakeLogPump()
    return srv
}

func (a *API) SetPeersFn(f func() []map[string]any) { a.peersFn = f }

// Middleware: CSRF cookie set and validation for non-GET
func (a *API) csrfMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Always set cookie SameSite=Strict for local-only UI
        http.SetCookie(w, &http.Cookie{
            Name:     "goc_csrf",
            Value:    a.csrfToken,
            HttpOnly: false,
            Secure:   false,
            Path:     "/",
            SameSite: http.SameSiteStrictMode,
        })
        if r.Method != http.MethodGet {
            // Allow local-only shutdown without CSRF to avoid getting stuck
            if r.URL.Path == "/api/exit" && isLoopback(r.RemoteAddr) {
                next.ServeHTTP(w, r)
                return
            }
            tok := r.Header.Get("X-CSRF-Token")
            if tok == "" {
                http.Error(w, "missing csrf", http.StatusForbidden)
                return
            }
            if tok != a.csrfToken {
                http.Error(w, "invalid csrf", http.StatusForbidden)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}

func isLoopback(remote string) bool {
    host, _, err := net.SplitHostPort(remote)
    if err != nil { host = remote }
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

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) (int, any) {
    ss, tun, ctrl, _, _, _ := a.state.Snapshot()
    return 200, map[string]any{
        "service_state": string(ss),
        "tun_state":     map[bool]string{true: "up", false: "down"}[tun],
        "controller":    map[bool]string{true: "connected", false: "disconnected"}[ctrl],
        "i18n":          gi18n.ActiveLanguage(),
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
    if len(nets) == 0 {
        nets = []core.Network{
            {ID: "demo1", Name: "Demo Network", Description: "Example network", Joined: true},
        }
    }
    return 200, map[string]any{"networks": nets}
}

func (a *API) handleNetworksJoin(w http.ResponseWriter, r *http.Request) (int, any) {
    a.log("net.join")
    return 200, map[string]string{"result": "ok"}
}

func (a *API) handleNetworksLeave(w http.ResponseWriter, r *http.Request) (int, any) {
    a.log("net.leave")
    return 200, map[string]string{"result": "ok"}
}

func (a *API) handlePeers(w http.ResponseWriter, r *http.Request) (int, any) {
    if a.peersFn != nil {
        return 200, map[string]any{"peers": a.peersFn()}
    }
    // Fallback to settings-derived peers
    _, _, _, _, _, s := a.state.Snapshot()
    out := []map[string]any{}
    for _, addr := range s.Peers {
        out = append(out, map[string]any{ "ID": addr, "Relay": false, "P2P": true, "RTTms": 0, "LastSeen": time.Now() })
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
            Port          int      `json:"port"`
            MTU           int      `json:"mtu"`
            LogLevel      string   `json:"log_level"`
            Language      string   `json:"language"`
            Autostart     bool     `json:"autostart"`
            ControllerURL string   `json:"controller_url"`
            RelayURLs     []string `json:"relay_urls"`
            UDPPort       int      `json:"udp_port"`
            Peers         []string `json:"peers"`
        }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
            return 400, map[string]string{"error": "bad_json"}
        }
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
        if in.UDPPort != 0 { a.cfg.UDPPort = in.UDPPort }
        if in.Peers != nil { a.cfg.Peers = in.Peers }
        _ = config.Save(a.cfg)
        a.state.SetSettings(core.Settings{
            Port: a.cfg.Port, MTU: a.cfg.MTU, LogLevel: a.cfg.LogLevel, Language: a.cfg.Language,
            Autostart: a.cfg.Autostart, ControllerURL: a.cfg.ControllerURL, RelayURLs: a.cfg.RelayURLs,
            UDPPort: a.cfg.UDPPort, Peers: a.cfg.Peers,
        })
        a.log("settings.update")
        return 200, map[string]string{"result": "ok"}
    default:
        return 405, map[string]string{"error": "method_not_allowed"}
    }
}

func (a *API) handleDiagRun(w http.ResponseWriter, r *http.Request) (int, any) {
    // Return stub results
    return 200, map[string]any{"stun": "ok", "mtu": "ok"}
}

func (a *API) handleUpdateCheck(w http.ResponseWriter, r *http.Request) (int, any) {
    return 200, map[string]any{"available": false, "version": "v1.0.0"}
}

func (a *API) handleUpdateApply(w http.ResponseWriter, r *http.Request) (int, any) {
    return 200, map[string]string{"result": "ok"}
}

func (a *API) handleExit(w http.ResponseWriter, r *http.Request) (int, any) {
    // Trigger graceful shutdown shortly after responding
    go func(){
        time.Sleep(100 * time.Millisecond)
        if a.shutdown != nil { a.shutdown() }
        // Fallback hard-exit to ensure process terminates even if service loop is blocking
        time.Sleep(400 * time.Millisecond)
        os.Exit(0)
    }()
    return 200, map[string]string{"result": "exiting"}
}

// SSE
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

    // initial
    fmt.Fprintf(w, "retry: 3000\n")
    flusher, _ := w.(http.Flusher)
    if flusher != nil { flusher.Flush() }

    for {
        select {
        case <-r.Context().Done():
            return
        case line := <-ch:
            fmt.Fprintf(w, "data: %s\n\n", line)
            if flusher != nil { flusher.Flush() }
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
    if a.logger != nil { a.logger.Println(event) }
    a.sseMu.Lock()
    for ch := range a.sseSubs {
        select { case ch <- event: default: }
    }
    a.sseMu.Unlock()
}

// Helper to resolve web assets path in dev vs installed locations.
func ResolveWebDir() string {
    // 1) Env override
    if v := os.Getenv("GOCONNECT_WEB_DIR"); v != "" {
        if st, err := os.Stat(v); err == nil && st.IsDir() { return v }
    }
    // 2) CWD/webui (dev)
    if cwd, err := os.Getwd(); err == nil {
        candidate := filepath.Join(cwd, "webui")
        if st, err := os.Stat(candidate); err == nil && st.IsDir() { return candidate }
    }
    // 3) exeDir/webui (installed alongside)
    if exe, err := os.Executable(); err == nil {
        exeDir := filepath.Dir(exe)
        candidate := filepath.Join(exeDir, "webui")
        if st, err := os.Stat(candidate); err == nil && st.IsDir() { return candidate }
        // 4) exeDir/../webui (binary in bin/, assets in project root)
        candidate2 := filepath.Join(exeDir, "..", "webui")
        if st, err := os.Stat(candidate2); err == nil && st.IsDir() { return candidate2 }
    }
    // 5) fallback relative
    return "webui"
}
