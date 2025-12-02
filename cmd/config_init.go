package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nsevo/v2sp/cmd/ui"
	"github.com/spf13/cobra"
)

var (
	interactiveMode bool
)

var configCommand = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	Long:  "Manage v2sp configuration",
}

var configInitCommand = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration",
	Long:  "Interactive configuration wizard with API integration",
	Run:   configInitHandle,
	Args:  cobra.NoArgs,
}

var configValidateCommand = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	Long:  "Check if configuration file is valid",
	Run:   configValidateHandle,
	Args:  cobra.NoArgs,
}

var configShowCommand = &cobra.Command{
	Use:   "show",
	Short: "Show configuration",
	Long:  "Display current configuration (sensitive data hidden)",
	Run:   configShowHandle,
	Args:  cobra.NoArgs,
}

func init() {
	configInitCommand.Flags().BoolVarP(&interactiveMode, "interactive", "i", true, "Interactive mode")

	configCommand.AddCommand(configInitCommand)
	configCommand.AddCommand(configValidateCommand)
	configCommand.AddCommand(configShowCommand)
	command.AddCommand(configCommand)
}

// NodeInfo 节点信息
type NodeInfo struct {
	NodeID     int
	NodeType   string
	ServerName string
	TLS        int
	CertReq    string // required, optional, none, reality
}

// TLS 需求类型
const (
	CertRequired = "required"
	CertOptional = "optional"
	CertNone     = "none"
	CertReality  = "reality"
)

func configInitHandle(_ *cobra.Command, _ []string) {
	fmt.Print(ui.Header("Configuration Wizard", "Let's set up your v2sp configuration"))

	reader := bufio.NewReader(os.Stdin)

	// 显示使用说明
	fmt.Println(ui.Section(
		ui.DimStyle.Render("Input format: URL KEY NODE_ID [NODE_ID ...]"),
		ui.DimStyle.Render("Example: https://panel.com/api mykey123 209 210"),
	))
	fmt.Println()

	// 读取输入
	fmt.Println(ui.DimStyle.Render("Enter configuration (URL KEY NODE_IDs):"))
	fmt.Print("  › ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	// 解析输入
	parts := strings.Fields(input)
	if len(parts) < 3 {
		fmt.Println()
		fmt.Println(ui.Error("Invalid input format"))
		fmt.Println(ui.DimStyle.Render("  Usage: URL KEY NODE_ID [NODE_ID ...]"))
		return
	}

	apiURL := parts[0]
	apiKey := parts[1]
	nodeIDs := []int{}

	for i := 2; i < len(parts); i++ {
		id, err := strconv.Atoi(parts[i])
		if err != nil {
			fmt.Println(ui.Warning(fmt.Sprintf("Skipping invalid node ID: %s", parts[i])))
			continue
		}
		nodeIDs = append(nodeIDs, id)
	}

	if len(nodeIDs) == 0 {
		fmt.Println(ui.Error("No valid node IDs provided"))
		return
	}

	fmt.Println()
	fmt.Println(ui.Divider(60))
	fmt.Println()

	// 从 API 获取节点信息
	fmt.Println(ui.Info("Fetching node information from API..."))
	fmt.Println()

	nodes := []NodeInfo{}
	for _, id := range nodeIDs {
		fmt.Print(ui.DimStyle.Render(fmt.Sprintf("  • Node %d... ", id)))

		info, err := fetchNodeInfo(apiURL, apiKey, id)
		if err != nil {
			fmt.Println(ui.Warning("Failed (will use defaults)"))
			// 使用默认配置
			nodes = append(nodes, NodeInfo{
				NodeID:   id,
				NodeType: "unknown",
				CertReq:  CertNone,
			})
		} else {
			nodes = append(nodes, *info)

			// 显示节点信息
			var statusText string
			switch info.CertReq {
			case CertRequired:
				if info.ServerName != "" {
					statusText = fmt.Sprintf("%s → %s (TLS required)",
						ui.SuccessStyle.Render(info.NodeType),
						ui.InfoStyle.Render(info.ServerName))
				} else {
					statusText = fmt.Sprintf("%s (TLS required, %s)",
						ui.SuccessStyle.Render(info.NodeType),
						ui.WarningStyle.Render("no domain"))
				}
			case CertReality:
				statusText = fmt.Sprintf("%s (Reality, no cert needed)",
					ui.SuccessStyle.Render(info.NodeType))
			case CertNone:
				statusText = fmt.Sprintf("%s (no TLS)",
					ui.SuccessStyle.Render(info.NodeType))
			default:
				statusText = ui.SuccessStyle.Render(info.NodeType)
			}
			fmt.Println(statusText)
		}
	}

	fmt.Println()
	fmt.Println(ui.Divider(60))
	fmt.Println()

	// 显示配置摘要
	fmt.Println(ui.Section(
		ui.DimStyle.Render("Configuration Summary:"),
		"",
		ui.KeyValue("API URL", apiURL),
		ui.KeyValue("API Key", maskKey(apiKey)),
		ui.KeyValue("Nodes", fmt.Sprintf("%v", nodeIDs)),
	))
	fmt.Println()

	// 检查是否有需要 TLS 的节点
	hasTLSNodes := false
	tlsDomains := []string{}
	for _, node := range nodes {
		if node.CertReq == CertRequired && node.ServerName != "" {
			hasTLSNodes = true
			tlsDomains = append(tlsDomains, node.ServerName)
		}
	}

	// 询问证书模式
	var autoCert bool
	if hasTLSNodes {
		fmt.Println(ui.Warning("TLS certificates required for some nodes"))
		fmt.Println()
		fmt.Println(ui.DimStyle.Render("  Domains: " + strings.Join(tlsDomains, ", ")))
		fmt.Println()
		fmt.Print(ui.DimStyle.Render("Enable auto-certificate (HTTP-01)? [Y/n]: "))
		answer, _ := reader.ReadString('\n')
		autoCert = !strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "n")
	}

	fmt.Println()

	// 确认生成
	fmt.Print(ui.DimStyle.Render("Generate configuration? [Y/n]: "))
	confirm, _ := reader.ReadString('\n')
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(confirm)), "n") {
		fmt.Println()
		fmt.Println(ui.Warning("Cancelled"))
		return
	}

	fmt.Println()
	fmt.Println(ui.Info("Generating configuration..."))

	// 备份现有配置
	if _, err := os.Stat("/etc/v2sp/config.json"); err == nil {
		backupPath := fmt.Sprintf("/etc/v2sp/config.json.bak.%d", time.Now().Unix())
		if err := copyFile("/etc/v2sp/config.json", backupPath); err == nil {
			fmt.Println(ui.Success("Backed up existing config to " + backupPath))
		}
	}

	// 生成配置
	config := generateConfig(apiURL, apiKey, nodes, autoCert)

	// 确保目录存在
	os.MkdirAll("/etc/v2sp", 0755)
	os.MkdirAll("/etc/v2sp/cert", 0755)

	// 保存配置
	configJSON, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		fmt.Println(ui.Error("Failed to generate configuration"))
		fmt.Println(ui.DimStyle.Render("  " + err.Error()))
		return
	}

	err = os.WriteFile("/etc/v2sp/config.json", configJSON, 0644)
	if err != nil {
		fmt.Println(ui.Error("Failed to write configuration file"))
		fmt.Println(ui.DimStyle.Render("  " + err.Error()))
		return
	}

	fmt.Println(ui.Success("Configuration saved to /etc/v2sp/config.json"))
	fmt.Println()
	fmt.Println(ui.Divider(60))
	fmt.Println()
	fmt.Println(ui.DimStyle.Render("  Next steps:"))
	fmt.Println(ui.DimStyle.Render("  1. Review config: v2sp config show"))
	fmt.Println(ui.DimStyle.Render("  2. Start service: v2sp start"))
	fmt.Println()
}

// fetchNodeInfo 从 API 获取节点信息
func fetchNodeInfo(apiURL, apiKey string, nodeID int) (*NodeInfo, error) {
	// 构建 API URL
	url := fmt.Sprintf("%s?action=config&node_id=%d&token=%s", apiURL, nodeID, apiKey)

	// 发起请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析 JSON
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	// 提取字段
	info := &NodeInfo{
		NodeID: nodeID,
	}

	if nodeType, ok := data["node_type"].(string); ok {
		info.NodeType = nodeType
	}
	if serverName, ok := data["server_name"].(string); ok {
		info.ServerName = serverName
	}
	if tls, ok := data["tls"].(float64); ok {
		info.TLS = int(tls)
	}

	// 判断 TLS 需求
	info.CertReq = checkTLSRequirement(info.NodeType, info.TLS)

	return info, nil
}

// checkTLSRequirement 检查 TLS 证书需求
func checkTLSRequirement(nodeType string, tls int) string {
	switch strings.ToLower(nodeType) {
	case "trojan", "hysteria":
		// 这些协议总是需要 TLS
		return CertRequired
	case "shadowsocks":
		// Shadowsocks 不需要 TLS
		return CertNone
	case "vmess", "vless":
		// 根据 tls 字段判断
		switch tls {
		case 1:
			return CertRequired // 普通 TLS
		case 2:
			return CertReality // Reality (不需要证书)
		default:
			return CertNone // 无 TLS
		}
	default:
		return CertNone
	}
}

// generateConfig 生成配置文件
func generateConfig(apiURL, apiKey string, nodes []NodeInfo, autoCert bool) map[string]interface{} {
	// 构建 Nodes 配置
	nodeConfigs := []map[string]interface{}{}
	for _, node := range nodes {
		certMode := "file"
		certDomain := node.ServerName

		// 根据节点需求确定证书模式
		switch node.CertReq {
		case CertRequired:
			if autoCert && certDomain != "" {
				certMode = "http"
			} else {
				certMode = "file"
			}
		case CertReality, CertNone:
			certMode = "none"
		}

		nodeConfig := map[string]interface{}{
			"ApiHost":                apiURL,
			"ApiKey":                 apiKey,
			"NodeID":                 node.NodeID,
			"Timeout":                30,
			"ListenIP":               "0.0.0.0",
			"SendIP":                 "0.0.0.0",
			"DeviceOnlineMinTraffic": 200,
			"CertConfig": map[string]interface{}{
				"CertMode":   certMode,
				"CertDomain": certDomain,
				"CertFile":   fmt.Sprintf("/etc/v2sp/cert/%s.crt", certDomain),
				"KeyFile":    fmt.Sprintf("/etc/v2sp/cert/%s.key", certDomain),
			},
		}

		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// 构建完整配置
	config := map[string]interface{}{
		"Log": map[string]interface{}{
			"Level":  "error",
			"Output": "",
		},
		"Cores": []map[string]interface{}{
			{
				"Type": "xray",
				"Log": map[string]interface{}{
					"Level":      "error",
					"AccessPath": "none",
					"ErrorPath":  "/etc/v2sp/error.log",
				},
				"AssetPath":          "/etc/v2sp/",
				"OutboundConfigPath": "/etc/v2sp/custom_outbound.json",
				"RouteConfigPath":    "/etc/v2sp/route.json",
			},
		},
		"Nodes": nodeConfigs,
	}

	return config
}

// maskKey 隐藏部分密钥
func maskKey(key string) string {
	if len(key) <= 8 {
		return "********"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}

func configValidateHandle(_ *cobra.Command, _ []string) {
	fmt.Print(ui.Header("Configuration Validation", ""))

	configPath := "/etc/v2sp/config.json"

	// 检查文件存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println(ui.Error("Configuration file not found: " + configPath))
		fmt.Println()
		fmt.Println(ui.DimStyle.Render("  Generate config: v2sp config init"))
		fmt.Println()
		return
	}

	// 读取文件
	content, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Println(ui.Error("Failed to read configuration file"))
		fmt.Println(ui.DimStyle.Render("  " + err.Error()))
		return
	}

	// 验证 JSON
	var config map[string]interface{}
	err = json.Unmarshal(content, &config)
	if err != nil {
		fmt.Println(ui.Error("Invalid JSON format"))
		fmt.Println(ui.DimStyle.Render("  " + err.Error()))
		return
	}

	// 基本验证
	errors := []string{}
	warnings := []string{}

	// 检查必要字段
	if _, ok := config["Cores"]; !ok {
		errors = append(errors, "Missing 'Cores' section")
	}
	if nodes, ok := config["Nodes"]; !ok {
		errors = append(errors, "Missing 'Nodes' section")
	} else if nodeList, ok := nodes.([]interface{}); ok {
		if len(nodeList) == 0 {
			warnings = append(warnings, "No nodes configured")
		}
	}

	// 显示结果
	if len(errors) > 0 {
		fmt.Println(ui.Error(fmt.Sprintf("Found %d error(s)", len(errors))))
		fmt.Println()
		for _, e := range errors {
			fmt.Println(ui.DimStyle.Render("  • " + e))
		}
	} else {
		fmt.Println(ui.Success("Configuration is valid"))
	}

	if len(warnings) > 0 {
		fmt.Println()
		fmt.Println(ui.Warning(fmt.Sprintf("Found %d warning(s)", len(warnings))))
		fmt.Println()
		for _, w := range warnings {
			fmt.Println(ui.DimStyle.Render("  • " + w))
		}
	}

	fmt.Println()
	fmt.Println(ui.Divider(60))
	fmt.Println()

	if len(errors) == 0 {
		fmt.Println(ui.DimStyle.Render("  Configuration looks good!"))
	} else {
		fmt.Println(ui.DimStyle.Render("  Fix errors: v2sp config init"))
	}
	fmt.Println()
}

func configShowHandle(_ *cobra.Command, _ []string) {
	fmt.Print(ui.Header("Current Configuration", ""))

	configPath := "/etc/v2sp/config.json"

	content, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Println(ui.Error("Failed to read configuration"))
		return
	}

	var config map[string]interface{}
	err = json.Unmarshal(content, &config)
	if err != nil {
		fmt.Println(ui.Error("Invalid configuration format"))
		return
	}

	// 隐藏敏感信息
	if nodes, ok := config["Nodes"].([]interface{}); ok {
		for _, node := range nodes {
			if nodeMap, ok := node.(map[string]interface{}); ok {
				if key, ok := nodeMap["ApiKey"].(string); ok {
					nodeMap["ApiKey"] = maskKey(key)
				}
			}
		}
	}

	// 格式化输出
	configJSON, _ := json.MarshalIndent(config, "", "  ")
	fmt.Println(ui.DimStyle.Render(string(configJSON)))
	fmt.Println()
	fmt.Println(ui.Divider(60))
	fmt.Println()
	fmt.Println(ui.DimStyle.Render("  Edit config: vi /etc/v2sp/config.json"))
	fmt.Println(ui.DimStyle.Render("  Full path: " + configPath))
	fmt.Println()
}
