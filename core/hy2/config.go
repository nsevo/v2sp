package hy2

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nsevo/v2sp/api/panel"
	"github.com/nsevo/v2sp/conf"
	"gopkg.in/yaml.v3"
)

// sanitizeFilename removes or replaces characters that are invalid in filenames
func sanitizeFilename(name string) string {
	// Replace common URL/path separators
	name = strings.ReplaceAll(name, "://", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "[", "")
	name = strings.ReplaceAll(name, "]", "")
	// Remove any remaining invalid characters
	re := regexp.MustCompile(`[<>"|?*]`)
	name = re.ReplaceAllString(name, "")
	// Collapse multiple underscores
	re = regexp.MustCompile(`_+`)
	name = re.ReplaceAllString(name, "_")
	// Trim underscores from ends
	name = strings.Trim(name, "_")
	return name
}

// Hy2ServerConfig represents Hysteria2 server configuration
type Hy2ServerConfig struct {
	Listen    string          `yaml:"listen"`
	TLS       *Hy2TLSConfig   `yaml:"tls,omitempty"`
	Auth      *Hy2AuthConfig  `yaml:"auth"`
	Masq      *Hy2MasqConfig  `yaml:"masquerade,omitempty"`
	Stats     *Hy2StatsConfig `yaml:"trafficStats,omitempty"`
	Outbounds []Hy2Outbound   `yaml:"outbounds,omitempty"`
	ACL       *Hy2ACLConfig   `yaml:"acl,omitempty"`
}

// Hy2TLSConfig represents TLS configuration
type Hy2TLSConfig struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

// Hy2AuthConfig represents authentication configuration
type Hy2AuthConfig struct {
	Type     string `yaml:"type"`
	Password string `yaml:"password,omitempty"`
	// For userpass type
	UserPass map[string]string `yaml:"userpass,omitempty"`
}

// Hy2MasqConfig represents masquerade configuration
type Hy2MasqConfig struct {
	Type string `yaml:"type"`
	File struct {
		Dir string `yaml:"dir"`
	} `yaml:"file,omitempty"`
	Proxy struct {
		URL         string `yaml:"url"`
		RewriteHost bool   `yaml:"rewriteHost"`
	} `yaml:"proxy,omitempty"`
}

// Hy2StatsConfig represents traffic stats API configuration
type Hy2StatsConfig struct {
	Listen string `yaml:"listen"`
	Secret string `yaml:"secret,omitempty"`
}

// Hy2Outbound represents an outbound configuration
type Hy2Outbound struct {
	Name   string           `yaml:"name"`
	Type   string           `yaml:"type"`
	Direct *Hy2DirectConfig `yaml:"direct,omitempty"`
}

// Hy2DirectConfig represents direct outbound configuration
type Hy2DirectConfig struct {
	Mode string `yaml:"mode,omitempty"` // auto, 4, 6, 46, 64
}

// Hy2ACLConfig represents ACL configuration
type Hy2ACLConfig struct {
	Inline []string `yaml:"inline,omitempty"`
	File   string   `yaml:"file,omitempty"`
}

// ConfigGenerator generates Hysteria2 configuration files
type ConfigGenerator struct {
	configDir string
}

// NewConfigGenerator creates a new config generator
func NewConfigGenerator() *ConfigGenerator {
	return &ConfigGenerator{
		configDir: DefaultHy2ConfigDir,
	}
}

// SetConfigDir sets the configuration directory
func (g *ConfigGenerator) SetConfigDir(dir string) {
	g.configDir = dir
}

// GenerateConfig generates a Hysteria2 configuration file for a node
func (g *ConfigGenerator) GenerateConfig(tag string, nodeInfo *panel.NodeInfo, option *conf.Options, users []panel.UserInfo) (string, error) {
	// Ensure config directory exists
	if err := os.MkdirAll(g.configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config dir: %v", err)
	}

	// Build config
	port := 443 // Default port
	if nodeInfo.Common != nil {
		port = nodeInfo.Common.ServerPort
	}
	config := &Hy2ServerConfig{
		Listen: fmt.Sprintf(":%d", port),
		Auth: &Hy2AuthConfig{
			Type:     "userpass",
			UserPass: make(map[string]string),
		},
		Stats: &Hy2StatsConfig{
			Listen: fmt.Sprintf("127.0.0.1:%d", 25590+nodeInfo.Id%1000), // Unique port per node
		},
		// Default outbounds
		Outbounds: []Hy2Outbound{
			{
				Name: "direct",
				Type: "direct",
				Direct: &Hy2DirectConfig{
					Mode: "auto",
				},
			},
		},
		// Default ACL - block private IPs (same as Xray route.json)
		ACL: &Hy2ACLConfig{
			Inline: []string{
				"reject(geoip:private)",  // Block private IP ranges
				"reject(127.0.0.0/8)",    // Block loopback
				"reject(10.0.0.0/8)",     // Block Class A private
				"reject(172.16.0.0/12)",  // Block Class B private
				"reject(192.168.0.0/16)", // Block Class C private
				"reject(fc00::/7)",       // Block IPv6 ULA
				"reject(fe80::/10)",      // Block IPv6 link-local
				"direct(all)",            // Allow everything else
			},
		},
	}

	// Add TLS config if certificate is configured
	if option.CertConfig != nil && option.CertConfig.CertFile != "" {
		config.TLS = &Hy2TLSConfig{
			Cert: option.CertConfig.CertFile,
			Key:  option.CertConfig.KeyFile,
		}
	}

	// Add users (password = uuid)
	for _, user := range users {
		config.Auth.UserPass[user.Uuid] = user.Uuid
	}

	// Generate YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %v", err)
	}

	// Write config file (sanitize tag for safe filename)
	safeTag := sanitizeFilename(tag)
	configPath := filepath.Join(g.configDir, fmt.Sprintf("%s.yaml", safeTag))
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write config: %v", err)
	}

	return configPath, nil
}

// UpdateUsers updates the users in a configuration file
func (g *ConfigGenerator) UpdateUsers(tag string, users []panel.UserInfo) error {
	safeTag := sanitizeFilename(tag)
	configPath := filepath.Join(g.configDir, fmt.Sprintf("%s.yaml", safeTag))

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}

	var config Hy2ServerConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	// Update users
	if config.Auth == nil {
		config.Auth = &Hy2AuthConfig{
			Type:     "userpass",
			UserPass: make(map[string]string),
		}
	}
	config.Auth.UserPass = make(map[string]string)
	for _, user := range users {
		config.Auth.UserPass[user.Uuid] = user.Uuid
	}

	// Write back
	newData, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

// GetStatsAddress returns the traffic stats API address for a node
func (g *ConfigGenerator) GetStatsAddress(nodeID int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", 25590+nodeID%1000)
}

// DeleteConfig removes a configuration file
func (g *ConfigGenerator) DeleteConfig(tag string) error {
	safeTag := sanitizeFilename(tag)
	configPath := filepath.Join(g.configDir, fmt.Sprintf("%s.yaml", safeTag))
	return os.Remove(configPath)
}
