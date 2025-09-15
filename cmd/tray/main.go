package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/getlantern/systray"

	"goconnect/internal/config"
	gi18n "goconnect/internal/i18n"
)

func main() {
	exeDir, _ := os.Getwd()
	_ = gi18n.LoadFromFiles(filepath.Join(exeDir, "internal", "i18n"))
	cfg, _ := config.Load()
	gi18n.SetActiveLanguage(cfg.Language)

	systray.Run(onReady, onExit)
}

func onReady() {
	status := systray.AddMenuItem(fmt.Sprintf("%s — %s", gi18n.T("app.title"), gi18n.T("status.running")), gi18n.T("status.running"))
	start := systray.AddMenuItem(gi18n.T("menu.start"), "start service")
	stop := systray.AddMenuItem(gi18n.T("menu.stop"), "stop service")
	_ = systray.AddMenuItem(gi18n.T("menu.networks"), "networks")
	_ = systray.AddMenuItem(gi18n.T("menu.diagnose"), "diagnose")
	openLogs := systray.AddMenuItem(gi18n.T("menu.openLogs"), "logs")
	openPanel := systray.AddMenuItem(gi18n.T("menu.openPanel"), "panel")
	langEN := systray.AddMenuItem(gi18n.T("menu.language.english"), "lang")
	langTR := systray.AddMenuItem(gi18n.T("menu.language.turkish"), "lang")
	exit := systray.AddMenuItem(gi18n.T("menu.exit"), "exit")
	shutdown := systray.AddMenuItem(gi18n.T("menu.shutdown"), "exit agent")

	go func() {
		for {
			select {
			case <-status.ClickedCh:
				_ = apiPost("/api/status")
			case <-start.ClickedCh:
				_ = apiPost("/api/service/start")
			case <-stop.ClickedCh:
				_ = apiPost("/api/service/stop")
			case <-openPanel.ClickedCh:
				openURL("http://localhost:2537/")
			case <-openLogs.ClickedCh:
				_, logDir, _ := config.Paths()
				openPath(logDir)
			case <-langEN.ClickedCh:
				_ = apiPutJSON("/api/settings", `{"language":"en"}`)
			case <-langTR.ClickedCh:
				_ = apiPutJSON("/api/settings", `{"language":"tr"}`)
			case <-exit.ClickedCh:
				systray.Quit()
				return
			case <-shutdown.ClickedCh:
				_ = apiPost("/api/exit")
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {}

func openURL(u string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}

func openPath(p string) { _ = exec.Command("explorer", p).Start() }

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
	csrf := extractCSRFFromSetCookie(resp.Header.Values("Set-Cookie"))
	_ = resp.Body.Close()
	req, _ := http.NewRequest(method, "http://127.0.0.1:2537"+path, body)
	req.Header.Set("Content-Type", ctype)
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
		req.Header.Set("Cookie", fmt.Sprintf("goc_csrf=%s", csrf))
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

var csrfRe = regexp.MustCompile(`goc_csrf=([^;\s]+)`)

func extractCSRFFromSetCookie(cookies []string) string {
	for _, c := range cookies {
		m := csrfRe.FindStringSubmatch(c)
		if len(m) == 2 {
			return m[1]
		}
	}
	return ""
}
