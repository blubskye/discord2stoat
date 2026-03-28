package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	labelStyle    = lipgloss.NewStyle().Width(22).Foreground(lipgloss.Color("243"))
	valueStyle    = lipgloss.NewStyle().Bold(true)
	subValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	buttonStyle   = lipgloss.NewStyle().Padding(0, 2).Background(lipgloss.Color("63")).Foreground(lipgloss.Color("255"))
	activeButton  = buttonStyle.Background(lipgloss.Color("57"))
)

// ConfirmEntry holds one source or target server's display info.
type ConfirmEntry struct {
	Label    string // e.g. "Source (Discord)"
	Name     string // resolved server name
	ServerID string
}

// ConfirmModel is the bubbletea model for Screen 1.
type ConfirmModel struct {
	entries  []ConfirmEntry
	selected int // 0 = Confirm, 1 = Quit
}

// NewConfirmModel creates Screen 1 with the resolved server entries.
func NewConfirmModel(entries []ConfirmEntry) ConfirmModel {
	return ConfirmModel{entries: entries}
}

func (m ConfirmModel) Init() tea.Cmd { return nil }

func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if m.selected > 0 {
				m.selected--
			}
		case "right", "l", "tab":
			if m.selected < 1 {
				m.selected++
			}
		case "enter", " ":
			if m.selected == 0 {
				return m, func() tea.Msg { return msgConfirmed{} }
			}
			return m, tea.Quit
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	s := titleStyle.Render("discord2stoat") + "\n\n"
	for _, e := range m.entries {
		s += labelStyle.Render(e.Label+":") + " " +
			valueStyle.Render(e.Name) + "  " +
			subValueStyle.Render(fmt.Sprintf("[%s]", e.ServerID)) + "\n"
	}
	s += "\n"

	btn0 := buttonStyle.Render("Confirm")
	if m.selected == 0 {
		btn0 = activeButton.Render("Confirm")
	}
	btn1 := buttonStyle.Render("Quit")
	if m.selected == 1 {
		btn1 = activeButton.Render("Quit")
	}
	s += btn0 + "  " + btn1
	return s
}
