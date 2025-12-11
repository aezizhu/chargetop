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

		return m, tickCmd()
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
	// Helper for rows
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

	// Helper for checking empty stats to avoid ugly zeros
	safeCycle := fmt.Sprintf("%d", m.info.CycleCount)
	if m.info.CycleCount == 0 {
		safeCycle = "..."
	}

	safeWattage := fmt.Sprintf("%dW", m.info.Wattage)
	if m.info.Wattage == 0 {
		safeWattage = "..."
	}

	statsSection := lipgloss.JoinVertical(lipgloss.Left,
		row("Battery Health", m.info.Health),
		row("Cycle Count", safeCycle),
		row("Temperature", fmt.Sprintf("%.1f°C", m.info.Temperature)),
		row("Max Capacity", fmt.Sprintf("%d%%", m.info.MaxCapacity)),
		lipgloss.NewStyle().Height(1).Render(""), // Spacer
		row("Power Source", "USB-C Power Type"),  // Static for now, consistent with goal
		row("Wattage Input", safeWattage),
		row("Serial Number", m.info.Serial),
	)

	// Combine
	content := lipgloss.JoinVertical(lipgloss.Center,
		heroSection,
		divider,
		statsSection,
	)

	// Footer (Subtle)
	helpFooter := m.help.View(m.keys)
	footer := lipgloss.NewStyle().Foreground(subtle).MarginTop(2).Render(helpFooter)

	return appStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			content,
			footer,
		),
	)
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
