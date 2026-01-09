package sync

import "time"

// MessageType 消息类型
type MessageType string

const (
	TypeClipboard MessageType = "clipboard" // 剪贴板内容
	TypePing      MessageType = "ping"      // 心跳检测
	TypePong      MessageType = "pong"      // 心跳响应
)

// Message WebSocket 通信消息
type Message struct {
	Type      MessageType `json:"type"`      // 消息类型
	Content   string      `json:"content"`   // 剪贴板内容
	Timestamp int64       `json:"timestamp"` // 时间戳
	Source    string      `json:"source"`    // 来源标识
}

// NewClipboardMessage 创建剪贴板消息
func NewClipboardMessage(content, source string) *Message {
	return &Message{
		Type:      TypeClipboard,
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
		Source:    source,
	}
}

// NewPingMessage 创建心跳消息
func NewPingMessage() *Message {
	return &Message{
		Type:      TypePing,
		Timestamp: time.Now().UnixMilli(),
	}
}

// NewPongMessage 创建心跳响应消息
func NewPongMessage() *Message {
	return &Message{
		Type:      TypePong,
		Timestamp: time.Now().UnixMilli(),
	}
}
