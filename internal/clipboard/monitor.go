package clipboard

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"golang.design/x/clipboard"
)

// Monitor 剪贴板监听器
type Monitor struct {
	running     bool
	runningLock sync.RWMutex
	cancelFunc  context.CancelFunc
	lastContent string
	lastLock    sync.RWMutex
	// 回调函数
	OnChange func(content string)
	OnLog    func(msg string)
}

// NewMonitor 创建剪贴板监听器
func NewMonitor() *Monitor {
	return &Monitor{}
}

// Init 初始化剪贴板
func (m *Monitor) Init() error {
	return nil
}

// Start 开始监听剪贴板变化
func (m *Monitor) Start() error {
	m.runningLock.Lock()
	if m.running {
		m.runningLock.Unlock()
		return nil
	}
	m.running = true
	m.runningLock.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel

	go m.watchLoop(ctx)

	m.log("Clipboard monitor started")
	return nil
}

// Stop 停止监听
func (m *Monitor) Stop() {
	m.runningLock.Lock()
	defer m.runningLock.Unlock()

	if !m.running {
		return
	}

	m.running = false
	if m.cancelFunc != nil {
		m.cancelFunc()
	}

	m.log("Clipboard monitor stopped")
}

// IsRunning 检查是否在运行
func (m *Monitor) IsRunning() bool {
	m.runningLock.RLock()
	defer m.runningLock.RUnlock()
	return m.running
}

// SetContent 设置剪贴板内容
func (m *Monitor) SetContent(content string) {
	if runtime.GOOS == "linux" {
		m.setContentLinux(content)
	} else {
		// Fallback to library for other OS
		clipboard.Write(clipboard.FmtText, []byte(content))
	}

	// 主动更新 lastContent (存储清理后的版本)
	m.setLastContent(cleanContent(content))
}

func (m *Monitor) setContentLinux(content string) {
	cmd := exec.Command("wl-copy")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		m.log(fmt.Sprintf("Failed to create stdin pipe for wl-copy: %v", err))
		return
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, content)
	}()

	if err := cmd.Run(); err != nil {
		m.log(fmt.Sprintf("wl-copy failed: %v", err))
	}
}

// GetContent 获取当前剪贴板内容
func (m *Monitor) GetContent() string {
	var content string
	if runtime.GOOS == "linux" {
		out, err := exec.Command("wl-paste").Output()
		if err == nil {
			content = string(out)
		}
	} else {
		// Fallback to library
		content = string(clipboard.Read(clipboard.FmtText))
	}
	// 返回原始内容，清理逻辑在使用处处理
	return content
}

func (m *Monitor) watchLoop(ctx context.Context) {
	if runtime.GOOS == "linux" {
		m.watchLoopLinux(ctx)
		return
	}
	
	// Non-Linux fallback using library Watch
	// 初始化
	initial := m.GetContent()
	m.setLastContent(cleanContent(initial))

	changed := clipboard.Watch(ctx, clipboard.FmtText)
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-changed:
			if !ok {
				return
			}
			content := string(data)
			m.processChange(content)
		}
	}
}

func (m *Monitor) watchLoopLinux(ctx context.Context) {
	// Initialize last content
	m.setLastContent(cleanContent(m.GetContent()))

	m.log("Starting wl-paste --watch monitor...")

	// Use wl-paste --watch to detect changes.
	cmd := exec.CommandContext(ctx, "wl-paste", "--watch", "echo", "change")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.log(fmt.Sprintf("Failed to start wl-paste watcher: %v", err))
		return
	}

	if err := cmd.Start(); err != nil {
		m.log(fmt.Sprintf("Failed to run wl-paste watcher: %v", err))
		return
	}

	reader := bufio.NewReader(stdout)
	
	go func() {
		// Wait for command to finish (when context cancelled)
		cmd.Wait()
	}()

	for {
		// Read line effectively waits for the next "change" output
		_, err := reader.ReadString('\n')
		if err != nil {
			// Process ended or error
			return
		}

		// Clipboard changed
		// We add a small delay to ensure content is ready or just read it
		content := m.GetContent()
		m.processChange(content)
	}
}

func (m *Monitor) processChange(current string) {
	cleaned := cleanContent(current)

	m.lastLock.RLock()
	lastContent := m.lastContent
	m.lastLock.RUnlock()

	// 比较清理后的内容，忽略无效空白字符差异
	if cleaned == lastContent {
		return
	}

	m.setLastContent(cleaned)

	m.log("Content changed detected")

	// 触发回调，传递清理后的内容，解决多余换行问题
	if m.OnChange != nil && cleaned != "" {
		m.OnChange(cleaned)
	}
}

func (m *Monitor) setLastContent(content string) {
	m.lastLock.Lock()
	m.lastContent = content
	m.lastLock.Unlock()
}

func (m *Monitor) log(msg string) {
	// Only log significant events, or if OnLog is set.
	// User requested to remove useless print content.
	// We will keep minimal logging.
	if m.OnLog != nil {
		m.OnLog(msg)
	}
}

func cleanContent(str string) string {
	// 仅移除尾部的换行符(包括 \r 和 \n)，保留其他空白
	return strings.TrimRight(str, "\r\n")
}

func preview(str string) string {
	str = cleanContent(str)
	if len(str) > 20 {
		return str[:20] + "..."
	}
	return str
}
