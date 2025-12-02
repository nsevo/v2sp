package conf

// Hy2Config contains Hysteria2 core configuration
type Hy2Config struct {
	LogConfig  *Hy2LogConfig `json:"Log"`
	BinaryPath string        `json:"BinaryPath"` // Path to hysteria binary
	ConfigDir  string        `json:"ConfigDir"`  // Directory for node configs
}

// Hy2LogConfig contains Hysteria2 log configuration
type Hy2LogConfig struct {
	Level     string `json:"Level"`
	ErrorPath string `json:"ErrorPath"`
}

// NewHy2Config creates a new Hy2Config with default values
func NewHy2Config() *Hy2Config {
	return &Hy2Config{
		LogConfig: &Hy2LogConfig{
			Level:     "info",
			ErrorPath: "/etc/v2sp/hy2_error.log",
		},
		BinaryPath: "/usr/local/bin/hysteria",
		ConfigDir:  "/etc/v2sp/hy2",
	}
}
