package main

import (
	"context"
	"goconnect/internal/config"
	"goconnect/internal/controller"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kardianos/service"
)

var logger service.Logger

type program struct {
	cancel context.CancelFunc
	srv    *http.Server
}

func (p *program) Start(s service.Service) error {
	if service.Interactive() {
		_ = logger.Info("Controller starting in terminal mode")
	} else {
		_ = logger.Info("Controller starting as Windows service")
	}
	go p.run()
	return nil
}

func (p *program) run() {
	// Ensure ProgramData dirs (for symmetry with agent logs)
	if err := config.EnsureDirs(); err != nil {
		_ = logger.Error("ensure dirs failed:", err)
	}
	logDir := filepath.Join(config.ProgramDataBase(), "logs")
	_ = os.MkdirAll(logDir, 0o755)
	// Simple file log (append)
	logPath := filepath.Join(logDir, "controller.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		log.SetOutput(f)
		_ = logger.Info("Logging to ", logPath)
	} else {
		_ = logger.Error("file log open failed:", err)
	}

	port := os.Getenv("CONTROLLER_PORT")
	if port == "" {
		port = "2538"
	}

	store := controller.NewStore("controller_state.json")
	h := controller.NewHandler(store)
	p.srv = &http.Server{Addr: ":" + port, Handler: h}

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	_ = logger.Info("Controller listening on port ", port)
	log.Printf("GOConnect Controller starting on :%s", port)

	go func() {
		if err := p.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			_ = logger.Error("ListenAndServe error:", err)
			log.Printf("ListenAndServe error: %v", err)
			cancel()
		}
	}()

	<-ctx.Done()
	_ = logger.Info("Controller shutdown sequence starting")
	if p.srv != nil {
		_ = p.srv.Close()
	}
	time.Sleep(300 * time.Millisecond)
	_ = logger.Info("Controller stopped")
}

func (p *program) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

func main() {
	svcCfg := &service.Config{
		Name:        "GOConnectController",
		DisplayName: "GOConnect Controller",
		Description: "GOConnect merkezi controller servisi.",
	}
	prg := &program{}
	s, err := service.New(prg, svcCfg)
	if err != nil {
		log.Fatalf("controller service.New failed: %v", err)
	}

	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatalf("controller logger init failed: %v", err)
	}

	// Support control commands (install, uninstall, start, stop, restart) like agent
	if len(os.Args) > 1 {
		cmd := os.Args[1]
		if cmd == "run" { // foreground dev run
			_ = logger.Info("Foreground run mode")
			prg.run()
			return
		}
		if err := service.Control(s, cmd); err != nil {
			log.Printf("Failed to %s %s: %v", cmd, svcCfg.Name, err)
		} else {
			log.Printf("%s %s.", svcCfg.Name, cmd)
		}
		return
	}

	if err := s.Run(); err != nil {
		_ = logger.Error(err)
	}
}
