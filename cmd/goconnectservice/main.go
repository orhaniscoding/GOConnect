package main

import (
	"context"
	"fmt"
	"goconnect/internal/api"
	"goconnect/internal/config"
	"goconnect/internal/core"
	gi18n "goconnect/internal/i18n"
	golog "goconnect/internal/logging"
	gtx "goconnect/internal/transport"
	gtun "goconnect/internal/tun"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kardianos/service"
)

var logger service.Logger

type program struct {
	cancel context.CancelFunc
	srv    *http.Server
	tx     *gtx.Manager
}

func (p *program) Start(s service.Service) error {
	if service.Interactive() {
		_ = logger.Info("Running in terminal.")
	} else {
		_ = logger.Info("Running under service manager.")
	}
	go p.run()
	return nil
}

func (p *program) run() {
	_ = logger.Info("run() called")
	// When running as a service, the working directory is System32, so we must
	// determine paths based on the executable's location.
	exePath, err := os.Executable()
	if err != nil {
		_ = logger.Errorf("CRITICAL: Failed to get executable path: %v", err)
		return
	}
	_ = logger.Info("Executable path obtained: " + exePath)
	rootDir := filepath.Dir(filepath.Dir(exePath))
	_ = logger.Info("Root directory determined: " + rootDir)

	// Setup file-based logging. This is our primary log for detailed diagnostics.
	logDir := filepath.Join(config.ProgramDataBase(), "logs")
	_ = logger.Info("Log directory path: " + logDir)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		_ = logger.Errorf("CRITICAL: Failed to create log directory: %v", err)
		return
	}
	_ = logger.Info("Log directory ensured.")
	logPath := filepath.Join(logDir, "agent.log")
	_ = logger.Info("Log file path: " + logPath)
	fileLogger, closer, err := golog.SetupLogger(logPath)
	if err != nil {
		_ = logger.Errorf("CRITICAL: Failed to setup file logger: %v", err)
		return
	}
	_ = logger.Info("File logger created.")
	defer func() {
		if closer != nil {
			_ = closer.Close()
		}
	}()
	// Redirect standard `log` package to our file.
	log.SetOutput(fileLogger.Writer())

	log.Println("--- GOConnect Service Starting (File Log) ---")
	log.Printf("Service executable path: %s", exePath)
	log.Printf("Determined root directory: %s", rootDir)
	log.Printf("Logging to file: %s", logPath)

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	if err := config.EnsureDirs(); err != nil {
		_ = logger.Errorf("Failed to ensure config directories: %v", err)
		log.Printf("Failed to ensure config directories: %v", err)
		return
	}
	_, _, secretsDir := config.Paths()
	log.Println("Configuration directories ensured.")

	cfg, err := config.Load()
	if err != nil {
		_ = logger.Errorf("Config load failed: %v", err)
		log.Printf("Config load failed: %v", err)
		return
	}
	log.Println("Config loaded successfully.")

	// Load resources from paths relative to the project root.
	if err := gi18n.LoadFromFiles(filepath.Join(rootDir, "internal", "i18n")); err != nil {
		log.Printf("i18n load from files failed: %v", err)
	}
	gi18n.SetActiveLanguage(cfg.Language)
	log.Println("i18n resources loaded.")

	// Legacy separate tray manager removed; Wails tray runs independently now.

	st := core.NewState(core.Settings{
		Port:             cfg.Port,
		MTU:              cfg.MTU,
		LogLevel:         cfg.LogLevel,
		Language:         cfg.Language,
		Autostart:        cfg.Autostart,
		ControllerURL:    cfg.ControllerURL,
		RelayURLs:        cfg.RelayURLs,
		UDPPort:          cfg.UDPPort,
		Peers:            cfg.Peers,
		StunServers:      cfg.StunServers,
		TrustedPeerCerts: cfg.TrustedPeerCerts,
	})
	st.SetTunDevice(gtun.New())
	st.Start()
	log.Println("Core state initialized and started.")

	a := api.New(st, cfg, fileLogger, func() {
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
	log.Println("API initialized.")

	addr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", cfg.Port))
	webDir := api.ResolveWebDir()
	p.srv = a.Serve(addr, webDir)

	go func() {
		log.Printf("HTTP API starting at http://%s (webDir=%s)", addr, webDir)
		if err := p.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			_ = logger.Errorf("HTTP server error: %v", err)
			log.Printf("HTTP server error: %v", err)
		}
	}()

	manager, err := gtx.NewManager(fmt.Sprintf(":%d", cfg.UDPPort), cfg.StunServers, secretsDir, cfg.TrustedPeerCerts)
	if err != nil {
		_ = logger.Errorf("Transport init failed: %v", err)
		log.Printf("Transport init failed: %v", err)
		return
	}
	p.tx = manager
	p.tx.OnNATInfo(func(ep string) {
		st.SetPublicEndpoint(ep)
		if ep != "" {
			log.Printf("Detected public endpoint: %s", ep)
		}
	})
	if err := p.tx.Start(cfg.Peers); err != nil {
		log.Printf("Transport start error: %v", err)
	}
	log.Println("Transport manager started.")

	_ = logger.Info("Service is running. Blocking until stop signal.")
	log.Println("Service is running. Blocking until stop signal.")

	// Block until the context is cancelled.
	<-ctx.Done()

	// --- Shutdown sequence ---
	_ = logger.Info("--- GOConnect Service Stopping ---")
	log.Println("--- GOConnect Service Stopping ---")
	if p.srv != nil {
		_ = p.srv.Close()
		log.Println("HTTP server stopped.")
	}
	if p.tx != nil {
		_ = p.tx.Stop()
		log.Println("Transport stopped.")
	}
	log.Println("All components stopped. Exiting run().")
}

func (p *program) Stop(s service.Service) error {
	_ = logger.Info("Stop signal received. Initiating shutdown.")
	log.Println("Program stopping.")
	// This is the crucial part: call the cancel function to unblock the `run` function.
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

func main() {
	svcCfg := &service.Config{
		Name:        "GOConnect",
		DisplayName: "GOConnect Service",
		Description: "GOConnect P2P Connectivity Service.",
	}

	prg := &program{}
	s, err := service.New(prg, svcCfg)
	if err != nil {
		log.Fatal(err)
	}
	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) > 1 {
		err = service.Control(s, os.Args[1])
		if err != nil {
			log.Printf("Failed to %s %s: %v", os.Args[1], svcCfg.Name, err)
			return
		}
		log.Printf("%s %s.", svcCfg.Name, os.Args[1])
		return
	}

	err = s.Run()
	if err != nil {
		_ = logger.Error(err.Error())
	}
}
