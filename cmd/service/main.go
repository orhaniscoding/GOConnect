package main

import (
    "fmt"
    "log"
    "net"
    "net/http"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
    "context"

    "github.com/kardianos/service"

    "goconnect/internal/api"
    "goconnect/internal/config"
    "goconnect/internal/core"
    gtun "goconnect/internal/tun"
    gtx "goconnect/internal/transport"
    gi18n "goconnect/internal/i18n"
    golog "goconnect/internal/logging"
)

type program struct{
    srv    *http.Server
    tx     *gtx.Manager
    cancel context.CancelFunc
}

func (p *program) Start(s service.Service) error {
    go p.run()
    return nil
}

func (p *program) run() {
    ctx, cancel := context.WithCancel(context.Background())
    p.cancel = cancel
    // Load config and i18n
    cfg, _ := config.Load()
    // Load internal i18n JSONs
    exeDir, _ := os.Getwd()
    _ = gi18n.LoadFromFiles(filepath.Join(exeDir, "internal", "i18n"))
    gi18n.SetActiveLanguage(cfg.Language)

    // Setup logging
    _, logDir, _ := config.Paths()
    logPath := filepath.Join(logDir, "agent.log")
    logger, closer, err := golog.SetupLogger(logPath)
    if err != nil {
        log.Printf("logging setup failed: %v", err)
    }
    defer func(){ if closer != nil { _ = closer.Close() } }()

    st := core.NewState(core.Settings{
        Port: cfg.Port, MTU: cfg.MTU, LogLevel: cfg.LogLevel, Language: cfg.Language,
        Autostart: cfg.Autostart, ControllerURL: cfg.ControllerURL, RelayURLs: cfg.RelayURLs,
        UDPPort: cfg.UDPPort, Peers: cfg.Peers,
    })
    // Attach TUN device (stub by default; real when built with -tags=wintun on Windows)
    st.SetTunDevice(gtun.New())

    a := api.New(st, cfg, logger, func(){ if p.cancel != nil { p.cancel() } })
    // provide peers snapshot to API without tight coupling
    a.SetPeersFn(func() []map[string]any {
        if p.tx == nil { return nil }
        snaps := p.tx.SnapshotPeers()
        out := make([]map[string]any, 0, len(snaps))
        for _, s := range snaps {
            out = append(out, map[string]any{ "ID": s.Address, "Relay": s.Relay, "P2P": s.P2P, "RTTms": s.RTTms, "LastSeen": s.LastSeen })
        }
        return out
    })
    addr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", cfg.Port))
    webDir := api.ResolveWebDir()
    p.srv = a.Serve(addr, webDir)

    go func(){ _ = p.srv.ListenAndServe() }()
    logger.Printf("HTTP API listening at http://%s (webDir=%s)", addr, webDir)

    // Start QUIC transport
    p.tx = gtx.NewManager(fmt.Sprintf(":%d", cfg.UDPPort))
    if err := p.tx.Start(cfg.Peers); err != nil { logger.Printf("transport start error: %v", err) }

    // Wait for signal or programmatic cancel to exit
    sigc := make(chan os.Signal, 1)
    signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
    select {
    case <-sigc:
    case <-ctx.Done():
    }
    if p.srv != nil { _ = p.srv.Close() }
}

func (p *program) Stop(s service.Service) error {
    if p.cancel != nil { p.cancel() }
    if p.srv != nil { _ = p.srv.Close() }
    if p.tx != nil { _ = p.tx.Stop() }
    return nil
}

func main() {
    svcCfg := &service.Config{
        Name:        "GOConnectService",
        DisplayName: "GOConnect Service",
        Description: "GOConnect agent service with local API and web UI",
        Option: map[string]interface{}{
            "StartType": "automatic-delayed",
        },
    }
    prg := &program{}
    s, err := service.New(prg, svcCfg)
    if err != nil {
        log.Fatalf("service create: %v", err)
    }
    // Run in foreground for dev. The stub service just calls Start.
    if err := s.Run(); err != nil {
        log.Fatalf("service run: %v", err)
    }
}
