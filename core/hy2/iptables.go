package hy2

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	// iptablesComment is used to identify v2sp-created rules
	iptablesComment = "v2sp-hy2"
)

// PortHoppingConfig represents port hopping configuration
type PortHoppingConfig struct {
	Enabled   bool
	StartPort int
	EndPort   int
	ListenPort int // The actual port Hysteria2 listens on
}

// IPTablesManager manages iptables rules for port hopping
type IPTablesManager struct {
	rules []iptablesRule
}

type iptablesRule struct {
	ipVersion  int    // 4 or 6
	chain      string // PREROUTING
	protocol   string // udp
	startPort  int
	endPort    int
	targetPort int
	comment    string
}

// NewIPTablesManager creates a new iptables manager
func NewIPTablesManager() *IPTablesManager {
	return &IPTablesManager{
		rules: make([]iptablesRule, 0),
	}
}

// CleanupAllRules removes all v2sp-created iptables rules
func (m *IPTablesManager) CleanupAllRules() error {
	log.Info("Cleaning up all v2sp port hopping iptables rules...")

	// Clean both IPv4 and IPv6 rules
	for _, ipv := range []int{4, 6} {
		if err := m.cleanupRulesForVersion(ipv); err != nil {
			log.Warnf("Failed to cleanup IPv%d rules: %v", ipv, err)
		}
	}

	m.rules = make([]iptablesRule, 0)
	return nil
}

// cleanupRulesForVersion removes rules for a specific IP version
func (m *IPTablesManager) cleanupRulesForVersion(ipVersion int) error {
	cmd := "iptables"
	if ipVersion == 6 {
		cmd = "ip6tables"
	}

	// List all rules in nat PREROUTING chain with line numbers
	output, err := exec.Command(cmd, "-t", "nat", "-L", "PREROUTING", "-n", "--line-numbers").Output()
	if err != nil {
		return fmt.Errorf("failed to list rules: %v", err)
	}

	// Find rules with our comment and delete them (in reverse order to preserve line numbers)
	lines := strings.Split(string(output), "\n")
	rulesToDelete := make([]int, 0)

	for _, line := range lines {
		if strings.Contains(line, iptablesComment) {
			// Extract line number (first field)
			fields := strings.Fields(line)
			if len(fields) > 0 {
				if lineNum, err := strconv.Atoi(fields[0]); err == nil {
					rulesToDelete = append(rulesToDelete, lineNum)
				}
			}
		}
	}

	// Delete in reverse order
	for i := len(rulesToDelete) - 1; i >= 0; i-- {
		lineNum := rulesToDelete[i]
		if err := exec.Command(cmd, "-t", "nat", "-D", "PREROUTING", strconv.Itoa(lineNum)).Run(); err != nil {
			log.Warnf("Failed to delete IPv%d rule at line %d: %v", ipVersion, lineNum, err)
		} else {
			log.Debugf("Deleted IPv%d rule at line %d", ipVersion, lineNum)
		}
	}

	if len(rulesToDelete) > 0 {
		log.Infof("Cleaned up %d IPv%d port hopping rules", len(rulesToDelete), ipVersion)
	}

	return nil
}

// AddPortHopping adds iptables rules for port hopping
func (m *IPTablesManager) AddPortHopping(nodeTag string, config *PortHoppingConfig) error {
	if !config.Enabled || config.StartPort <= 0 || config.EndPort <= 0 {
		return nil // Port hopping not enabled
	}

	if config.StartPort > config.EndPort {
		return fmt.Errorf("invalid port range: %d-%d", config.StartPort, config.EndPort)
	}

	comment := fmt.Sprintf("%s:%s", iptablesComment, nodeTag)
	portRange := fmt.Sprintf("%d:%d", config.StartPort, config.EndPort)

	log.Infof("Adding port hopping rules for %s: UDP %s -> %d", nodeTag, portRange, config.ListenPort)

	// Add rules for both IPv4 and IPv6
	for _, ipv := range []int{4, 6} {
		if err := m.addRuleForVersion(ipv, portRange, config.ListenPort, comment); err != nil {
			log.Warnf("Failed to add IPv%d rule: %v", ipv, err)
			// Continue - IPv6 might not be available
		} else {
			m.rules = append(m.rules, iptablesRule{
				ipVersion:  ipv,
				chain:      "PREROUTING",
				protocol:   "udp",
				startPort:  config.StartPort,
				endPort:    config.EndPort,
				targetPort: config.ListenPort,
				comment:    comment,
			})
		}
	}

	return nil
}

// addRuleForVersion adds a single iptables rule
func (m *IPTablesManager) addRuleForVersion(ipVersion int, portRange string, targetPort int, comment string) error {
	cmd := "iptables"
	if ipVersion == 6 {
		cmd = "ip6tables"
	}

	// Check if rule already exists
	checkArgs := []string{
		"-t", "nat", "-C", "PREROUTING",
		"-p", "udp",
		"--dport", portRange,
		"-j", "REDIRECT",
		"--to-ports", strconv.Itoa(targetPort),
		"-m", "comment", "--comment", comment,
	}

	if err := exec.Command(cmd, checkArgs...).Run(); err == nil {
		log.Debugf("IPv%d rule already exists, skipping", ipVersion)
		return nil // Rule already exists
	}

	// Add the rule
	addArgs := []string{
		"-t", "nat", "-A", "PREROUTING",
		"-p", "udp",
		"--dport", portRange,
		"-j", "REDIRECT",
		"--to-ports", strconv.Itoa(targetPort),
		"-m", "comment", "--comment", comment,
	}

	if output, err := exec.Command(cmd, addArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add rule: %v, output: %s", err, string(output))
	}

	log.Debugf("Added IPv%d port hopping rule: %s -> %d", ipVersion, portRange, targetPort)
	return nil
}

// RemovePortHopping removes iptables rules for a specific node
func (m *IPTablesManager) RemovePortHopping(nodeTag string) error {
	comment := fmt.Sprintf("%s:%s", iptablesComment, nodeTag)

	log.Infof("Removing port hopping rules for node: %s", nodeTag)

	// Remove rules for both IPv4 and IPv6
	for _, ipv := range []int{4, 6} {
		if err := m.removeRuleByComment(ipv, comment); err != nil {
			log.Warnf("Failed to remove IPv%d rule for %s: %v", ipv, nodeTag, err)
		}
	}

	// Update internal rules list
	newRules := make([]iptablesRule, 0)
	for _, rule := range m.rules {
		if rule.comment != comment {
			newRules = append(newRules, rule)
		}
	}
	m.rules = newRules

	return nil
}

// removeRuleByComment removes rules matching a specific comment
func (m *IPTablesManager) removeRuleByComment(ipVersion int, comment string) error {
	cmd := "iptables"
	if ipVersion == 6 {
		cmd = "ip6tables"
	}

	// List all rules
	output, err := exec.Command(cmd, "-t", "nat", "-L", "PREROUTING", "-n", "--line-numbers").Output()
	if err != nil {
		return fmt.Errorf("failed to list rules: %v", err)
	}

	// Find matching rules
	lines := strings.Split(string(output), "\n")
	rulesToDelete := make([]int, 0)

	for _, line := range lines {
		if strings.Contains(line, comment) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				if lineNum, err := strconv.Atoi(fields[0]); err == nil {
					rulesToDelete = append(rulesToDelete, lineNum)
				}
			}
		}
	}

	// Delete in reverse order
	for i := len(rulesToDelete) - 1; i >= 0; i-- {
		lineNum := rulesToDelete[i]
		exec.Command(cmd, "-t", "nat", "-D", "PREROUTING", strconv.Itoa(lineNum)).Run()
	}

	return nil
}

// HasCapability checks if the system supports iptables NAT
func HasIPTablesCapability() bool {
	// Check if iptables is available
	if _, err := exec.LookPath("iptables"); err != nil {
		return false
	}

	// Check if we can access nat table
	if err := exec.Command("iptables", "-t", "nat", "-L", "-n").Run(); err != nil {
		return false
	}

	return true
}

