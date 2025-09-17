package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	trayMenu := []*options.TrayMenuItem{
		options.TrayMenuItem{Label: "Open Dashboard", Click: func(_ *options.TrayMenuItem) { app.OpenDashboard() }},
		options.TrayMenuItem{Label: "Open Logs", Click: func(_ *options.TrayMenuItem) { app.OpenLogs() }},
		options.TrayMenuItem{Label: "Copy Public Endpoint", Click: func(_ *options.TrayMenuItem) { app.CopyPublicEndpoint() }},
		options.TrayMenuItem{Label: "Toggle Language (EN/TR)", Click: func(_ *options.TrayMenuItem) { _ = app.ToggleLanguage() }},
		options.TrayMenuItem{Label: "About", Click: func(_ *options.TrayMenuItem) {
			wails.ShowMessage("GOConnect Tray\nby orhaniscoding\nhttps://github.com/orhaniscoding/GOConnect")
		}},
		options.TrayMenuItem{Label: "Shutdown", Click: func(_ *options.TrayMenuItem) { app.Shutdown() }},
	}

	err := wails.Run(&options.App{
		Title:  "tray-wails",
		Width:  400,
		Height: 300,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		SystemTray: &options.SystemTray{
			Menu:    trayMenu,
			Tooltip: "GOConnect by orhaniscoding",
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
