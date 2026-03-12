package sync

import "time"

// MessageType 消息类型
type MessageType string

const (
	TypeClipboard   MessageType = "clipboard"    // 剪贴板内容
	TypePing        MessageType = "ping"         // 心跳检测
	TypePong        MessageType = "pong"         // 心跳响应
	TypeFileCopy    MessageType = "file_copy"    // 文件复制信令
	TypeFileRequest MessageType = "file_request" // 请求传输文件
	TypeFileChunk   MessageType = "file_chunk"   // 文件数据块
)

// FileMeta 提供文件复制的元数据信息
type FileMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	Hash string `json:"hash"`
	Path string `json:"path"` // 发送端的源绝对路径，仅发送端可见，不发送给接收端
}

// FileChunk 提供文件传输的二进制分块
type FileChunk struct {
	FileID string `json:"file_id"`
	Offset int64  `json:"offset"`
	Data   []byte `json:"data"` // Go 的 json 包会自动将 []byte 转换为 base64
	IsLast bool   `json:"is_last"`
}

// Message WebSocket 通信消息
type Message struct {
	Type      MessageType `json:"type"`                  // 消息类型
	Content   string      `json:"content"`               // 剪贴板内容
	Timestamp int64       `json:"timestamp"`             // 时间戳
	Source    string      `json:"source"`                // 来源标识
	FileMeta  *FileMeta   `json:"file_meta,omitempty"`   // 文件元信息
	FileChunk *FileChunk  `json:"file_chunk,omitempty"`  // 文件数据分块
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

// NewFileCopyMessage 创建文件复制消息
func NewFileCopyMessage(meta *FileMeta, source string) *Message {
	// 清理掉绝对路径防止隐私泄露
	safeMeta := *meta
	safeMeta.Path = ""
	return &Message{
		Type:      TypeFileCopy,
		FileMeta:  &safeMeta,
		Timestamp: time.Now().UnixMilli(),
		Source:    source,
	}
}

// NewFileRequestMessage 请求拉取文件
func NewFileRequestMessage(fileID, source string) *Message {
	return &Message{
		Type:      TypeFileRequest,
		Content:   fileID,
		Timestamp: time.Now().UnixMilli(),
		Source:    source,
	}
}

// NewFileChunkMessage 创建分块数据消息
func NewFileChunkMessage(chunk *FileChunk, source string) *Message {
	return &Message{
		Type:      TypeFileChunk,
		FileChunk: chunk,
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
