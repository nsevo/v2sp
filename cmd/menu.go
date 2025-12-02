package cmd

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nsevo/v2sp/cmd/ui"
	"github.com/nsevo/v2sp/common/exec"
	"github.com/spf13/cobra"
)

var menuCommand = &cobra.Command{
	Use:   "menu",
	Short: "Interactive menu (TUI)",
	Long:  "Launch interactive terminal UI for managing v2sp",
	Run:   menuHandle,
	Args:  cobra.NoArgs,
}

func init() {
	command.AddCommand(menuCommand)
}

func menuHandle(_ *cobra.Command, _ []string) {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println(ui.Error("Failed to start menu"))
		os.Exit(1)
	}
}

// 菜单项定义
type MenuItem struct {
	Key         string
	Title       string
	Description string
	Action      func() tea.Msg
	Condition   func() bool // 是否显示此项
}

type model struct {
	items       []MenuItem
	cursor      int
	selected    int
	status      string
	message     string
	messageType string // success, error, warning, info
	loading     bool
	spinner     *ui.Spinner
}

func initialModel() model {
	return model{
		items:   getMenuItems(),
		cursor:  0,
		spinner: ui.NewSpinner(),
	}
}

func getMenuItems() []MenuItem {
	return []MenuItem{
		{
			Key:         "1",
			Title:       "Start Service",
			Description: "Start v2sp service",
			Action:      startServiceAction,
			Condition: func() bool {
				running, _ := checkRunning()
				return !running
			},
		},
		{
			Key:         "2",
			Title:       "Stop Service",
			Description: "Stop v2sp service",
			Action:      stopServiceAction,
			Condition: func() bool {
				running, _ := checkRunning()
				return running
			},
		},
		{
			Key:         "3",
			Title:       "Restart Service",
			Description: "Restart v2sp service",
			Action:      restartServiceAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "4",
			Title:       "View Status",
			Description: "Show detailed status information",
			Action:      statusAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "5",
			Title:       "View Logs",
			Description: "Show service logs",
			Action:      logsAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "6",
			Title:       "Edit Configuration",
			Description: "Edit config.json with $EDITOR",
			Action:      editConfigAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "7",
			Title:       "Generate Config",
			Description: "Interactive configuration wizard",
			Action:      generateConfigAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "8",
			Title:       "Health Check",
			Description: "Run system health checks",
			Action:      healthCheckAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "9",
			Title:       "Generate X25519 Key",
			Description: "Generate encryption keys",
			Action:      x25519Action,
			Condition:   func() bool { return true },
		},
		{
			Key:         "U",
			Title:       "Update v2sp",
			Description: "Update to latest version",
			Action:      updateAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "Q",
			Title:       "Quit",
			Description: "Exit menu",
			Action:      func() tea.Msg { return tea.Quit() },
			Condition:   func() bool { return true },
		},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			visibleItems := m.getVisibleItems()
			if m.cursor < len(visibleItems)-1 {
				m.cursor++
			}

		case "enter":
			visibleItems := m.getVisibleItems()
			if m.cursor < len(visibleItems) {
				item := visibleItems[m.cursor]
				if item.Action != nil {
					// 执行操作
					result := item.Action()
					if result != nil {
						// 处理返回消息
						if quitMsg, ok := result.(tea.QuitMsg); ok {
							return m, func() tea.Msg { return quitMsg }
						}
					}
					// 刷新菜单项
					m.items = getMenuItems()
				}
			}

		default:
			// 检查快捷键
			visibleItems := m.getVisibleItems()
			for _, item := range visibleItems {
				if strings.ToLower(msg.String()) == strings.ToLower(item.Key) {
					if item.Action != nil {
						result := item.Action()
						if result != nil {
							if quitMsg, ok := result.(tea.QuitMsg); ok {
								return m, func() tea.Msg { return quitMsg }
							}
						}
						m.items = getMenuItems()
					}
					break
				}
			}
		}
	}

	return m, nil
}

func (m model) getVisibleItems() []MenuItem {
	var visible []MenuItem
	for _, item := range m.items {
		if item.Condition == nil || item.Condition() {
			visible = append(visible, item)
		}
	}
	return visible
}

func (m model) View() string {
	var s strings.Builder

	// 获取服务状态
	running, _ := checkRunning()
	var statusLine string
	if running {
		statusLine = ui.StatusLine("running", "Service is running")
	} else {
		statusLine = ui.StatusLine("stopped", "Service is stopped")
	}

	// 头部
	s.WriteString(ui.Header("v2sp Control Panel", ""))
	s.WriteString(ui.Section(statusLine))
	s.WriteString("\n\n")

	// 菜单项
	visibleItems := m.getVisibleItems()
	for i, item := range visibleItems {
		cursor := " "
		if m.cursor == i {
			cursor = ui.InfoStyle.Render(">")
		}

		keyStyle := ui.KeyStyle
		titleStyle := ui.TextStyle
		if m.cursor == i {
			titleStyle = ui.SuccessStyle
		}

		line := fmt.Sprintf("%s %s %-20s %s",
			cursor,
			keyStyle.Render("["+item.Key+"]"),
			titleStyle.Render(item.Title),
			ui.DimStyle.Render(item.Description),
		)
		s.WriteString("  " + line + "\n")
	}

	// 底部帮助
	s.WriteString(ui.Footer(
		ui.DimStyle.Render("↑↓ Navigate"),
		ui.DimStyle.Render("Enter Select"),
		ui.DimStyle.Render("Number/Letter Quick action"),
		ui.Key("Q", "Quit"),
	))

	return s.String()
}

// 操作函数

func startServiceAction() tea.Msg {
	fmt.Println()
	fmt.Println(ui.Info("Starting v2sp..."))
	_, err := exec.RunCommandByShell("systemctl start v2sp")
	if err != nil {
		fmt.Println(ui.Error("Failed to start service"))
		fmt.Println(ui.DimStyle.Render("  " + err.Error()))
	} else {
		fmt.Println(ui.Success("Service started successfully"))
	}
	fmt.Println()
	fmt.Print(ui.DimStyle.Render("Press Enter to continue..."))
	fmt.Scanln()
	return nil
}

func stopServiceAction() tea.Msg {
	fmt.Println()
	fmt.Println(ui.Info("Stopping v2sp..."))
	_, err := exec.RunCommandByShell("systemctl stop v2sp")
	if err != nil {
		fmt.Println(ui.Error("Failed to stop service"))
		fmt.Println(ui.DimStyle.Render("  " + err.Error()))
	} else {
		fmt.Println(ui.Success("Service stopped successfully"))
	}
	fmt.Println()
	fmt.Print(ui.DimStyle.Render("Press Enter to continue..."))
	fmt.Scanln()
	return nil
}

func restartServiceAction() tea.Msg {
	fmt.Println()
	fmt.Println(ui.Info("Restarting v2sp..."))
	_, err := exec.RunCommandByShell("systemctl restart v2sp")
	if err != nil {
		fmt.Println(ui.Error("Failed to restart service"))
		fmt.Println(ui.DimStyle.Render("  " + err.Error()))
	} else {
		fmt.Println(ui.Success("Service restarted successfully"))
	}
	fmt.Println()
	fmt.Print(ui.DimStyle.Render("Press Enter to continue..."))
	fmt.Scanln()
	return nil
}

func statusAction() tea.Msg {
	fmt.Println()
	showStatus()
	fmt.Println()
	fmt.Print(ui.DimStyle.Render("Press Enter to continue..."))
	fmt.Scanln()
	return nil
}

func logsAction() tea.Msg {
	fmt.Println()
	fmt.Println(ui.Info("Opening logs (Ctrl+C to exit)..."))
	fmt.Println()
	exec.RunCommandStd("journalctl", "-u", "v2sp", "-f", "-n", "50")
	return nil
}

func editConfigAction() tea.Msg {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	fmt.Println()
	fmt.Println(ui.Info("Opening config in " + editor + "..."))
	exec.RunCommandStd(editor, "/etc/v2sp/config.json")
	fmt.Println()
	fmt.Println(ui.Warning("Remember to restart service after editing config"))
	fmt.Println()
	fmt.Print(ui.DimStyle.Render("Press Enter to continue..."))
	fmt.Scanln()
	return nil
}

func generateConfigAction() tea.Msg {
	fmt.Println()
	fmt.Println(ui.Info("Launching configuration wizard..."))
	fmt.Println()
	// TODO: 实现配置向导
	fmt.Println(ui.Warning("Configuration wizard not yet implemented"))
	fmt.Println(ui.DimStyle.Render("  Use: v2sp config init"))
	fmt.Println()
	fmt.Print(ui.DimStyle.Render("Press Enter to continue..."))
	fmt.Scanln()
	return nil
}

func healthCheckAction() tea.Msg {
	fmt.Println()
	healthHandle(nil, nil)
	fmt.Println()
	fmt.Print(ui.DimStyle.Render("Press Enter to continue..."))
	fmt.Scanln()
	return nil
}

func x25519Action() tea.Msg {
	fmt.Println()
	executeX25519()
	fmt.Println()
	fmt.Print(ui.DimStyle.Render("Press Enter to continue..."))
	fmt.Scanln()
	return nil
}

func updateAction() tea.Msg {
	fmt.Println()
	fmt.Println(ui.Warning("This will update v2sp to the latest version"))
	fmt.Print(ui.DimStyle.Render("Continue? (y/N): "))
	var answer string
	fmt.Scanln(&answer)
	if strings.ToLower(answer) != "y" {
		fmt.Println(ui.Info("Update cancelled"))
		fmt.Println()
		fmt.Print(ui.DimStyle.Render("Press Enter to continue..."))
		fmt.Scanln()
		return nil
	}

	fmt.Println()
	fmt.Println(ui.Info("Downloading latest version..."))
	// TODO: 实现更新逻辑
	fmt.Println(ui.Warning("Update not yet implemented"))
	fmt.Println()
	fmt.Print(ui.DimStyle.Render("Press Enter to continue..."))
	fmt.Scanln()
	return nil
}

