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
	Key       string
	Title     string
	Action    func() tea.Msg
	Condition func() bool
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
			Key:   "1",
			Title: "Start",
			Action: startServiceAction,
			Condition: func() bool {
				running, _ := checkRunning()
				return !running
			},
		},
		{
			Key:   "2",
			Title: "Stop",
			Action: stopServiceAction,
			Condition: func() bool {
				running, _ := checkRunning()
				return running
			},
		},
		{
			Key:       "3",
			Title:     "Restart",
			Action:    restartServiceAction,
			Condition: func() bool { return true },
		},
		{
			Key:       "s",
			Title:     "Status",
			Action:    statusAction,
			Condition: func() bool { return true },
		},
		{
			Key:       "l",
			Title:     "Logs",
			Action:    logsAction,
			Condition: func() bool { return true },
		},
		{
			Key:       "c",
			Title:     "Config",
			Action:    editConfigAction,
			Condition: func() bool { return true },
		},
		{
			Key:       "h",
			Title:     "Health",
			Action:    healthCheckAction,
			Condition: func() bool { return true },
		},
		{
			Key:       "q",
			Title:     "Quit",
			Action:    func() tea.Msg { return tea.Quit() },
			Condition: func() bool { return true },
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

		s.WriteString(fmt.Sprintf("%s[%s] %s\n",
			cursor,
			ui.SuccessStyle.Render(item.Key),
			item.Title,
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
	fmt.Print("\033[H\033[2J")
	showStatus()
	fmt.Print("\nPress Enter...")
	fmt.Scanln()
	return nil
}

func logsAction() tea.Msg {
	fmt.Print("\033[H\033[2J")
	fmt.Println("Logs (Ctrl+C to exit)")
	fmt.Println(strings.Repeat("─", 50))
	exec.RunCommandStd("journalctl", "-u", "v2sp", "-f", "-n", "50")
	return nil
}

func editConfigAction() tea.Msg {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	exec.RunCommandStd(editor, "/etc/v2sp/config.json")
	return nil
}

func healthCheckAction() tea.Msg {
	fmt.Print("\033[H\033[2J")
	healthHandle(nil, nil)
	fmt.Print("\nPress Enter...")
	fmt.Scanln()
	return nil
}
