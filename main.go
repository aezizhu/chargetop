package main

import (
	"fmt"
	"strings"
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
	// Apple-esque Palette
	bg       = lipgloss.Color("0")   // Pitch black or terminal default
	fg       = lipgloss.Color("255") // White
	subtle   = lipgloss.Color("240") // Dark Grey
	accent   = lipgloss.Color("39")  // Dodson Blue (Classic Apple)
	warning  = lipgloss.Color("208") // Orange
	critical = lipgloss.Color("196") // Red
	success  = lipgloss.Color("46")  // Green

	appStyle = lipgloss.NewStyle().
			Padding(1, 4).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Align(lipgloss.Center)

	mainTextStyle = lipgloss.NewStyle().
			Foreground(fg).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(subtle).
			Width(25) // Fixed width for alignment

	valueStyle = lipgloss.NewStyle().
			Foreground(fg).
			Bold(true)
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

// Messages
type tickMsg time.Time
type batteryMsg struct {
	info battery.BatteryInfo
	err  error
}

type model struct {
	info battery.BatteryInfo

	// sparkModel sparkline.Model
	history []int

	help help.Model
	keys keyMap

	width  int
	height int
	err    error
	now    time.Time
}

func initialModel() model {
	// Initial fetch is synchronous to populate first frame, or we can start empty
	b, _ := battery.GetBatteryInfo()

	return model{
		info:    b,
		history: []int{b.Percent},
		help:    help.New(),
		keys:    keys,
		now:     time.Now(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		fetchBatteryCmd(), // Start first fetch immediately
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Refresh):
			return m, fetchBatteryCmd()
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

	case tickMsg:
		m.now = time.Time(msg)
		// Every tick, we trigger a fetch in the background
		return m, tea.Batch(tickCmd(), fetchBatteryCmd())

	case batteryMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.info = msg.info
			m.err = nil
			m.history = append(m.history, msg.info.Percent)
			if len(m.history) > 60 {
				m.history = m.history[1:]
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n", m.err)
	}

	// Dynamic Status Color
	statusColor := success
	if m.info.Percent < 15 {
		statusColor = critical
	} else if m.info.Percent < 30 {
		statusColor = warning
	}

	// --- 1. The Big Percentage (The Hero) ---
	pctBig := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true).
		Render(fmt.Sprintf("%d%%", m.info.Percent))

	statusIcon := "⚡"
	if !m.info.IsCharging {
		statusIcon = "🔋"
	}

	heroSection := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Foreground(subtle).Render(strings.ToUpper(m.info.Status)),
		lipgloss.NewStyle().Margin(1, 0).Render(
			lipgloss.JoinHorizontal(lipgloss.Center,
				lipgloss.NewStyle().Foreground(statusColor).MarginRight(1).Render(statusIcon),
				pctBig,
			),
		),
		lipgloss.NewStyle().Foreground(subtle).Render(m.info.Remaining),
	)

	// --- 2. The Grid (The Details) ---
	row := func(label, value string) string {
		return lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render(label),
			valueStyle.Render(value),
		)
	}

	// Minimalist Divider
	divider := lipgloss.NewStyle().
		Foreground(subtle).
		Margin(1, 0).
		Render("───────────────────────────────────────")

	safeCycle := fmt.Sprintf("%d", m.info.CycleCount)
	if m.info.CycleCount == 0 {
		safeCycle = "..."
	}

	safeWattage := fmt.Sprintf("%dW", m.info.Wattage)
	if m.info.Wattage == 0 {
		safeWattage = "..."
	}

	statsSection := lipgloss.JoinVertical(lipgloss.Left,
		row("Condition", m.info.Condition),
		row("Cycle Count", safeCycle),
		row("Max Capacity", m.info.MaxCapacity),
		row("Temperature", fmt.Sprintf("%.1f°C", m.info.Temperature)),
		lipgloss.NewStyle().Height(1).Render(""),
		row("Power Source", "USB-C Power Type"),
		row("Wattage Input", safeWattage),
		row("Serial Number", m.info.Serial),
	)

	// Combine
	content := lipgloss.JoinVertical(lipgloss.Center,
		heroSection,
		divider,
		statsSection,
	)

	// Footer (Help + Clock)
	helpView := m.help.View(m.keys)
	clockView := lipgloss.NewStyle().Foreground(subtle).Render(m.now.Format("15:04:05"))

	footerRow := lipgloss.JoinHorizontal(lipgloss.Center,
		helpView,
		lipgloss.NewStyle().Foreground(subtle).Margin(0, 2).Render("•"),
		clockView,
	)

	footer := lipgloss.NewStyle().Foreground(subtle).MarginTop(2).Render(footerRow)

	return appStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			content,
			footer,
		),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchBatteryCmd() tea.Cmd {
	return func() tea.Msg {
		info, err := battery.GetBatteryInfo()
		return batteryMsg{info: info, err: err}
	}
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
	}
}
