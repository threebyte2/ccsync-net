package sync

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// Server WebSocket 服务端
type Server struct {
	port        int
	clients     map[*websocket.Conn]bool
	clientsLock sync.RWMutex
	server      *http.Server
	running     bool
	runningLock sync.RWMutex

	// 回调函数
	OnClipboardReceived func(content string)
	OnClientConnected   func(count int)
	OnClientDisconnected func(count int)
	OnLog               func(msg string)
}

// NewServer 创建服务端实例
func NewServer() *Server {
	return &Server{
		clients: make(map[*websocket.Conn]bool),
	}
}

// Start 启动服务端
func (s *Server) Start(port int) error {
	s.runningLock.Lock()
	if s.running {
		s.runningLock.Unlock()
		return nil
	}
	s.port = port
	s.runningLock.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleConnection)

	s.server = &http.Server{
		Addr:    ":" + string(rune(port)),
		Handler: mux,
	}

	// 修正端口格式
	addr := "0.0.0.0:" + itoa(port)
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	s.runningLock.Lock()
	s.running = true
	s.runningLock.Unlock()

	s.log("服务端启动于 " + addr)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log("服务端错误: " + err.Error())
			s.runningLock.Lock()
			s.running = false
			s.runningLock.Unlock()
		}
	}()

	return nil
}

// Stop 停止服务端
func (s *Server) Stop() error {
	s.runningLock.Lock()
	defer s.runningLock.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	// 关闭所有客户端连接
	s.clientsLock.Lock()
	for conn := range s.clients {
		conn.Close()
	}
	s.clients = make(map[*websocket.Conn]bool)
	s.clientsLock.Unlock()

	if s.server != nil {
		s.server.Close()
	}

	s.log("服务端已停止")
	return nil
}

// IsRunning 检查服务端是否运行中
func (s *Server) IsRunning() bool {
	s.runningLock.RLock()
	defer s.runningLock.RUnlock()
	return s.running
}

// GetClientCount 获取连接的客户端数量
func (s *Server) GetClientCount() int {
	s.clientsLock.RLock()
	defer s.clientsLock.RUnlock()
	return len(s.clients)
}

// Broadcast 广播消息给所有客户端
func (s *Server) Broadcast(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		s.log("消息序列化失败: " + err.Error())
		return
	}

	s.clientsLock.RLock()
	defer s.clientsLock.RUnlock()

	for conn := range s.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			s.log("发送消息失败: " + err.Error())
		}
	}
}

// BroadcastClipboard 广播剪贴板内容
func (s *Server) BroadcastClipboard(content, source string) {
	msg := NewClipboardMessage(content, source)
	s.Broadcast(msg)
}

func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log("连接升级失败: " + err.Error())
		return
	}

	s.clientsLock.Lock()
	s.clients[conn] = true
	count := len(s.clients)
	s.clientsLock.Unlock()

	s.log("新客户端连接，当前连接数: " + itoa(count))
	if s.OnClientConnected != nil {
		s.OnClientConnected(count)
	}

	defer func() {
		s.clientsLock.Lock()
		delete(s.clients, conn)
		count := len(s.clients)
		s.clientsLock.Unlock()
		conn.Close()

		s.log("客户端断开，当前连接数: " + itoa(count))
		if s.OnClientDisconnected != nil {
			s.OnClientDisconnected(count)
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case TypeClipboard:
			if s.OnClipboardReceived != nil {
				s.OnClipboardReceived(msg.Content)
			}
			// 转发给其他客户端
			s.clientsLock.RLock()
			for c := range s.clients {
				if c != conn {
					c.WriteMessage(websocket.TextMessage, data)
				}
			}
			s.clientsLock.RUnlock()

		case TypePing:
			pong := NewPongMessage()
			pongData, _ := json.Marshal(pong)
			conn.WriteMessage(websocket.TextMessage, pongData)
		}
	}
}

func (s *Server) log(msg string) {
	log.Println("[Server]", msg)
	if s.OnLog != nil {
		s.OnLog(msg)
	}
}

// 简单的 int 转 string
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var result string
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if negative {
		result = "-" + result
	}
	return result
}
