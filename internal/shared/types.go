package shared

// ConfigRequest represents a request to update configuration
type ConfigRequest struct {
	Mode          string `json:"mode"`
	ServerPort    int    `json:"serverPort"`
	ServerAddress string `json:"serverAddress"`
	AutoStart     bool   `json:"autoStart"`
	SyncMode      string `json:"syncMode"`
}

// StatusResponse represents the current status of the service
type StatusResponse struct {
	Running         bool   `json:"running"`
	ClientConnected bool   `json:"client_connected"`
	ClientCount     int    `json:"client_count"`
	LastCopied      string `json:"last_copied"`
	Message         string `json:"message"`
}
