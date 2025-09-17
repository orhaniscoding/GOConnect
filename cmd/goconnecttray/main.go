package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/getlantern/systray"

	"goconnect/internal"
	"goconnect/internal/config"
)

var (
	statusItem     *systray.MenuItem
	startItem      *systray.MenuItem
	stopItem       *systray.MenuItem
	restartItem    *systray.MenuItem
	networksItem   *systray.MenuItem
	diagnoseItem   *systray.MenuItem
	logsItem       *systray.MenuItem
	panelItem      *systray.MenuItem
	languageMenu   *systray.MenuItem
	langENItem     *systray.MenuItem
	langTRItem     *systray.MenuItem
	exitItem       *systray.MenuItem
	shutdownItem   *systray.MenuItem
	appTitleItem   *systray.MenuItem
	statusInfoItem *systray.MenuItem

	currentLanguage = "en"
)

func main() {
	internal.InitI18n(currentLanguage)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load: %v", err)
	}
	if cfg.Language != "" {
		currentLanguage = cfg.Language
	}

	systray.Run(onReady, onExit)
}

func onReady() {
	appTitleItem = systray.AddMenuItem("GOConnect by orhaniscoding", "app title")
	appTitleItem.Disable()
	statusInfoItem = systray.AddMenuItem("", "status info")
	statusInfoItem.Disable()

	systray.AddSeparator()

	// Kategorize edilmiş menüler
	// Durum ve servis
	statusItem = systray.AddMenuItem("", "status")
	startItem = systray.AddMenuItem("", "start service")
	stopItem = systray.AddMenuItem("", "stop service")
	restartItem = systray.AddMenuItem("", "restart service")

	systray.AddSeparator()

	// Ağ ve tanılama
	networksItem = systray.AddMenuItem("", "open networks")
	diagnoseItem = systray.AddMenuItem("", "diagnose")
	logsItem = systray.AddMenuItem("", "logs")
	panelItem = systray.AddMenuItem("", "panel")

	systray.AddSeparator()

	// Dil menüsü
	languageMenu = systray.AddMenuItem("", "Languages")
	langENItem = languageMenu.AddSubMenuItem("", "English")
	langTRItem = languageMenu.AddSubMenuItem("", "Türkçe")

	systray.AddSeparator()

	exitItem = systray.AddMenuItem("", "exit")
	shutdownItem = systray.AddMenuItem("", "exit agent")

	updateMenuTitles(currentLanguage)
	updateStatusInfo()

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
			case <-restartItem.ClickedCh:
				_ = apiPost("/api/service/restart")
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

func updateMenuTitles(lang string) {
	localizer := internal.NewLocalizer(lang)
	statusItem.SetTitle(localizer("app.title") + " - " + localizer("status.running"))
	startItem.SetTitle(localizer("menu.start"))
	stopItem.SetTitle(localizer("menu.stop"))
	restartItem.SetTitle(localizer("menu.restart"))
	networksItem.SetTitle(localizer("menu.networks"))
	diagnoseItem.SetTitle(localizer("menu.diagnose"))
	logsItem.SetTitle(localizer("menu.openLogs"))
	panelItem.SetTitle(localizer("menu.openPanel"))
	languageMenu.SetTitle(localizer("menu.languages"))
	langENItem.SetTitle(localizer("menu.language.english"))
	langTRItem.SetTitle(localizer("menu.language.turkish"))
	exitItem.SetTitle(localizer("menu.exit"))
	shutdownItem.SetTitle(localizer("menu.shutdown"))
	appTitleItem.SetTitle("GOConnect by orhaniscoding")
}

func updateStatusInfo() {
	// Servis ve bağlantı durumunu API'den çek
	go func() {
		serviceStatus := "?"
		controllerStatus := "?"
		serviceColor := "⚪"
		controllerColor := "⚪"
		resp, err := http.Get("http://127.0.0.1:2537/api/status")
		if err == nil {
			defer resp.Body.Close()
			var payload struct {
				ServiceState string `json:"service_state"`
				Controller   string `json:"controller"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
				if payload.ServiceState == "running" {
					serviceStatus = "Running"
					serviceColor = "🟢"
				} else {
					serviceStatus = "Stopped"
					serviceColor = "🔴"
				}
				if payload.Controller == "connected" {
					controllerStatus = "Connected"
					controllerColor = "🟢"
				} else {
					controllerStatus = "Disconnected"
					controllerColor = "🔴"
				}
			}
		}
		statusInfoItem.SetTitle(serviceColor + " Service: " + serviceStatus + "  " + controllerColor + " Controller: " + controllerStatus)
		// Durumu periyodik güncelle
		time.Sleep(5 * time.Second)
		updateStatusInfo()
	}()
}

func changeLanguage(lang string) {
	currentLanguage = lang
	updateMenuTitles(lang)
	// İsteğe bağlı: config dosyasına kaydet
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
	// Get status to fetch the latest CSRF token from the JSON body
	resp, err := client.Get("http://127.0.0.1:2537/api/status")
	if err != nil {
		return fmt.Errorf("failed to get status for csrf token: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Token string `json:"csrf_token"`
	}
	//	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil { // TODO: Yeni i18n sistemine geçerken json decode kodunu tekrar ekle
	// return fmt.Errorf("failed to decode csrf token from status: %w", err) // TODO: Yeni i18n sistemine geçerken tekrar eklenecek
	//}
	//if payload.Token == "" {
	// return fmt.Errorf("did not receive csrf token from status endpoint") // TODO: Yeni i18n sistemine geçerken tekrar eklenecek
	//}

	req, _ := http.NewRequest(method, "http://127.0.0.1:2537"+path, body)
	req.Header.Set("Content-Type", ctype)
	req.Header.Set("X-CSRF-Token", payload.Token)

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
