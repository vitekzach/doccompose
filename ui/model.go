package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vitekzach/doccompose/compose"
)

// ActualState is the live running state of a service as reported by Docker.
type ActualState int

const (
	ActualStopped ActualState = iota
	ActualRunning
)

// DesiredState is the user-selected target state.
type DesiredState int

const (
	DesiredStopped DesiredState = iota
	DesiredRunning
)

type serviceRow struct {
	name    string
	service compose.Service
	actual  ActualState
	desired DesiredState
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	nameStyle = lipgloss.NewStyle().
			Bold(true)

	nameCursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	checkOnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("78")) // green

	checkOffStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("78"))

	stoppedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
)

type Model struct {
	composePath string
	rows        []serviceRow
	cursor      int
	width       int
	height      int
}

func New(composePath string, composeFile *compose.File) Model {
	names := make([]string, 0, len(composeFile.Services))
	for name := range composeFile.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := make([]serviceRow, len(names))
	for i, name := range names {
		rows[i] = serviceRow{
			name:    name,
			service: composeFile.Services[name],
			actual:  ActualStopped,
			desired: DesiredStopped,
		}
	}

	return Model{
		composePath: composePath,
		rows:        rows,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case " ":
			row := &m.rows[m.cursor]
			if row.desired == DesiredStopped {
				row.desired = DesiredRunning
			} else {
				row.desired = DesiredStopped
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("doccompose"))
	b.WriteString("  ")
	b.WriteString(subtitleStyle.Render(fmt.Sprintf("%s  •  %d services", m.composePath, len(m.rows))))
	b.WriteString("\n\n")

	for i, row := range m.rows {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▶ ")
		}

		check := checkOffStyle.Render("[ ]")
		if row.desired == DesiredRunning {
			check = checkOnStyle.Render("[✓]")
		}

		var svcName string
		if i == m.cursor {
			svcName = nameCursorStyle.Render(row.name)
		} else {
			svcName = nameStyle.Render(row.name)
		}

		var source string
		if row.service.Image != "" {
			source = dimStyle.Render("image: " + row.service.Image)
		} else if row.service.Build != nil && row.service.Build.Context != "" {
			source = dimStyle.Render("build: " + row.service.Build.Context)
		}

		var status string
		if row.actual == ActualRunning {
			status = runningStyle.Render("● running")
		} else {
			status = stoppedStyle.Render("○ stopped")
		}

		b.WriteString(fmt.Sprintf("%s%s  %-28s  %-30s  %s\n",
			cursor, check, svcName, source, status,
		))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ navigate  •  space select  •  q quit"))

	return b.String()
}
