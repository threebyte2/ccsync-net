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
	ignoreNext  bool
	ignoreLock  sync.Mutex

	// 回调函数
	OnChange func(content string)
	OnLog    func(msg string)
}

// NewMonitor 创建剪贴板监听器
func NewMonitor() *Monitor {
	return &Monitor{}
}

// Init 初始化剪贴板（必须在主线程调用）
func (m *Monitor) Init() error {
	return clipboard.Init()
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

// SetContent 设置剪贴板内容（会忽略下一次变化事件）
func (m *Monitor) SetContent(content string) {
	m.ignoreLock.Lock()
	m.ignoreNext = true
	m.ignoreLock.Unlock()

	m.lastLock.Lock()
	m.lastContent = content
	m.lastLock.Unlock()

	clipboard.Write(clipboard.FmtText, []byte(content))
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

	// 检查是否需要忽略
	m.ignoreLock.Lock()
	if m.ignoreNext {
		m.ignoreNext = false
		m.ignoreLock.Unlock()
		m.lastLock.Lock()
		m.lastContent = current
		m.lastLock.Unlock()
		return
	}
	m.ignoreLock.Unlock()

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
