package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vitekzach/doccompose/compose"
	"github.com/vitekzach/doccompose/docker"
)

var logPalette = []lipgloss.Color{
	"39", "118", "214", "171", "51", "196", "226", "213", "82", "45",
}

func serviceColor(name string) lipgloss.Color {
	h := 0
	for _, c := range name {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return logPalette[h%len(logPalette)]
}

// colorizeLogLine detects the "service  | message" prefix emitted by compose
// logs and applies a deterministic per-service color to the prefix.
func colorizeLogLine(line string) string {
	idx := strings.Index(line, " | ")
	if idx < 0 {
		return line
	}
	prefix := line[:idx]
	rest := line[idx:]
	color := serviceColor(strings.TrimSpace(prefix))
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(prefix) + rest
}

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
	output  string
	err     error
}

type downDoneMsg struct {
	output string
	err    error
}

type upAllDoneMsg struct {
	output string
	err    error
}

type pollTickMsg struct{}

type logLineMsg string

type logFollowerStartedMsg struct {
	ch <-chan string
}

type logFollowerDoneMsg struct{}

type restartLogFollowerMsg struct{}

type buildStartedMsg struct {
	lineCh  <-chan string
	errCh   <-chan error
	service string // empty = build all
}

type buildLineMsg string

type buildDoneMsg struct {
	service string
	err     error
}

// commands

func fetchStatusesCmd(client docker.Client, composePath string) tea.Cmd {
	return func() tea.Msg {
		statuses, err := client.GetStatuses(composePath)
		return statusesMsg{statuses: statuses, err: err}
	}
}

func startServiceCmd(client docker.Client, composePath, service string) tea.Cmd {
	return func() tea.Msg {
		output, err := client.Start(composePath, service)
		return actionDoneMsg{service: service, output: output, err: err}
	}
}

func stopServiceCmd(client docker.Client, composePath, service string) tea.Cmd {
	return func() tea.Msg {
		output, err := client.Stop(composePath, service)
		return actionDoneMsg{service: service, output: output, err: err}
	}
}

func downCmd(client docker.Client, composePath string) tea.Cmd {
	return func() tea.Msg {
		output, err := client.Down(composePath)
		return downDoneMsg{output: output, err: err}
	}
}

func upAllCmd(client docker.Client, composePath string) tea.Cmd {
	return func() tea.Msg {
		output, err := client.UpAll(composePath)
		return upAllDoneMsg{output: output, err: err}
	}
}

func pollTickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

func buildCmd(client docker.Client, composePath, service string) tea.Cmd {
	return func() tea.Msg {
		lineCh, errCh, err := client.StreamBuild(composePath, service)
		if err != nil {
			return buildDoneMsg{service: service, err: err}
		}
		return buildStartedMsg{lineCh: lineCh, errCh: errCh, service: service}
	}
}

func waitForBuildLine(lineCh <-chan string, errCh <-chan error, service string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-lineCh
		if !ok {
			return buildDoneMsg{service: service, err: <-errCh}
		}
		return buildLineMsg(line)
	}
}

func startLogFollowerCmd(client docker.Client, composePath string) tea.Cmd {
	return func() tea.Msg {
		ch, err := client.FollowLogs(composePath)
		if err != nil {
			return logFollowerDoneMsg{} // will retry
		}
		return logFollowerStartedMsg{ch: ch}
	}
}

func waitForLogLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logFollowerDoneMsg{}
		}
		return logLineMsg(line)
	}
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

	btnStartStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("78")).
			Padding(0, 1).
			Bold(true)

	btnStopStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("196")).
			Padding(0, 1).
			Bold(true)

	btnBuildStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("214")).
			Padding(0, 1).
			Bold(true)

	logBorderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	logTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	logSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238"))

	colNameStyle   = lipgloss.NewStyle().Width(24)
	colSourceStyle = lipgloss.NewStyle().Width(34)
	colStatusStyle = lipgloss.NewStyle().Width(12)
)

const (
	// fixed lines outside the log viewport:
	// title+gap(2) + gap+buttons(2) + box borders(2) + blank after box(1) = 7; help has no trailing newline
	fixedLines   = 7
	minLogHeight = 3
	maxLogLines  = 2000
)

type Model struct {
	composePath  string
	dirName      string
	client       docker.Client
	rows         []serviceRow
	cursor       int
	spinner      spinner.Model
	log          viewport.Model
	logLines     []string
	logCh        <-chan string
	buildLineCh  <-chan string
	buildErrCh   <-chan error
	buildService string
	lastErr      string
	hideTop      bool
	width        int
	height       int
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

	vp := viewport.New(0, minLogHeight)

	cwd, _ := os.Getwd()
	dirName := filepath.Base(cwd)

	return Model{
		composePath: composePath,
		dirName:     dirName,
		client:      client,
		rows:        rows,
		spinner:     sp,
		log:         vp,
	}
}

func (m *Model) appendLogLines(lines ...string) {
	atBottom := m.log.AtBottom()
	m.logLines = append(m.logLines, lines...)
	if len(m.logLines) > maxLogLines {
		m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
		m.logLines[0] = logSeparatorStyle.Render(fmt.Sprintf("── (oldest lines dropped, showing last %d) ──", maxLogLines))
	}
	m.log.SetContent(strings.Join(m.logLines, "\n"))
	if atBottom {
		m.log.GotoBottom()
	}
}

// appendCommandOutput adds a labelled block of command output to the log.
func (m *Model) appendCommandOutput(label, output string) {
	sep := logSeparatorStyle.Render(fmt.Sprintf("── %s %s", label, strings.Repeat("─", max(0, 40-len(label)))))
	lines := []string{sep}
	if output != "" {
		lines = append(lines, strings.Split(output, "\n")...)
	}
	m.appendLogLines(lines...)
}

func (m *Model) resizeLog() {
	if m.width == 0 || m.height == 0 {
		return
	}
	var available int
	if m.hideTop {
		// only the log box borders (top+bottom) and the blank line after remain
		available = m.height - 3
	} else {
		available = m.height - fixedLines - len(m.rows) - 1
	}
	logH := available
	if logH < minLogHeight {
		logH = minLogHeight
	}
	m.log.Width = m.width - 4 // 1 border + 1 space on each side
	m.log.Height = logH
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) renderLogBox() string {
	w := m.width
	if w <= 0 {
		w = 60
	}
	bd := logBorderStyle
	title := logTitleStyle.Render(" LOGS ")
	tw := lipgloss.Width(title)
	iw := w - 2 // chars between ╭ and ╮

	// ╭─ LOGS ──────╮
	rightDashes := iw - 1 - tw // 1 dash + space before title already counted in "─ "
	if rightDashes < 0 {
		rightDashes = 0
	}
	top := bd.Render("╭─") + title + bd.Render(strings.Repeat("─", rightDashes)+"╮")

	// │ <content> │
	cw := m.log.Width
	var mid strings.Builder
	for _, line := range strings.Split(m.log.View(), "\n") {
		pad := max(0, cw-lipgloss.Width(line))
		mid.WriteString(bd.Render("│") + " " + line + strings.Repeat(" ", pad) + " " + bd.Render("│") + "\n")
	}

	// ╰─────────────╯
	bottom := bd.Render("╰" + strings.Repeat("─", iw) + "╯")

	return top + "\n" + mid.String() + bottom
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("doccompose — "+m.dirName),
		fetchStatusesCmd(m.client, m.composePath),
		pollTickCmd(),
		m.spinner.Tick,
		startLogFollowerCmd(m.client, m.composePath),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeLog()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case pollTickMsg:
		return m, tea.Batch(fetchStatusesCmd(m.client, m.composePath), pollTickCmd())

	case logFollowerStartedMsg:
		m.logCh = msg.ch
		return m, waitForLogLine(m.logCh)

	case logLineMsg:
		m.appendLogLines(colorizeLogLine(string(msg)))
		return m, waitForLogLine(m.logCh)

	case logFollowerDoneMsg:
		// follower process exited; restart after a short delay so we
		// re-attach once containers come back up
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return restartLogFollowerMsg{}
		})

	case restartLogFollowerMsg:
		return m, startLogFollowerCmd(m.client, m.composePath)

	case statusesMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			return m, nil
		}
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

	case upAllDoneMsg:
		for i := range m.rows {
			m.rows[i].busy = false
			m.rows[i].busyMsg = ""
			m.rows[i].desired = DesiredRunning
		}
		m.appendCommandOutput("compose up -d", msg.output)
		if msg.err != nil {
			m.lastErr = msg.err.Error()
		} else {
			m.lastErr = ""
		}
		return m, fetchStatusesCmd(m.client, m.composePath)

	case downDoneMsg:
		for i := range m.rows {
			m.rows[i].busy = false
			m.rows[i].busyMsg = ""
			m.rows[i].desired = DesiredStopped
		}
		m.appendCommandOutput("compose down", msg.output)
		if msg.err != nil {
			m.lastErr = msg.err.Error()
		} else {
			m.lastErr = ""
		}
		return m, fetchStatusesCmd(m.client, m.composePath)

	case actionDoneMsg:
		for i := range m.rows {
			if m.rows[i].name == msg.service {
				m.rows[i].busy = false
				m.rows[i].busyMsg = ""
				break
			}
		}
		m.appendCommandOutput(msg.service, msg.output)
		if msg.err != nil {
			m.lastErr = fmt.Sprintf("%s: %s", msg.service, msg.err)
		} else {
			m.lastErr = ""
		}
		return m, fetchStatusesCmd(m.client, m.composePath)

	case buildStartedMsg:
		m.buildLineCh = msg.lineCh
		m.buildErrCh = msg.errCh
		m.buildService = msg.service
		label := msg.service
		if label == "" {
			label = "all"
		}
		m.appendLogLines(logSeparatorStyle.Render(fmt.Sprintf("── building %s ──", label)))
		return m, waitForBuildLine(m.buildLineCh, m.buildErrCh, m.buildService)

	case buildLineMsg:
		m.appendLogLines(string(msg))
		return m, waitForBuildLine(m.buildLineCh, m.buildErrCh, m.buildService)

	case buildDoneMsg:
		m.buildLineCh = nil
		m.buildErrCh = nil
		if msg.service == "" {
			for i := range m.rows {
				if m.rows[i].busyMsg == "building" {
					m.rows[i].busy = false
					m.rows[i].busyMsg = ""
				}
			}
		} else {
			for i := range m.rows {
				if m.rows[i].name == msg.service {
					m.rows[i].busy = false
					m.rows[i].busyMsg = ""
					break
				}
			}
		}
		if msg.err != nil {
			m.lastErr = fmt.Sprintf("build: %s", msg.err)
			m.appendLogLines(logSeparatorStyle.Render(fmt.Sprintf("── build failed: %s ──", msg.err)))
		} else {
			m.lastErr = ""
			m.appendLogLines(logSeparatorStyle.Render("── build complete ──"))
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
		case "pgup":
			m.log.HalfViewUp()
		case "pgdown":
			m.log.HalfViewDown()
		case "h":
			m.hideTop = !m.hideTop
			m.resizeLog()
		case "G", "end":
			m.log.GotoBottom()
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
		case "s":
			for i := range m.rows {
				m.rows[i].busy = true
				m.rows[i].busyMsg = "starting"
			}
			return m, upAllCmd(m.client, m.composePath)
		case "x":
			for i := range m.rows {
				m.rows[i].busy = true
				m.rows[i].busyMsg = "stopping"
			}
			return m, downCmd(m.client, m.composePath)
		case "b":
			if m.buildLineCh != nil {
				break
			}
			row := &m.rows[m.cursor]
			if row.busy {
				break
			}
			row.busy = true
			row.busyMsg = "building"
			return m, buildCmd(m.client, m.composePath, row.name)
		case "B":
			if m.buildLineCh != nil {
				break
			}
			for i := range m.rows {
				if m.rows[i].service.Build != nil {
					m.rows[i].busy = true
					m.rows[i].busyMsg = "building"
				}
			}
			return m, buildCmd(m.client, m.composePath, "")
		}
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	if !m.hideTop {
		// header — centered across terminal width
		sep := subtitleStyle.Render("  •  ")
		header := titleStyle.Render("doccompose") +
			sep + titleStyle.Render(m.dirName) +
			sep + subtitleStyle.Render(m.composePath) +
			sep + subtitleStyle.Render(fmt.Sprintf("%d services", len(m.rows)))
		w := m.width
		if w <= 0 {
			w = 80
		}
		b.WriteString(lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(header))
		b.WriteString("\n\n")

		// service list
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

			b.WriteString(cursor)
			b.WriteString(check)
			b.WriteString("  ")
			b.WriteString(colNameStyle.Render(svcName))
			b.WriteString("  ")
			b.WriteString(colSourceStyle.Render(source))
			b.WriteString("  ")
			b.WriteString(colStatusStyle.Render(status))
			b.WriteString("\n")
		}

		// buttons
		b.WriteString("\n")
		b.WriteString(btnStartStyle.Render("▶ Start All (s)"))
		b.WriteString("  ")
		b.WriteString(btnStopStyle.Render("■ Stop All (x)"))
		b.WriteString("  ")
		b.WriteString(btnBuildStyle.Render("⚙ Build All (B)"))
		b.WriteString("\n")
	}

	// log box
	b.WriteString(m.renderLogBox())
	b.WriteString("\n")

	// error + help
	if m.lastErr != "" {
		b.WriteString(errorStyle.Render("error: " + m.lastErr))
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("↑/↓ navigate  •  space toggle  •  s start all  •  x stop all  •  b build  •  B build all  •  pgup/pgdn scroll  •  G end  •  h toggle panel  •  q quit"))

	return b.String()
}
