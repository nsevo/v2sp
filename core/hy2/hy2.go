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
	access    sync.Mutex
	config    *conf.Hy2Config
	configGen *ConfigGenerator
	nodes     sync.Map // tag -> *Hy2Node
}

// New creates a new Hy2 core
func New(c *conf.CoreConfig) (vCore.Core, error) {
	// Check if Hysteria2 binary exists
	if !CheckBinaryExists() {
		log.Warn("Hysteria2 binary not found at " + DefaultHy2Binary + ". Please install it first.")
	}

	return &Hy2{
		config:    c.Hy2Config,
		configGen: NewConfigGenerator(),
	}, nil
}

// Start starts the Hy2 core
func (h *Hy2) Start() error {
	log.Info("Hysteria2 Core (subprocess mode) started")

	// Check binary exists
	if !CheckBinaryExists() {
		log.Warn("Hysteria2 binary not found. Nodes will fail to start until installed.")
	} else {
		if version, err := GetBinaryVersion(); err == nil {
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
	if !CheckBinaryExists() {
		return fmt.Errorf("hysteria2 binary not found at %s", DefaultHy2Binary)
	}

	// Create process manager
	process := NewProcess(tag)

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
