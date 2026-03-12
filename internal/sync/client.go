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
	serverAddr string
	conn       *websocket.Conn
	connected  bool
	connLock   sync.RWMutex
	stopChan   chan struct{}
	reconnect  bool
	transfer   *FileTransferManager

	// 回调函数
	OnClipboardReceived  func(content string)
	OnFileCopyReceived   func(meta *FileMeta)
	OnFileRequestReceived func(fileID string)
	OnFileChunkReceived  func(chunk *FileChunk)
	OnConnected         func()
	OnDisconnected      func()
	OnLog               func(msg string)
}

// NewClient 创建客户端实例
func NewClient() *Client {
	return &Client{
		stopChan: make(chan struct{}),
		transfer: NewFileTransferManager(),
	}
}

// Connect 连接到服务端
func (c *Client) Connect(serverAddr string) error {
	c.connLock.Lock()
	if c.connected || c.reconnect {
		// Already connected or already trying to connect
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

// SendFileRequest 发送拉取文件请求
func (c *Client) SendFileRequest(fileID string, source string) error {
	c.connLock.RLock()
	conn := c.conn
	connected := c.connected
	c.connLock.RUnlock()

	if !connected || conn == nil {
		return nil
	}

	msg := NewFileRequestMessage(fileID, source)
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

// SendFileChunk 发送文件分块数据
func (c *Client) SendFileChunk(chunk *FileChunk, source string) error {
	c.connLock.RLock()
	conn := c.conn
	connected := c.connected
	c.connLock.RUnlock()

	if !connected || conn == nil {
		return nil
	}

	msg := NewFileChunkMessage(chunk, source)
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}



// SendFileCopy 发送文件复制信令
func (c *Client) SendFileCopy(meta *FileMeta, source string) error {
	c.connLock.RLock()
	conn := c.conn
	connected := c.connected
	c.connLock.RUnlock()

	if !connected || conn == nil {
		return nil
	}

	msg := NewFileCopyMessage(meta, source)
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
		case TypeFileCopy:
			if c.OnFileCopyReceived != nil && msg.FileMeta != nil {
				c.OnFileCopyReceived(msg.FileMeta)
			}
		case TypeFileRequest:
			if c.OnFileRequestReceived != nil {
				c.OnFileRequestReceived(msg.Content)
			}
			// 如果是向本地请求分块，开启协程发送
			go c.startSendingFile(msg.Content, msg.Source)
		case TypeFileChunk:
			if c.OnFileChunkReceived != nil && msg.FileChunk != nil {
				c.OnFileChunkReceived(msg.FileChunk)
			}
			// 写入本地
			if msg.FileChunk != nil {
				c.transfer.WriteFileChunk(msg.FileChunk)
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

// ---------------------- File Transfer APIs ----------------------

func (c *Client) GetTransferManager() *FileTransferManager {
	return c.transfer
}

func (c *Client) startSendingFile(fileID string, requester string) {
	// Chunk size 1MB
	chunkSize := 1024 * 1024
	var offset int64 = 0
	
	for {
		chunk, err := c.transfer.ReadFileChunk(fileID, offset, chunkSize)
		if err != nil {
			c.log("Error reading file chunk: " + err.Error())
			break
		}
		
		c.SendFileChunk(chunk, "client")
		if chunk.IsLast {
			break
		}
		offset += int64(len(chunk.Data))
	}
}
