package main

import (
	"context"
	"fmt"
	
	"ccsync-net/clipboard"
	"ccsync-net/config"
	"ccsync-net/sync"

	wailsRun "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx        context.Context
	cfg        *config.Config
	server     *sync.Server
	client     *sync.Client
	clipboard  *clipboard.Monitor
	lastCopied string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		server:    sync.NewServer(),
		client:    sync.NewClient(),
		clipboard: clipboard.NewMonitor(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadConfig()
	a.loadConfig()
	a.initCallbacks() // Init callbacks first so logging works during clipboard start
	a.initClipboard()

	if a.cfg.AutoStart {
		if a.cfg.Mode == "server" {
			a.StartServer(a.cfg.ServerPort)
		} else {
			a.ConnectToServer(a.cfg.ServerAddress)
		}
	}
}

func (a *App) initClipboard() {
	// clipboard.Init() already called in main.go
	// if err := a.clipboard.Init(); err != nil {
	// 	wailsRun.LogError(a.ctx, "剪贴板初始化失败: "+err.Error())
	// }
	a.clipboard.Start()
}

func (a *App) initCallbacks() {
	// 剪贴板变化 -> 发送给网络
	a.clipboard.OnChange = func(content string) {
		// 防止回环：如果内容与最后一次处理的内容相同，则忽略
		if content == a.lastCopied {
			return
		}

		a.lastCopied = content
		wailsRun.EventsEmit(a.ctx, "clipboard:local", content)

		// 如果模式为 receive_only，则不发送
		if a.cfg.SyncMode == "receive_only" {
			wailsRun.EventsEmit(a.ctx, "log", "同步模式为只入，跳过发送")
			return
		}

		if a.cfg.Mode == "server" && a.server.IsRunning() {
			a.server.BroadcastClipboard(content, "server")
		} else if a.cfg.Mode == "client" && a.client.IsConnected() {
			a.client.SendClipboard(content, "client")
		}
	}

	// 服务端收到消息 -> 更新本地剪贴板
	a.server.OnClipboardReceived = func(content string) {
		// 如果模式为 send_only，则不写入本地剪贴板
		if a.cfg.SyncMode == "send_only" {
			// 仍然可以通知界面收到了消息，但不写入
			wailsRun.EventsEmit(a.ctx, "log", "同步模式为只出，跳过写入本地剪贴板")
			return
		}

		if content != a.lastCopied {
			a.clipboard.SetContent(content)
			a.lastCopied = content
			wailsRun.EventsEmit(a.ctx, "clipboard:remote", content)
		}
	}

	a.server.OnClientConnected = func(count int) {
		wailsRun.EventsEmit(a.ctx, "server:client_count", count)
	}

	a.server.OnClientDisconnected = func(count int) {
		wailsRun.EventsEmit(a.ctx, "server:client_count", count)
	}

	// 客户端收到消息 -> 更新本地剪贴板
	a.client.OnClipboardReceived = func(content string) {
		// 如果模式为 send_only，则不写入本地剪贴板
		if a.cfg.SyncMode == "send_only" {
			wailsRun.EventsEmit(a.ctx, "log", "同步模式为只出，跳过写入本地剪贴板")
			return
		}

		if content != a.lastCopied {
			a.clipboard.SetContent(content)
			a.lastCopied = content
			wailsRun.EventsEmit(a.ctx, "clipboard:remote", content)
		}
	}

	a.client.OnConnected = func() {
		wailsRun.EventsEmit(a.ctx, "client:status", true)
	}

	a.client.OnDisconnected = func() {
		wailsRun.EventsEmit(a.ctx, "client:status", false)
	}

	// 日志
	a.server.OnLog = func(msg string) {
		wailsRun.LogInfo(a.ctx, msg)
		wailsRun.EventsEmit(a.ctx, "log", msg)
	}
	a.client.OnLog = func(msg string) {
		wailsRun.LogInfo(a.ctx, msg)
		wailsRun.EventsEmit(a.ctx, "log", msg)
	}
	a.clipboard.OnLog = func(msg string) {
		wailsRun.LogInfo(a.ctx, msg)
		wailsRun.EventsEmit(a.ctx, "log", msg)
	}
}

// loadConfig 加载配置
func (a *App) loadConfig() {
	cfg, err := config.Load()
	if err != nil {
		wailsRun.LogError(a.ctx, "加载配置失败: "+err.Error())
		a.cfg = config.DefaultConfig()
	} else {
		a.cfg = cfg
	}
}

// SaveConfig 保存配置
func (a *App) SaveConfig(cfg config.Config) error {
	a.cfg.Mode = cfg.Mode
	a.cfg.ServerPort = cfg.ServerPort
	a.cfg.ServerAddress = cfg.ServerAddress
	a.cfg.AutoStart = cfg.AutoStart
	a.cfg.SyncMode = cfg.SyncMode // 保存 SyncMode
	return a.cfg.Save()
}

// GetConfig 获取当前配置
func (a *App) GetConfig() *config.Config {
	return a.cfg
}

// StartServer 启动服务端
func (a *App) StartServer(port int) error {
	wailsRun.EventsEmit(a.ctx, "status", "正在启动服务端...")
	err := a.server.Start(port)
	if err == nil {
		wailsRun.EventsEmit(a.ctx, "status", fmt.Sprintf("服务端运行中 (端口: %d)", port))
		wailsRun.EventsEmit(a.ctx, "server:running", true)
	}
	return err
}

// StopServer 停止服务端
func (a *App) StopServer() {
	a.server.Stop()
	wailsRun.EventsEmit(a.ctx, "status", "服务端已停止")
	wailsRun.EventsEmit(a.ctx, "server:running", false)
}

// ConnectToServer 连接服务端
func (a *App) ConnectToServer(addr string) error {
	wailsRun.EventsEmit(a.ctx, "status", "正在连接到 "+addr+"...")
	return a.client.Connect(addr)
}

// Disconnect 断开连接
func (a *App) Disconnect() {
	a.client.Disconnect()
	wailsRun.EventsEmit(a.ctx, "status", "已断开连接")
}

// shutdown 清理资源
func (a *App) shutdown(ctx context.Context) {
	a.clipboard.Stop()
	a.server.Stop()
	a.client.Disconnect()
}
