package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 应用配置
type Config struct {
	// 模式: "server" 或 "client"
	Mode string `json:"mode"`

	// 服务端配置
	ServerPort int `json:"serverPort"`

	// 客户端配置
	ServerAddress string `json:"serverAddress"`

	// 是否自动启动
	AutoStart bool `json:"autoStart"`

	// 同步模式: "bidirectional", "send_only", "receive_only"
	SyncMode string `json:"syncMode"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Mode:          "server",
		ServerPort:    8765,
		ServerAddress: "127.0.0.1:8765",
		AutoStart:     false,
		SyncMode:      "bidirectional",
	}
}

// configPath 获取配置文件路径
func configPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(homeDir, ".ccsync-net")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

// Load 加载配置
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), nil
	}

	return &cfg, nil
}

// Save 保存配置
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
