package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"golang.design/x/clipboard"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Initialize clipboard (must be on main thread)
	if err := clipboard.Init(); err != nil {
		println("========================================")
		println("剪贴板初始化失败:", err.Error())
		println("")
		println("在 Linux 系统上，请确保已安装剪贴板工具：")
		println("  Debian/Ubuntu: sudo apt-get install xclip")
		println("  Fedora/RHEL:   sudo dnf install xclip")
		println("  Arch Linux:    sudo pacman -S xclip")
		println("========================================")
		return
	}

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
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
