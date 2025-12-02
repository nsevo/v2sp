package cmd

import (
	"fmt"
	"os"
	osexec "os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nsevo/v2sp/cmd/ui"
	"github.com/nsevo/v2sp/common/exec"
	"github.com/spf13/cobra"
)

var menuCommand = &cobra.Command{
	Use:   "menu",
	Short: "Interactive menu",
	Run:   menuHandle,
	Args:  cobra.NoArgs,
}

func init() {
	command.AddCommand(menuCommand)
}

func menuHandle(_ *cobra.Command, _ []string) {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Failed to start menu:", err)
		os.Exit(1)
	}
}

type MenuItem struct {
	Key         string
	Title       string
	Description string
	Action      func() tea.Msg
	Condition   func() bool
}

type model struct {
	items  []MenuItem
	cursor int
}

func initialModel() model {
	return model{
		items:  getMenuItems(),
		cursor: 0,
	}
}

func getMenuItems() []MenuItem {
	return []MenuItem{
		{
			Key:         "1",
			Title:       "Start",
			Description: "systemctl start v2sp",
			Action:      startServiceAction,
			Condition: func() bool {
				running, _ := checkRunning()
				return !running
			},
		},
		{
			Key:         "2",
			Title:       "Stop",
			Description: "systemctl stop v2sp",
			Action:      stopServiceAction,
			Condition: func() bool {
				running, _ := checkRunning()
				return running
			},
		},
		{
			Key:         "3",
			Title:       "Restart",
			Description: "systemctl restart v2sp",
			Action:      restartServiceAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "4",
			Title:       "Status",
			Description: "v2sp status",
			Action:      statusAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "5",
			Title:       "Logs (follow)",
			Description: "v2sp logs -f",
			Action:      logsAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "6",
			Title:       "Logs (error)",
			Description: "v2sp logs -l error",
			Action:      logsErrorAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "7",
			Title:       "Health",
			Description: "v2sp health",
			Action:      healthCheckAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "8",
			Title:       "Config init",
			Description: "v2sp config init",
			Action:      configInitAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "9",
			Title:       "Config show",
			Description: "v2sp config show",
			Action:      configShowAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "0",
			Title:       "Config validate",
			Description: "v2sp config validate",
			Action:      configValidateAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "c",
			Title:       "Edit config",
			Description: "edit /etc/v2sp/config.json",
			Action:      editConfigAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "y",
			Title:       "System setup",
			Description: "v2sp system setup",
			Action:      systemSetupAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "t",
			Title:       "Sync time",
			Description: "v2sp synctime",
			Action:      syncTimeAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "k",
			Title:       "X25519 key",
			Description: "v2sp x25519",
			Action:      x25519Action,
			Condition:   func() bool { return true },
		},
		{
			Key:         "v",
			Title:       "Version",
			Description: "v2sp version",
			Action:      versionAction,
			Condition:   func() bool { return true },
		},
		{
			Key:         "q",
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
		case "ctrl+c", "q":
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
					result := item.Action()
					if result != nil {
						if quitMsg, ok := result.(tea.QuitMsg); ok {
							return m, func() tea.Msg { return quitMsg }
						}
					}
					m.items = getMenuItems()
				}
			}
		default:
			// 快捷键
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

	running, _ := checkRunning()

	s.WriteString("\n")
	s.WriteString(strings.Repeat("─", 50))
	s.WriteString("\n")
	s.WriteString("v2sp Control")
	s.WriteString("\n")
	s.WriteString(strings.Repeat("─", 50))
	s.WriteString("\n\n")

	if running {
		s.WriteString(ui.SuccessStyle.Render("● Running"))
	} else {
		s.WriteString(ui.DimStyle.Render("○ Stopped"))
	}
	s.WriteString("\n\n")

	visibleItems := m.getVisibleItems()
	for i, item := range visibleItems {
		cursor := "  "
		if m.cursor == i {
			cursor = ui.InfoStyle.Render("> ")
		}

		desc := ""
		if item.Description != "" {
			desc = ui.DimStyle.Render(item.Description)
		}
		s.WriteString(fmt.Sprintf("%s[%s] %-16s %s\n",
			cursor,
			ui.SuccessStyle.Render(item.Key),
			item.Title,
			desc,
		))
	}

	s.WriteString("\n")
	s.WriteString(strings.Repeat("─", 50))
	s.WriteString("\n")
	s.WriteString(ui.DimStyle.Render("↑↓/jk: navigate | Enter/Key: select | q: quit"))
	s.WriteString("\n")

	return s.String()
}

// 操作函数
func startServiceAction() tea.Msg {
	fmt.Print("\033[H\033[2J") // 清屏
	fmt.Println("Starting v2sp...")
	_, err := exec.RunCommandByShell("systemctl start v2sp")
	if err != nil {
		fmt.Println("✗ Failed:", err)
	} else {
		fmt.Println("✓ Started")
	}
	fmt.Print("\nPress Enter...")
	fmt.Scanln()
	return nil
}

func stopServiceAction() tea.Msg {
	fmt.Print("\033[H\033[2J")
	fmt.Println("Stopping v2sp...")
	_, err := exec.RunCommandByShell("systemctl stop v2sp")
	if err != nil {
		fmt.Println("✗ Failed:", err)
	} else {
		fmt.Println("✓ Stopped")
	}
	fmt.Print("\nPress Enter...")
	fmt.Scanln()
	return nil
}

func restartServiceAction() tea.Msg {
	fmt.Print("\033[H\033[2J")
	fmt.Println("Restarting v2sp...")
	_, err := exec.RunCommandByShell("systemctl restart v2sp")
	if err != nil {
		fmt.Println("✗ Failed:", err)
	} else {
		fmt.Println("✓ Restarted")
	}
	fmt.Print("\nPress Enter...")
	fmt.Scanln()
	return nil
}

func statusAction() tea.Msg {
	return runBinaryAction("Status", true, "status")
}

func logsAction() tea.Msg {
	fmt.Print("\033[H\033[2J")
	fmt.Println("Logs (Ctrl+C to exit)")
	fmt.Println(strings.Repeat("─", 50))
	runSelfBinary("logs", "-f")
	pausePrompt()
	return nil
}

func logsErrorAction() tea.Msg {
	return runBinaryAction("Logs (error)", true, "logs", "-l", "error", "-n", "100")
}

func editConfigAction() tea.Msg {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}
	if _, err := osexec.LookPath(editor); err != nil {
		fmt.Println("Editor not found:", editor)
		fmt.Println("Please install nano, e.g. apt install nano / yum install nano")
		pausePrompt()
		return nil
	}
	exec.RunCommandStd(editor, "/etc/v2sp/config.json")
	return nil
}

func healthCheckAction() tea.Msg {
	return runBinaryAction("Health Check", true, "health")
}

func configInitAction() tea.Msg {
	return runBinaryAction("Config Init", true, "config", "init")
}

func configShowAction() tea.Msg {
	return runBinaryAction("Config Show", true, "config", "show")
}

func configValidateAction() tea.Msg {
	return runBinaryAction("Config Validate", true, "config", "validate")
}

func systemSetupAction() tea.Msg {
	return runBinaryAction("System Setup", true, "system", "setup")
}

func syncTimeAction() tea.Msg {
	return runBinaryAction("Sync Time", true, "synctime")
}

func x25519Action() tea.Msg {
	return runBinaryAction("Generate X25519 key", true, "x25519")
}

func versionAction() tea.Msg {
	return runBinaryAction("Version", true, "version")
}

func runBinaryAction(title string, pause bool, args ...string) tea.Msg {
	fmt.Print("\033[H\033[2J")
	if title != "" {
		fmt.Println(title)
		fmt.Println(strings.Repeat("─", 50))
	}
	runSelfBinary(args...)
	if pause {
		pausePrompt()
	}
	return nil
}

func runSelfBinary(args ...string) {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		exe = "v2sp"
	}
	exec.RunCommandStd(exe, args...)
}

func pausePrompt() {
	fmt.Print("\nPress Enter...")
	fmt.Scanln()
}
