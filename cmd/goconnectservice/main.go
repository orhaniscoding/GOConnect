package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/kardianos/service"

	"goconnect/internal/api"
	"goconnect/internal/config"
	"goconnect/internal/core"
	gi18n "goconnect/internal/i18n"
	golog "goconnect/internal/logging"
	gtx "goconnect/internal/transport"
	"goconnect/internal/traymgr"
	gtun "goconnect/internal/tun"
)

type program struct {
	srv    *http.Server
	tx     *gtx.Manager
	tray   *traymgr.Manager
	cancel context.CancelFunc
}

func (p *program) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *program) run() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load: %v", err)
	}
	exeDir, _ := os.Getwd()
	_ = gi18n.LoadFromFiles(filepath.Join(exeDir, "internal", "i18n"))
	gi18n.SetActiveLanguage(cfg.Language)

	_, logDir, _ := config.Paths()
	logPath := filepath.Join(logDir, "agent.log")
	logger, closer, err := golog.SetupLogger(logPath)
	if err != nil {
		log.Printf("logging setup failed: %v", err)
	}
	defer func() {
		if closer != nil {
			_ = closer.Close()
		}
	}()

	trayBinary := filepath.Join(exeDir, "bin", "GOConnectTray.exe")
	p.tray = traymgr.New(exeDir, trayBinary, logger)

	st := core.NewState(core.Settings{
		Port:          cfg.Port,
		MTU:           cfg.MTU,
		LogLevel:      cfg.LogLevel,
		Language:      cfg.Language,
		Autostart:     cfg.Autostart,
		ControllerURL: cfg.ControllerURL,
		RelayURLs:     cfg.RelayURLs,
		UDPPort:       cfg.UDPPort,
		Peers:         cfg.Peers,
		StunServers:   cfg.StunServers,
	})
	st.SetTunDevice(gtun.New())
	st.Start()

	a := api.New(st, cfg, logger, p.tray, func() {
		if p.cancel != nil {
			p.cancel()
		}
	})
	a.SetPeersFn(func() []map[string]any {
		if p.tx == nil {
			return nil
		}
		snaps := p.tx.SnapshotPeers()
		out := make([]map[string]any, 0, len(snaps))
		for _, s := range snaps {
			out = append(out, map[string]any{
				"ID":       s.Address,
				"Relay":    s.Relay,
				"P2P":      s.P2P,
				"RTTms":    s.RTTms,
				"LastSeen": s.LastSeen,
			})
		}
		return out
	})

	addr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", cfg.Port))
	webDir := api.ResolveWebDir()
	p.srv = a.Serve(addr, webDir)

	go func() {
		if err := p.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("http server: %v", err)
		}
	}()
	logger.Printf("HTTP API listening at http://%s (webDir=%s)", addr, webDir)

	if p.tray != nil {
		if err := p.tray.Start(); err != nil {
			logger.Printf("tray start error: %v", err)
		} else {
			st.RecordTrayHeartbeat()
		}
	}

	p.tx = gtx.NewManager(fmt.Sprintf(":%d", cfg.UDPPort), cfg.StunServers)
	p.tx.SetNATUpdateFn(func(ep string) {
		st.SetPublicEndpoint(ep)
		if ep != "" {
			logger.Printf("detected public endpoint: %s", ep)
		}
	})
	if err := p.tx.Start(cfg.Peers); err != nil {
		logger.Printf("transport start error: %v", err)
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigc:
	case <-ctx.Done():
	}
	signal.Stop(sigc)

	if p.srv != nil {
		_ = p.srv.Close()
	}
	if p.tx != nil {
		_ = p.tx.Stop()
	}
	if p.tray != nil {
		_ = p.tray.Stop()
	}
}

func (p *program) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.srv != nil {
		_ = p.srv.Close()
	}
	if p.tx != nil {
		_ = p.tx.Stop()
	}
	if p.tray != nil {
		_ = p.tray.Stop()
	}
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
	if err := s.Run(); err != nil {
		log.Fatalf("service run: %v", err)
	}
}
