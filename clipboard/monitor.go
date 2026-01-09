package clipboard

import (
	"context"
	"log"
	"sync"
	"time"

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

// Init 初始化剪贴板（已移至 main.go，此处仅保留接口兼容）
func (m *Monitor) Init() error {
	// 实际初始化必须在 main 线程完成
	// 这里可以添加检查或者直接返回 nil
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

	m.log("剪贴板监听已启动")
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

	m.log("剪贴板监听已停止")
}

// IsRunning 检查是否在运行
func (m *Monitor) IsRunning() bool {
	m.runningLock.RLock()
	defer m.runningLock.RUnlock()
	return m.running
}

// SetContent 设置剪贴板内容
func (m *Monitor) SetContent(content string) {
	// 记录日志确保调用
	m.log("设置剪贴板内容: " + preview(content))
	
	// 直接写入，不更新 lastContent，让 watchLoop 自然发现变化
	// 这样可以避免 "Old" read race 导致的状态混乱
	// 同时也依赖 App 层做回环检测
	
	// 写入剪贴板
	start := time.Now()
	clipboard.Write(clipboard.FmtText, []byte(content))
	m.log("写入耗时: " + time.Since(start).String())
}

func preview(str string) string {
	if len(str) > 20 {
		return str[:20] + "..."
	}
	return str
}

// GetContent 获取当前剪贴板内容
func (m *Monitor) GetContent() string {
	data := clipboard.Read(clipboard.FmtText)
	return string(data)
}

func (m *Monitor) watchLoop(ctx context.Context) {
	// 初始化最后内容
	m.lastLock.Lock()
	m.lastContent = m.GetContent()
	m.lastLock.Unlock()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Debug log to ensure detection is running (reduce frequency if too noisy)
			// m.log("Checking clipboard...") 
			m.checkClipboard()
		}
	}
}

func (m *Monitor) checkClipboard() {
	current := m.GetContent()

	m.lastLock.RLock()
	lastContent := m.lastContent
	m.lastLock.RUnlock()

	if current == lastContent {
		return
	}

	// 更新最后内容
	m.lastLock.Lock()
	m.lastContent = current
	m.lastLock.Unlock()

	// 触发回调
	if m.OnChange != nil && current != "" {
		m.OnChange(current)
	}
}

func (m *Monitor) log(msg string) {
	log.Println("[Clipboard]", msg)
	if m.OnLog != nil {
		m.OnLog(msg)
	}
}
