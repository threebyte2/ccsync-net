package sync

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client WebSocket 客户端
type Client struct {
	serverAddr  string
	conn        *websocket.Conn
	connected   bool
	connLock    sync.RWMutex
	stopChan    chan struct{}
	reconnect   bool

	// 回调函数
	OnClipboardReceived func(content string)
	OnConnected         func()
	OnDisconnected      func()
	OnLog               func(msg string)
}

// NewClient 创建客户端实例
func NewClient() *Client {
	return &Client{
		stopChan: make(chan struct{}),
	}
}

// Connect 连接到服务端
func (c *Client) Connect(serverAddr string) error {
	c.connLock.Lock()
	if c.connected {
		c.connLock.Unlock()
		return nil
	}
	c.serverAddr = serverAddr
	c.reconnect = true
	c.connLock.Unlock()

	go c.connectLoop()
	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() {
	c.connLock.Lock()
	c.reconnect = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
	c.connLock.Unlock()

	select {
	case c.stopChan <- struct{}{}:
	default:
	}

	c.log("已断开连接")
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.connLock.RLock()
	defer c.connLock.RUnlock()
	return c.connected
}

// SendClipboard 发送剪贴板内容
func (c *Client) SendClipboard(content, source string) error {
	c.connLock.RLock()
	conn := c.conn
	connected := c.connected
	c.connLock.RUnlock()

	if !connected || conn == nil {
		return nil
	}

	msg := NewClipboardMessage(content, source)
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) connectLoop() {
	for {
		c.connLock.RLock()
		shouldReconnect := c.reconnect
		serverAddr := c.serverAddr
		c.connLock.RUnlock()

		if !shouldReconnect {
			return
		}

		url := "ws://" + serverAddr + "/ws"
		c.log("正在连接 " + url)

		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			c.log("连接失败: " + err.Error())
			time.Sleep(3 * time.Second)
			continue
		}

		c.connLock.Lock()
		c.conn = conn
		c.connected = true
		c.connLock.Unlock()

		c.log("连接成功")
		if c.OnConnected != nil {
			c.OnConnected()
		}

		c.readLoop(conn)

		c.connLock.Lock()
		c.connected = false
		c.conn = nil
		c.connLock.Unlock()

		if c.OnDisconnected != nil {
			c.OnDisconnected()
		}

		c.connLock.RLock()
		shouldReconnect = c.reconnect
		c.connLock.RUnlock()

		if shouldReconnect {
			c.log("连接断开，3秒后重连...")
			time.Sleep(3 * time.Second)
		}
	}
}

func (c *Client) readLoop(conn *websocket.Conn) {
	// 启动心跳
	go c.heartbeat(conn)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			c.log("读取消息失败: " + err.Error())
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case TypeClipboard:
			if c.OnClipboardReceived != nil {
				c.OnClipboardReceived(msg.Content)
			}
		case TypePong:
			// 心跳响应，忽略
		}
	}
}

func (c *Client) heartbeat(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.connLock.RLock()
			connected := c.connected
			c.connLock.RUnlock()

			if !connected {
				return
			}

			ping := NewPingMessage()
			data, _ := json.Marshal(ping)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-c.stopChan:
			return
		}
	}
}

func (c *Client) log(msg string) {
	log.Println("[Client]", msg)
	if c.OnLog != nil {
		c.OnLog(msg)
	}
}
