package sync

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// FileTransferManager 管理正在进行的接收或发送的文件流
type FileTransferManager struct {
	// 用于存储正在被对方读取的文件 (FileID -> File Object)
	outgoingFiles map[string]*os.File
	outgoingLock  sync.RWMutex

	// 用于存储正在写入到本地的文件 (FileID -> File Object)
	incomingFiles map[string]*os.File
	incomingLock  sync.RWMutex
}

func NewFileTransferManager() *FileTransferManager {
	return &FileTransferManager{
		outgoingFiles: make(map[string]*os.File),
		incomingFiles: make(map[string]*os.File),
	}
}

// GenerateFileID 基于文件路径和大小生成唯一ID
func GenerateFileID(path string, size int64) string {
	hash := md5.Sum([]byte(fmt.Sprintf("%s_%d", path, size)))
	return hex.EncodeToString(hash[:])
}

// GenerateFileHash 获取文件的MD5，只读取一点点信息或者整个，看需求，这里简单hash路径和size作为简易版本
func GenerateFileHash(path string, size int64) string {
	return GenerateFileID(path, size)
}

// GetFileInfo 准备文件元数据并打开供读取
func (m *FileTransferManager) GetFileInfo(path string) (*FileMeta, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, fmt.Errorf("directories are not supported yet")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	size := info.Size()
	id := GenerateFileID(absPath, size)
	hash := GenerateFileHash(absPath, size)

	meta := &FileMeta{
		ID:   id,
		Name: info.Name(),
		Size: size,
		Hash: hash,
		Path: absPath,
	}

	return meta, nil
}

// StartOutgoingFile 注册一个文件准备发送给对方
func (m *FileTransferManager) StartOutgoingFile(meta *FileMeta) error {
	file, err := os.Open(meta.Path)
	if err != nil {
		return err
	}

	m.outgoingLock.Lock()
	m.outgoingFiles[meta.ID] = file
	m.outgoingLock.Unlock()
	return nil
}

// StopOutgoingFile 完成或中止发送时关闭文件
func (m *FileTransferManager) StopOutgoingFile(fileID string) {
	m.outgoingLock.Lock()
	defer m.outgoingLock.Unlock()

	if file, exists := m.outgoingFiles[fileID]; exists {
		file.Close()
		delete(m.outgoingFiles, fileID)
	}
}

// ReadFileChunk 读取指定偏移量的数据块
func (m *FileTransferManager) ReadFileChunk(fileID string, offset int64, chunkSize int) (*FileChunk, error) {
	m.outgoingLock.RLock()
	file, exists := m.outgoingFiles[fileID]
	m.outgoingLock.RUnlock()

	if !exists {
		return nil, fmt.Errorf("file not open for sending")
	}

	data := make([]byte, chunkSize)
	n, err := file.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}

	isLast := err == io.EOF || n < chunkSize

	return &FileChunk{
		FileID: fileID,
		Offset: offset,
		Data:   data[:n],
		IsLast: isLast,
	}, nil
}

// PrepareIncomingFile 准备接收文件
func (m *FileTransferManager) PrepareIncomingFile(fileID string, savePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(savePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	m.incomingLock.Lock()
	m.incomingFiles[fileID] = file
	m.incomingLock.Unlock()
	return nil
}

// WriteFileChunk 将数据块写入磁盘
func (m *FileTransferManager) WriteFileChunk(chunk *FileChunk) error {
	m.incomingLock.RLock()
	file, exists := m.incomingFiles[chunk.FileID]
	m.incomingLock.RUnlock()

	if !exists {
		return fmt.Errorf("incoming file not prepared")
	}

	_, err := file.WriteAt(chunk.Data, chunk.Offset)
	if chunk.IsLast {
		// Close when done
		m.StopIncomingFile(chunk.FileID)
	}
	return err
}

// StopIncomingFile 停止/关闭接收的文件
func (m *FileTransferManager) StopIncomingFile(fileID string) {
	m.incomingLock.Lock()
	defer m.incomingLock.Unlock()

	if file, exists := m.incomingFiles[fileID]; exists {
		file.Close()
		delete(m.incomingFiles, fileID)
	}
}
