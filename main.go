package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsRun "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "ccsync-net",
		Width:  512,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "ccsync-net-ui-instance",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				wailsRun.WindowShow(app.ctx)
				wailsRun.WindowSetAlwaysOnTop(app.ctx, true)
				wailsRun.WindowSetAlwaysOnTop(app.ctx, false)
			},
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
			DisableWebViewDrop: true,
		},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
