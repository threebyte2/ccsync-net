package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	goSync "sync"
	"syscall"
	"time"

	"ccsync-net/internal/clipboard"
	"ccsync-net/internal/config"
	"ccsync-net/internal/shared"
	"ccsync-net/internal/sync"

	"os/exec"
	"path/filepath"
	"runtime"

	_ "embed"

	"github.com/energye/systray"
)

var (
	//go:embed icon.ico
	iconIco []byte
	//go:embed icon.png
	iconPng []byte
)

var (
	cfg         *config.Config
	server      *sync.Server
	client      *sync.Client
	clipMonitor *clipboard.Monitor
	lastCopied  string
)

// SSE
var (
	sseClients   = make(map[chan string]bool)
	sseClientsMu goSync.Mutex
)

func main() {
	// Initialize components
	server = sync.NewServer()
	client = sync.NewClient()
	clipMonitor = clipboard.NewMonitor()

	// Load config
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Println("Error loading config:", err)
		cfg = config.DefaultConfig()
	}

	// Start HTTP Server for IPC
	http.HandleFunc("/config", handleConfig)
	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/shutdown", handleShutdown)
	http.HandleFunc("/start", handleStart)
	http.HandleFunc("/stop", handleStop)
	http.HandleFunc("/events", handleEvents)
	http.HandleFunc("/accept_file", handleAcceptFile)
	http.HandleFunc("/send_files", handleSendFiles)

	go func() {
		fmt.Println("Starting IPC server on :12345")
		if err := http.ListenAndServe(":12345", nil); err != nil {
			fmt.Println("IPC Server Error:", err)
		}
	}()

	// Handle termination signal in background
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		systray.Quit()
	}()

	// Run Systray on main thread (Blocking)
	// Must be the last thing in main()
	systray.Run(onReady, onExit)
}

func onReady() {
	// Debug print
	fmt.Printf("Icon sizes - ICO: %d, PNG: %d\n", len(iconIco), len(iconPng))

	// systray on Windows definitely needs .ico (or at least works best with it)
	// On Linux it uses AppIndicator which prefers png, but let's try to be specific.
	if runtime.GOOS == "windows" {
		if len(iconIco) == 0 {
			fmt.Println("Error: icon.ico is empty")
		} else {
			systray.SetIcon(iconIco)
		}
	} else {
		// Linux
		if len(iconPng) == 0 {
			// Fallback to ico if png is missing?
			if len(iconIco) > 0 {
				systray.SetIcon(iconIco)
			} else {
				fmt.Println("Error: icon.png is empty")
			}
		} else {
			systray.SetIcon(iconPng)
		}
	}
	systray.SetTitle("CCSync Service")
	systray.SetTooltip("CCSync Service")

	// Add menu items
	mOpenUI := systray.AddMenuItem("打开界面", "Open UI Application")
	mOpenUI.Click(func() {
		openUI()
	})

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("退出", "Quit Application")
	mQuit.Click(func() {
		onExit()
		// We don't call os.Exit(0) here because systray.Run will return after onExit finishes (if it calls systray.Quit)
		// But onExit calls systray.Quit(), so Run returns.
		// However, systray docs say onExit is called after the Run loop exits.
		// Wait, if we click Quit menu, we want to exit.
		// If we call systray.Quit(), the Run loop terminates, then onExit is called?
		// No, systray.Quit() stops the loop. onExit is the callback passed to Run.
		// So we should just call systray.Quit() here?
		// But we want to run our cleanup logic.
		// If we call onExit() manually, we might duplicate logic if systray also calls it.
		// Let's just call onExit() and then os.Exit(0).
		// OR: just call systray.Quit(). systray will call onExit.
		systray.Quit()
	})

	// Initialize Monitor
	initMonitor()

	// Auto Start Sync
	if cfg.AutoStart {
		startSync()
	}
}

func openUI() {
	ex, err := os.Executable()
	if err != nil {
		fmt.Println("Failed to get executable path:", err)
		return
	}
	exDir := filepath.Dir(ex)

	uiName := "ccsync-net"
	if runtime.GOOS == "windows" {
		uiName += ".exe"
	}

	uiPath := filepath.Join(exDir, uiName)
	if _, err := os.Stat(uiPath); os.IsNotExist(err) {
		// Try fallback like parent dir?
		// For now assume same dir
		fmt.Println("UI executable not found at:", uiPath)
		return
	}

	cmd := exec.Command(uiPath)
	if err := cmd.Start(); err != nil {
		fmt.Println("Failed to start UI:", err)
	}
}

func onExit() {
	sseClientsMu.Lock()
	clientCount := len(sseClients)
	sseClientsMu.Unlock()

	if clientCount > 0 {
		// Broadcast shutdown signal
		broadcastMSE("shutdown")
		// Give UI some time to process and close itself
		// Reduced from 500ms to 100ms for faster exit feeling
		time.Sleep(100 * time.Millisecond)
	} else {
		// No UI connected, skip killUI to avoid lag
		fmt.Println("No UI connected, exiting immediately.")
	}

	// User requested to remove taskkill and rely on SSE
	// if clientCount > 0 {
	// 	killUI()
	// }

	if clipMonitor != nil {
		clipMonitor.Stop()
	}
	if server != nil {
		server.Stop()
	}
	if client != nil {
		client.Disconnect()
	}
	// systray.Quit() // already quitting if called from systray
}

// killUI removed as per user request to rely on SSE
// func killUI() { ... }

func handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	messageChan := make(chan string)

	sseClientsMu.Lock()
	sseClients[messageChan] = true
	sseClientsMu.Unlock()

	defer func() {
		sseClientsMu.Lock()
		delete(sseClients, messageChan)
		sseClientsMu.Unlock()
		close(messageChan)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Send initial connection message to flush headers
	// Without this, Go HTTP never sends headers back, and client.Get() blocks forever
	fmt.Fprintf(w, "data: connected\n\n")
	flusher.Flush()
	fmt.Fprintf(w, "data: %s\n\n", statusEventMessage())
	flusher.Flush()

	notify := r.Context().Done()

	for {
		select {
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-notify:
			return
		}
	}
}

func broadcastMSE(msg string) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()
	for clientChan := range sseClients {
		// Non-blocking send
		select {
		case clientChan <- msg:
		default:
		}
	}
}

func getCurrentStatus() shared.StatusResponse {
	status := shared.StatusResponse{
		Running:         false,
		ClientConnected: false,
		ClientCount:     0,
		LastCopied:      lastCopied,
		Message:         "Service Running",
	}

	if server != nil {
		status.Running = server.IsRunning()
		status.ClientCount = server.GetClientCount()
	}
	if client != nil {
		status.ClientConnected = client.IsConnected()
	}

	if cfg != nil && cfg.Mode == "server" {
		if status.Running {
			status.Message = "Server Running"
		} else {
			status.Message = "Server Stopped"
		}
	} else {
		// Client mode: use Running field as the active connection state
		status.Running = status.ClientConnected
		if status.Running {
			status.Message = "Client Connected"
		} else {
			status.Message = "Client Disconnected"
		}
	}

	return status
}

func statusEventMessage() string {
	data, _ := json.Marshal(map[string]interface{}{
		"type":   "status",
		"status": getCurrentStatus(),
	})
	return string(data)
}

func broadcastStatus() {
	broadcastMSE(statusEventMessage())
}

func initMonitor() {
	clipMonitor.Start()

	clipMonitor.OnChange = func(content string) {
		if content == lastCopied {
			return
		}
		lastCopied = content
		broadcastStatus()

		fmt.Println("Clipboard Changed:", content)

		if cfg.SyncMode == "receive_only" {
			return
		}

		if cfg.Mode == "server" && server.IsRunning() {
			server.BroadcastClipboard(content, "server")
		} else if cfg.Mode == "client" && client.IsConnected() {
			client.SendClipboard(content, "client")
		}
	}

	server.OnClipboardReceived = func(content string) {
		if cfg.SyncMode == "send_only" {
			return
		}
		if content != lastCopied {
			clipMonitor.SetContent(content)
			lastCopied = content
			broadcastStatus()
			fmt.Println("Remote Clipboard Received:", content)
		}
	}

	server.OnFileCopyReceived = func(meta *sync.FileMeta) {
		if cfg.SyncMode == "send_only" {
			return
		}
		// Push SSE to Wails
		data, _ := json.Marshal(map[string]interface{}{
			"type": "file_copy",
			"meta": meta,
		})
		broadcastMSE(string(data))
	}

	client.OnClipboardReceived = func(content string) {
		if cfg.SyncMode == "send_only" {
			return
		}
		if content != lastCopied {
			clipMonitor.SetContent(content)
			lastCopied = content
			broadcastStatus()
			fmt.Println("Remote Clipboard Received:", content)
		}
	}

	client.OnFileCopyReceived = func(meta *sync.FileMeta) {
		if cfg.SyncMode == "send_only" {
			return
		}
		// Push SSE to Wails
		data, _ := json.Marshal(map[string]interface{}{
			"type": "file_copy",
			"meta": meta,
		})
		broadcastMSE(string(data))
	}

	server.OnClientConnected = func(count int) {
		_ = count
		broadcastStatus()
	}

	server.OnClientDisconnected = func(count int) {
		_ = count
		broadcastStatus()
	}

	client.OnConnected = func() {
		broadcastStatus()
	}

	client.OnDisconnected = func() {
		broadcastStatus()
	}
}

func startSync() {
	if cfg.Mode == "server" {
		go server.Start(cfg.ServerPort)
	} else {
		go client.Connect(cfg.ServerAddress)
	}
	broadcastStatus()
}

func stopSync() {
	server.Stop()
	client.Disconnect()
	broadcastStatus()
}

// HTTP Handlers

func handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		json.NewEncoder(w).Encode(cfg)
	} else if r.Method == "POST" {
		var newCfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Check if we need to restart
		wasRunning := server.IsRunning() || client.IsConnected()

		// Apply changes
		cfg.Mode = newCfg.Mode
		cfg.ServerPort = newCfg.ServerPort
		cfg.ServerAddress = newCfg.ServerAddress
		cfg.AutoStart = newCfg.AutoStart
		cfg.SyncMode = newCfg.SyncMode
		cfg.Save()

		// Restart sync if it was running, to apply new config
		if wasRunning {
			stopSync()
			// Short delay to ensure port release if needed
			time.Sleep(100 * time.Millisecond)
			startSync()
		}
		broadcastStatus()

		json.NewEncoder(w).Encode(cfg)
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(getCurrentStatus())
}

func handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		startSync()
		w.WriteHeader(http.StatusOK)
	}
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		stopSync()
		w.WriteHeader(http.StatusOK)
	}
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		systray.Quit()
	}
}

func handleAcceptFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req shared.AcceptFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 准备接收文件
	var transfer *sync.FileTransferManager
	var isServer bool

	if cfg.Mode == "server" && server != nil && server.IsRunning() {
		transfer = server.GetTransferManager()
		isServer = true
	} else if cfg.Mode == "client" && client != nil && client.IsConnected() {
		transfer = client.GetTransferManager()
		isServer = false
	} else {
		http.Error(w, "Not connected", http.StatusServiceUnavailable)
		return
	}

	if err := transfer.PrepareIncomingFile(req.FileID, req.SavePath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isServer {
		server.BroadcastFileRequest(req.FileID, "server")
	} else {
		client.SendFileRequest(req.FileID, "client")
	}

	w.WriteHeader(http.StatusOK)
}

func handleSendFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req shared.SendFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var transfer *sync.FileTransferManager
	var isServer bool

	if cfg.Mode == "server" && server != nil && server.IsRunning() {
		transfer = server.GetTransferManager()
		isServer = true
	} else if cfg.Mode == "client" && client != nil && client.IsConnected() {
		transfer = client.GetTransferManager()
		isServer = false
	} else {
		http.Error(w, "Not connected", http.StatusServiceUnavailable)
		return
	}

	for _, p := range req.Paths {
		meta, err := transfer.GetFileInfo(p)
		if err != nil {
			continue // Skip invalid/directories for now
		}

		if err := transfer.StartOutgoingFile(meta); err != nil {
			continue
		}

		if isServer {
			server.BroadcastFileCopy(meta, "server")
		} else {
			client.SendFileCopy(meta, "client")
		}
	}

	w.WriteHeader(http.StatusOK)
}
