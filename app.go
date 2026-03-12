package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	goSync "sync"
	"time"

	"ccsync-net/internal/shared"

	wailsRun "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	httpClient   *http.Client
	baseURL      string
	sseConnected bool
	sseLock      goSync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		baseURL: "http://localhost:12345",
	}
}

// IsSSEConnected Returns true if the backend has an active SSE connection to the service
func (a *App) IsSSEConnected() bool {
	a.sseLock.Lock()
	defer a.sseLock.Unlock()
	return a.sseConnected
}

// setSSEConnected Helper to update state safely
func (a *App) setSSEConnected(connected bool) {
	a.sseLock.Lock()
	defer a.sseLock.Unlock()
	if a.sseConnected != connected {
		a.sseConnected = connected
		// Also emit event here if missed?
		// No, monitor loop emits event. Just update state.
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.checkAndStartService()
	go a.monitorGlobalEvents()
}

func (a *App) monitorGlobalEvents() {
	// Create a client with NO timeout for SSE
	sseClient := &http.Client{
		Timeout: 0,
	}

	// Retry loop
	for {
		// Connect to SSE
		resp, err := sseClient.Get(a.baseURL + "/events")
		if err != nil {
			// Service might be down or starting
			a.setSSEConnected(false)
			wailsRun.EventsEmit(a.ctx, "backend:sse:status", false)
			time.Sleep(2 * time.Second)
			continue
		}

		// Connected
		a.setSSEConnected(true)
		wailsRun.EventsEmit(a.ctx, "backend:sse:status", true)
		wailsRun.LogInfo(a.ctx, "SSE Connected")

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				a.setSSEConnected(false)
				wailsRun.EventsEmit(a.ctx, "backend:sse:status", false)
				break // Disconnected, retry
			}
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "shutdown" {
					wailsRun.LogInfo(a.ctx, "Received shutdown signal from service")
					wailsRun.Quit(a.ctx)
					return // Exit loop
				} else if strings.HasPrefix(data, "{") {
					// Parse as JSON event wrapper
					var evt map[string]interface{}
					if err := json.Unmarshal([]byte(data), &evt); err == nil {
						evtType, _ := evt["type"].(string)
						if evtType == "file_copy" {
							// Emit to Vue
							wailsRun.EventsEmit(a.ctx, "backend:file_copy", evt["meta"])
						}
					}
				}
			}
		}
		resp.Body.Close()
		time.Sleep(1 * time.Second)
	}
}

func (a *App) checkAndStartService() {
	// 1. Check if service is responsive
	resp, err := a.httpClient.Get(a.baseURL + "/status")
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			wailsRun.LogInfo(a.ctx, "Background service is already running.")
			return
		}
	}

	// 2. Service not running, attempt to start it
	wailsRun.LogInfo(a.ctx, "Background service not found. Attempting to start...")

	ex, err := os.Executable()
	if err != nil {
		wailsRun.LogError(a.ctx, "Failed to get executable path: "+err.Error())
		return
	}
	exDir := filepath.Dir(ex)

	// Determine service executable name based on OS
	serviceName := "ccsync-service"
	if wailsRun.Environment(a.ctx).Platform == "windows" {
		serviceName += ".exe"
	}

	servicePath := filepath.Join(exDir, serviceName)
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		// Try development path (go run ...) or adjacent in build
		// For dev, it might be tricky. Let's just assume same dir for production.
		// Fallback for dev: try finding it in build/bin/ or ../
		servicePath = filepath.Join(exDir, "..", serviceName) // e.g. wails dev builds in build/bin
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			wailsRun.LogError(a.ctx, "Service executable not found at: "+filepath.Join(exDir, serviceName))
			return
		}
	}

	cmd := exec.Command(servicePath)
	if err := cmd.Start(); err != nil {
		wailsRun.LogError(a.ctx, "Failed to start service: "+err.Error())
		return
	}

	wailsRun.LogInfo(a.ctx, "Service started successfully. PID: "+fmt.Sprint(cmd.Process.Pid))

	// Wait for service to be ready (Poll for status)
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := a.httpClient.Get(a.baseURL + "/status")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				wailsRun.LogInfo(a.ctx, "Service is ready and responding.")
				return
			}
		}
	}
	wailsRun.LogError(a.ctx, "Service started but failed to respond within timeout.")
}

// shutdown 清理资源
func (a *App) shutdown(ctx context.Context) {
	// UI shutdown cleanup if needed
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

// GetConfig 获取当前配置
func (a *App) GetConfig() *shared.ConfigRequest {
	resp, err := a.httpClient.Get(a.baseURL + "/config")
	if err != nil {
		wailsRun.LogError(a.ctx, "无法获取配置: "+err.Error())
		return nil
	}
	defer resp.Body.Close()

	var cfg shared.ConfigRequest
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		wailsRun.LogError(a.ctx, "配置解析失败: "+err.Error())
		return nil
	}
	return &cfg
}

// SaveConfig 保存配置
func (a *App) SaveConfig(cfg shared.ConfigRequest) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Post(a.baseURL+"/config", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("保存配置失败: %s", string(body))
	}

	// Refresh status immediately
	a.GetStatus()

	return nil
}

// StartServerMode 切换到服务端模式并启动
func (a *App) StartServerMode(port int) error {
	cfg := a.GetConfig()
	if cfg == nil {
		return fmt.Errorf("无法获取当前配置")
	}

	cfg.Mode = "server"
	cfg.ServerPort = port
	// 保持其他配置不变

	if err := a.SaveConfig(*cfg); err != nil {
		return err
	}

	return a.StartSync()
}

// StartClientMode 切换到客户端模式并启动
func (a *App) StartClientMode(address string) error {
	cfg := a.GetConfig()
	if cfg == nil {
		return fmt.Errorf("无法获取当前配置")
	}

	cfg.Mode = "client"
	cfg.ServerAddress = address

	if err := a.SaveConfig(*cfg); err != nil {
		return err
	}

	return a.StartSync()
}

// StartSync 启动同步
func (a *App) StartSync() error {
	resp, err := a.httpClient.Post(a.baseURL+"/start", "application/json", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// StopSync 停止同步
func (a *App) StopSync() error {
	resp, err := a.httpClient.Post(a.baseURL+"/stop", "application/json", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// GetStatus 获取服务状态
func (a *App) GetStatus() *shared.StatusResponse {
	resp, err := a.httpClient.Get(a.baseURL + "/status")
	if err != nil {
		// return default offline status
		return &shared.StatusResponse{
			Running: false,
			Message: "服务未连接",
		}
	}
	defer resp.Body.Close()

	var status shared.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return &shared.StatusResponse{
			Running: false,
			Message: "状态解析失败",
		}
	}
	return &status
}

// SaveFileDialog 弹出系统保存文件对话框
func (a *App) SaveFileDialog(defaultName string) (string, error) {
	return wailsRun.SaveFileDialog(a.ctx, wailsRun.SaveDialogOptions{
		DefaultFilename: defaultName,
		Title:           "另存为...",
	})
}

// AcceptFile 接受文件并指定保存路径
func (a *App) AcceptFile(fileID string, savePath string) error {
	req := shared.AcceptFileRequest{
		FileID:   fileID,
		SavePath: savePath,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Post(a.baseURL+"/accept_file", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to accept file")
	}
	return nil
}

// SendFiles 请求发送本地文件列表
func (a *App) SendFiles(paths []string) error {
	req := shared.SendFilesRequest{
		Paths: paths,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Post(a.baseURL+"/send_files", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send files")
	}
	return nil
}
