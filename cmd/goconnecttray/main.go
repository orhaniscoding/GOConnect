package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlantern/systray"

	"goconnect/internal/config"
	gi18n "goconnect/internal/i18n"
)

var (
	statusItem   *systray.MenuItem
	startItem    *systray.MenuItem
	stopItem     *systray.MenuItem
	networksItem *systray.MenuItem
	diagnoseItem *systray.MenuItem
	logsItem     *systray.MenuItem
	panelItem    *systray.MenuItem
	langENItem   *systray.MenuItem
	langTRItem   *systray.MenuItem
	exitItem     *systray.MenuItem
	shutdownItem *systray.MenuItem

	currentLanguage = "en"
)

func main() {
	exeDir, _ := os.Getwd()
	_ = gi18n.LoadFromFiles(filepath.Join(exeDir, "internal", "i18n"))
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load: %v", err)
	}
	if cfg.Language != "" {
		currentLanguage = cfg.Language
	}
	gi18n.SetActiveLanguage(currentLanguage)

	systray.Run(onReady, onExit)
}

func onReady() {
	statusItem = systray.AddMenuItem("", "status")
	startItem = systray.AddMenuItem("", "start service")
	stopItem = systray.AddMenuItem("", "stop service")
	networksItem = systray.AddMenuItem("", "open networks")
	diagnoseItem = systray.AddMenuItem("", "diagnose")
	logsItem = systray.AddMenuItem("", "logs")
	panelItem = systray.AddMenuItem("", "panel")
	langENItem = systray.AddMenuItem("", "lang en")
	langTRItem = systray.AddMenuItem("", "lang tr")
	exitItem = systray.AddMenuItem("", "exit")
	shutdownItem = systray.AddMenuItem("", "exit agent")

	applyLanguage(currentLanguage)

	_ = apiPost("/api/tray/heartbeat")
	go heartbeatLoop()

	go func() {
		for {
			select {
			case <-statusItem.ClickedCh:
				openURL("http://localhost:2537/")
			case <-startItem.ClickedCh:
				_ = apiPost("/api/service/start")
			case <-stopItem.ClickedCh:
				_ = apiPost("/api/service/stop")
			case <-networksItem.ClickedCh:
				openURL("http://localhost:2537/#networks")
			case <-diagnoseItem.ClickedCh:
				openURL("http://localhost:2537/#logs")
			case <-logsItem.ClickedCh:
				_, logDir, _ := config.Paths()
				openPath(logDir)
			case <-panelItem.ClickedCh:
				openURL("http://localhost:2537/")
			case <-langENItem.ClickedCh:
				changeLanguage("en")
			case <-langTRItem.ClickedCh:
				changeLanguage("tr")
			case <-exitItem.ClickedCh:
				systray.Quit()
				return
			case <-shutdownItem.ClickedCh:
				_ = apiPost("/api/exit")
				systray.Quit()
				return
			}
		}
	}()
}

func heartbeatLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		_ = apiPost("/api/tray/heartbeat")
	}
}

func changeLanguage(lang string) {
	if lang == "" || lang == currentLanguage {
		applyLanguage(currentLanguage)
		return
	}
	if err := apiPutJSON("/api/settings", fmt.Sprintf("{\"language\":\"%s\"}", lang)); err != nil {
		log.Printf("tray: change language: %v", err)
		return
	}
	applyLanguage(lang)
}

func applyLanguage(lang string) {
	currentLanguage = lang
	gi18n.SetActiveLanguage(lang)

	if statusItem != nil {
		statusItem.SetTitle(fmt.Sprintf("%s - %s", gi18n.T("app.title"), gi18n.T("status.running")))
	}
	if startItem != nil {
		startItem.SetTitle(gi18n.T("menu.start"))
	}
	if stopItem != nil {
		stopItem.SetTitle(gi18n.T("menu.stop"))
	}
	if networksItem != nil {
		networksItem.SetTitle(gi18n.T("menu.networks"))
	}
	if diagnoseItem != nil {
		diagnoseItem.SetTitle(gi18n.T("menu.diagnose"))
	}
	if logsItem != nil {
		logsItem.SetTitle(gi18n.T("menu.openLogs"))
	}
	if panelItem != nil {
		panelItem.SetTitle(gi18n.T("menu.openPanel"))
	}
	if langENItem != nil {
		langENItem.SetTitle(gi18n.T("menu.language.english"))
	}
	if langTRItem != nil {
		langTRItem.SetTitle(gi18n.T("menu.language.turkish"))
	}
	if exitItem != nil {
		exitItem.SetTitle(gi18n.T("menu.exit"))
	}
	if shutdownItem != nil {
		shutdownItem.SetTitle(gi18n.T("menu.shutdown"))
	}
}

func onExit() {
	_ = apiPost("/api/tray/offline")
}

func openURL(u string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}

func openPath(p string) {
	_ = exec.Command("explorer", p).Start()
}

func apiPost(path string) error {
	return apiWithCSRF("POST", path, "application/json", strings.NewReader("{}"))
}

func apiPutJSON(path, body string) error {
	return apiWithCSRF("PUT", path, "application/json", strings.NewReader(body))
}

func apiWithCSRF(method, path, ctype string, body *strings.Reader) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:2537/api/status")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var payload struct {
		Token string `json:"csrf_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	req, _ := http.NewRequest(method, "http://127.0.0.1:2537"+path, body)
	req.Header.Set("Content-Type", ctype)
	if payload.Token != "" {
		req.Header.Set("X-CSRF-Token", payload.Token)
	}
	resp2, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode >= 300 {
		return fmt.Errorf("status: %s", resp2.Status)
	}
	return nil
}
