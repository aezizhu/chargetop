package main

import (
	"fmt"
	"time"

	"github.com/aezizhu/chargetop/battery"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"

	// "github.com/charmbracelet/bubbles/sparkline"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	danger    = lipgloss.AdaptiveColor{Light: "#F25D94", Dark: "#F55385"}
	textMuted = lipgloss.AdaptiveColor{Light: "#A8A8A8", Dark: "#626262"}

	appStyle = lipgloss.NewStyle().
			Margin(1, 1).
			Padding(1, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	listStyle = lipgloss.NewStyle().
			MarginRight(1).
			Height(8).
			Width(35).
			Padding(0, 1)

	detailStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Width(50).
			BorderLeft(true).
			BorderForeground(subtle)

	titleStyle = lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Foreground(special).
			Bold(true).
			Underline(true).
			MarginBottom(1)
)

// Keys
type keyMap struct {
	Quit    key.Binding
	Refresh key.Binding
	Help    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Refresh, k.Quit},
	}
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh now"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
}

type tickMsg time.Time

type model struct {
	info battery.BatteryInfo

	// sparkModel sparkline.Model
	history []int

	help help.Model
	keys keyMap

	width  int
	height int
	err    error
}

func initialModel() model {
	b, _ := battery.GetBatteryInfo()

	// Sparkline commented out until dependency fix

	return model{
		info: b,
		// sparkModel: s,
		history: []int{b.Percent},
		help:    help.New(),
		keys:    keys,
	}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Refresh):
			b, err := battery.GetBatteryInfo()
			m.info = b
			m.err = err
			return m, nil
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

	case tickMsg:
		b, err := battery.GetBatteryInfo()
		m.info = b
		m.err = err

		m.history = append(m.history, b.Percent)
		if len(m.history) > 60 {
			m.history = m.history[1:]
		}
		// m.sparkModel.SetData(m.history)

		return m, tickCmd()
	}

	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nError: %v\n", m.err)
	}

	// Dynamic Color for Percentage
	statusColor := special
	if m.info.Percent < 20 {
		statusColor = danger
	} else if m.info.Percent < 50 {
		statusColor = lipgloss.AdaptiveColor{Light: "#FFD700", Dark: "#D4AF37"} // Gold
	}

	// Left Column: Main Status + Graph
	pctView := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true).
		Align(lipgloss.Center).
		Width(12).
		Height(3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(subtle).
		Render(fmt.Sprintf("\n%d%%", m.info.Percent))

	statusText := lipgloss.NewStyle().Foreground(textMuted).Render(fmt.Sprintf("%s\n%s", m.info.Status, m.info.Remaining))

	graphView := "" // Placeholder for sparkline

	leftCol := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render("Power Status"),
		lipgloss.JoinHorizontal(lipgloss.Top, pctView, lipgloss.NewStyle().MarginLeft(2).Render(statusText)),
		lipgloss.NewStyle().MarginTop(2).Render(graphView),
	)

	// Right Column: Advanced Stats (+ Temperature)
	advContent := fmt.Sprintf(`
%s %s
%s    %d
%s    %s
%s       %.1f°C

%s   %s
%s   %dW
%s    %s
`,
		titleStyle.Render("Health:   "), m.info.Health,
		titleStyle.Render("Cycles:   "), m.info.CycleCount,
		titleStyle.Render("Max Cap:  "), fmt.Sprintf("%d%%", m.info.MaxCapacity),
		titleStyle.Render("Temp:     "), m.info.Temperature,
		titleStyle.Render("Charger:  "), "USB-C", // ioreg doesn't always give friendly name easily, hardcode or leave generic
		titleStyle.Render("Wattage:  "), m.info.Wattage,
		titleStyle.Render("Serial:   "), m.info.Serial,
	)

	rightCol := detailStyle.Render(
		headerStyle.Render("Health & Diagnostics") + "\n" + advContent,
	)

	// Combine
	mainView := lipgloss.JoinHorizontal(lipgloss.Top, listStyle.Render(leftCol), rightCol)

	// Add Help
	helpView := m.help.View(m.keys)

	finalView := lipgloss.JoinVertical(lipgloss.Left, mainView, lipgloss.NewStyle().MarginTop(1).Foreground(subtle).Render(helpView))

	return appStyle.Render(finalView)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
	}
}
