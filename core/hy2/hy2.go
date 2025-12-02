package hy2

import (
	"fmt"
	"sync"

	"github.com/nsevo/v2sp/api/panel"
	"github.com/nsevo/v2sp/conf"
	vCore "github.com/nsevo/v2sp/core"
	log "github.com/sirupsen/logrus"
)

var _ vCore.Core = (*Hy2)(nil)

func init() {
	vCore.RegisterCore("hysteria2", New)
}

// Hy2Node represents a single Hysteria2 node
type Hy2Node struct {
	Tag         string
	Process     *Process
	StatsClient *TrafficStatsClient
	NodeInfo    *panel.NodeInfo
	Options     *conf.Options
	Users       []panel.UserInfo
	UserMap     map[string]int // uuid -> uid
}

// Hy2 is the Hysteria2 core implementation using subprocess mode
type Hy2 struct {
	access     sync.Mutex
	config     *conf.Hy2Config
	configGen  *ConfigGenerator
	binaryPath string
	nodes      sync.Map // tag -> *Hy2Node
	iptables   *IPTablesManager
}

// New creates a new Hy2 core
func New(c *conf.CoreConfig) (vCore.Core, error) {
	cfg := c.Hy2Config
	if cfg == nil {
		cfg = conf.NewHy2Config()
	}

	// Use configured paths or defaults
	binaryPath := cfg.BinaryPath
	if binaryPath == "" {
		binaryPath = DefaultHy2Binary
	}

	configDir := cfg.ConfigDir
	if configDir == "" {
		configDir = DefaultHy2ConfigDir
	}

	// Check if Hysteria2 binary exists
	if !CheckBinaryExistsAt(binaryPath) {
		log.Warn("Hysteria2 binary not found at " + binaryPath + ". Please install it first.")
	}

	configGen := NewConfigGenerator()
	configGen.SetConfigDir(configDir)

	return &Hy2{
		config:     cfg,
		configGen:  configGen,
		binaryPath: binaryPath,
		iptables:   NewIPTablesManager(),
	}, nil
}

// Start starts the Hy2 core
func (h *Hy2) Start() error {
	log.Info("Hysteria2 Core (subprocess mode) started")

	// Clean up any stale iptables rules from previous runs
	if HasIPTablesCapability() {
		log.Info("Cleaning up stale port hopping rules...")
		h.iptables.CleanupAllRules()
	} else {
		log.Warn("iptables not available, port hopping will not work")
	}

	// Check binary exists
	if !CheckBinaryExistsAt(h.binaryPath) {
		log.Warn("Hysteria2 binary not found at " + h.binaryPath + ". Nodes will fail to start until installed.")
	} else {
		if version, err := GetBinaryVersionAt(h.binaryPath); err == nil {
			log.WithField("version", version).Info("Hysteria2 binary found")
		}
	}

	return nil
}

// Close closes the Hy2 core and all nodes
func (h *Hy2) Close() error {
	h.access.Lock()
	defer h.access.Unlock()

	var errs []error
	h.nodes.Range(func(key, value interface{}) bool {
		node := value.(*Hy2Node)
		if err := node.Process.Stop(); err != nil {
			errs = append(errs, err)
		}
		return true
	})

	// Cleanup all iptables rules
	if HasIPTablesCapability() {
		log.Info("Cleaning up port hopping iptables rules...")
		h.iptables.CleanupAllRules()
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing nodes: %v", errs)
	}

	log.Info("Hysteria2 Core closed")
	return nil
}

// AddNode adds a new node
func (h *Hy2) AddNode(tag string, info *panel.NodeInfo, option *conf.Options) error {
	h.access.Lock()
	defer h.access.Unlock()

	// Check if node already exists
	if _, ok := h.nodes.Load(tag); ok {
		return fmt.Errorf("node %s already exists", tag)
	}

	// Check binary
	if !CheckBinaryExistsAt(h.binaryPath) {
		return fmt.Errorf("hysteria2 binary not found at %s", h.binaryPath)
	}

	// Create process manager with configured binary path
	process := NewProcess(tag)
	process.SetBinaryPath(h.binaryPath)

	// Create node (users will be added later via AddUsers)
	node := &Hy2Node{
		Tag:      tag,
		Process:  process,
		NodeInfo: info,
		Options:  option,
		Users:    []panel.UserInfo{},
		UserMap:  make(map[string]int),
	}

	// Generate initial config (empty users, will be populated by AddUsers)
	configPath, err := h.configGen.GenerateConfig(tag, info, option, node.Users)
	if err != nil {
		return fmt.Errorf("failed to generate config: %v", err)
	}
	process.SetConfigPath(configPath)

	// Create stats client
	node.StatsClient = NewTrafficStatsClient(
		h.configGen.GetStatsAddress(info.Id),
		"", // No secret by default
	)

	// Store node (don't start yet, wait for users)
	h.nodes.Store(tag, node)

	port := 443
	if info.Common != nil {
		port = info.Common.ServerPort
	}

	// Setup port hopping iptables rules if enabled
	// Port hopping is detected by PortRange field (e.g., "20000-50000")
	if info.Hysteria2 != nil {
		enabled, startPort, endPort := info.Hysteria2.GetPortHoppingConfig()
		if enabled {
			phConfig := &PortHoppingConfig{
				Enabled:    true,
				StartPort:  startPort,
				EndPort:    endPort,
				ListenPort: port,
			}
			if err := h.iptables.AddPortHopping(tag, phConfig); err != nil {
				log.WithFields(log.Fields{
					"tag":   tag,
					"range": fmt.Sprintf("%d-%d", startPort, endPort),
					"err":   err,
				}).Warn("Failed to setup port hopping, continuing without it")
			} else {
				log.WithFields(log.Fields{
					"tag":   tag,
					"range": fmt.Sprintf("%d-%d -> %d", startPort, endPort, port),
				}).Info("Port hopping enabled")
			}
		}
	}

	log.WithFields(log.Fields{
		"tag":  tag,
		"port": port,
	}).Info("Hysteria2 node added (waiting for users)")

	return nil
}

// DelNode removes a node
func (h *Hy2) DelNode(tag string) error {
	h.access.Lock()
	defer h.access.Unlock()

	value, ok := h.nodes.Load(tag)
	if !ok {
		return fmt.Errorf("node %s not found", tag)
	}

	node := value.(*Hy2Node)

	// Stop process
	if err := node.Process.Stop(); err != nil {
		log.WithField("tag", tag).WithError(err).Warn("Error stopping process")
	}

	// Remove port hopping iptables rules if enabled
	if node.NodeInfo.Hysteria2 != nil {
		enabled, _, _ := node.NodeInfo.Hysteria2.GetPortHoppingConfig()
		if enabled {
			if err := h.iptables.RemovePortHopping(tag); err != nil {
				log.WithField("tag", tag).WithError(err).Warn("Error removing port hopping rules")
			}
		}
	}

	// Delete config file
	if err := h.configGen.DeleteConfig(tag); err != nil {
		log.WithField("tag", tag).WithError(err).Warn("Error deleting config")
	}

	// Remove from map
	h.nodes.Delete(tag)

	log.WithField("tag", tag).Info("Hysteria2 node removed")
	return nil
}

// AddUsers adds users to a node
func (h *Hy2) AddUsers(p *vCore.AddUsersParams) (added int, err error) {
	value, ok := h.nodes.Load(p.Tag)
	if !ok {
		return 0, fmt.Errorf("node %s not found", p.Tag)
	}

	node := value.(*Hy2Node)

	// Add users to node
	for _, user := range p.Users {
		node.Users = append(node.Users, user)
		node.UserMap[user.Uuid] = user.Id
		added++
	}

	// Update config file
	if err := h.configGen.UpdateUsers(p.Tag, node.Users); err != nil {
		return 0, fmt.Errorf("failed to update config: %v", err)
	}

	// Start or restart process
	if node.Process.IsRunning() {
		// Restart to pick up new users
		if err := node.Process.Restart(); err != nil {
			return 0, fmt.Errorf("failed to restart process: %v", err)
		}
	} else {
		// Start process
		if err := node.Process.Start(); err != nil {
			return 0, fmt.Errorf("failed to start process: %v", err)
		}
	}

	log.WithFields(log.Fields{
		"tag":   p.Tag,
		"added": added,
		"total": len(node.Users),
	}).Debug("Hysteria2 users updated")

	return added, nil
}

// DelUsers removes users from a node
func (h *Hy2) DelUsers(users []panel.UserInfo, tag string, _ *panel.NodeInfo) error {
	value, ok := h.nodes.Load(tag)
	if !ok {
		return fmt.Errorf("node %s not found", tag)
	}

	node := value.(*Hy2Node)

	// Build set of UUIDs to delete
	deleteSet := make(map[string]bool)
	for _, user := range users {
		deleteSet[user.Uuid] = true
		delete(node.UserMap, user.Uuid)
	}

	// Filter out deleted users
	newUsers := make([]panel.UserInfo, 0, len(node.Users)-len(users))
	for _, user := range node.Users {
		if !deleteSet[user.Uuid] {
			newUsers = append(newUsers, user)
		}
	}
	node.Users = newUsers

	// Update config file
	if err := h.configGen.UpdateUsers(tag, node.Users); err != nil {
		return fmt.Errorf("failed to update config: %v", err)
	}

	// Restart process to pick up changes
	if node.Process.IsRunning() {
		if err := node.Process.Restart(); err != nil {
			return fmt.Errorf("failed to restart process: %v", err)
		}
	}

	log.WithFields(log.Fields{
		"tag":     tag,
		"deleted": len(users),
		"total":   len(node.Users),
	}).Debug("Hysteria2 users removed")

	return nil
}

// GetUserTrafficSlice returns traffic data for all users
func (h *Hy2) GetUserTrafficSlice(tag string, reset bool) ([]panel.UserTraffic, error) {
	value, ok := h.nodes.Load(tag)
	if !ok {
		return nil, fmt.Errorf("node %s not found", tag)
	}

	node := value.(*Hy2Node)

	// Don't query if process is not running
	if !node.Process.IsRunning() {
		return nil, nil
	}

	// Get traffic from stats API
	stats, err := node.StatsClient.GetTraffic(reset)
	if err != nil {
		log.WithField("tag", tag).WithError(err).Debug("Failed to get traffic stats")
		return nil, nil // Don't return error, just empty traffic
	}

	// Convert to v2sp format
	return ConvertToUserTraffic(stats, node.UserMap), nil
}

// Protocols returns supported protocols
func (h *Hy2) Protocols() []string {
	return []string{"hysteria2"}
}

// Type returns the core type
func (h *Hy2) Type() string {
	return "hysteria2"
}
