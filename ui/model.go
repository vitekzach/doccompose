package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vitekzach/doccompose/compose"
	"github.com/vitekzach/doccompose/docker"
)

// ActualState is the live running state reported by Docker.
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
	busy    bool
	busyMsg string
}

// tea messages

type statusesMsg struct {
	statuses map[string]bool
	err      error
}

type actionDoneMsg struct {
	service string
	err     error
}

type pollTickMsg struct{}

// commands

func fetchStatusesCmd(client docker.Client, composePath string) tea.Cmd {
	return func() tea.Msg {
		statuses, err := client.GetStatuses(composePath)
		return statusesMsg{statuses: statuses, err: err}
	}
}

func startServiceCmd(client docker.Client, composePath, service string) tea.Cmd {
	return func() tea.Msg {
		err := client.Start(composePath, service)
		return actionDoneMsg{service: service, err: err}
	}
}

func stopServiceCmd(client docker.Client, composePath, service string) tea.Cmd {
	return func() tea.Msg {
		err := client.Stop(composePath, service)
		return actionDoneMsg{service: service, err: err}
	}
}

func pollTickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

// styles

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

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
			Foreground(lipgloss.Color("78"))

	checkOffStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("78"))

	stoppedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	busyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))
)

type Model struct {
	composePath string
	client      docker.Client
	rows        []serviceRow
	cursor      int
	spinner     spinner.Model
	lastErr     string
	width       int
	height      int
}

func New(composePath string, composeFile *compose.File, client docker.Client) Model {
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
		}
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = busyStyle

	return Model{
		composePath: composePath,
		client:      client,
		rows:        rows,
		spinner:     sp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchStatusesCmd(m.client, m.composePath),
		pollTickCmd(),
		m.spinner.Tick,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case pollTickMsg:
		return m, tea.Batch(fetchStatusesCmd(m.client, m.composePath), pollTickCmd())

	case statusesMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			return m, nil
		}
		m.lastErr = ""
		for i := range m.rows {
			if running, ok := msg.statuses[m.rows[i].name]; ok {
				if running {
					m.rows[i].actual = ActualRunning
				} else {
					m.rows[i].actual = ActualStopped
				}
			} else {
				m.rows[i].actual = ActualStopped
			}
		}

	case actionDoneMsg:
		for i := range m.rows {
			if m.rows[i].name == msg.service {
				m.rows[i].busy = false
				m.rows[i].busyMsg = ""
				break
			}
		}
		if msg.err != nil {
			m.lastErr = fmt.Sprintf("%s: %s", msg.service, msg.err)
		} else {
			m.lastErr = ""
		}
		return m, fetchStatusesCmd(m.client, m.composePath)

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
			if row.busy {
				break
			}
			if row.desired == DesiredStopped {
				row.desired = DesiredRunning
				row.busy = true
				row.busyMsg = "starting"
				return m, startServiceCmd(m.client, m.composePath, row.name)
			} else {
				row.desired = DesiredStopped
				row.busy = true
				row.busyMsg = "stopping"
				return m, stopServiceCmd(m.client, m.composePath, row.name)
			}
		}
	}

	return m, nil
}

func (m Model) anyBusy() bool {
	for _, r := range m.rows {
		if r.busy {
			return true
		}
	}
	return false
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
		if row.busy {
			status = busyStyle.Render(m.spinner.View() + row.busyMsg)
		} else if row.actual == ActualRunning {
			status = runningStyle.Render("● running")
		} else {
			status = stoppedStyle.Render("○ stopped")
		}

		b.WriteString(fmt.Sprintf("%s%s  %-28s  %-30s  %s\n",
			cursor, check, svcName, source, status,
		))
	}

	b.WriteString("\n")

	if m.lastErr != "" {
		b.WriteString(errorStyle.Render("error: "+m.lastErr) + "\n")
	}

	b.WriteString(helpStyle.Render("↑/↓ navigate  •  space start/stop  •  q quit"))

	return b.String()
}
