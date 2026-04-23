package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vitekzach/doccompose/compose"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)

type serviceItem struct {
	name    string
	service compose.Service
}

func (i serviceItem) Title() string { return i.name }

func (i serviceItem) Description() string {
	if i.service.Image != "" {
		return "image: " + i.service.Image
	}
	if i.service.Build != nil && i.service.Build.Context != "" {
		return "build: " + i.service.Build.Context
	}
	return ""
}

func (i serviceItem) FilterValue() string { return i.name }

type Model struct {
	composePath string
	composeFile *compose.File
	list        list.Model
	width       int
	height      int
}

func New(composePath string, composeFile *compose.File) Model {
	items := makeItems(composeFile)

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("205")).
		BorderLeftForeground(lipgloss.Color("205"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("205")).
		BorderLeftForeground(lipgloss.Color("205"))

	l := list.New(items, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	return Model{
		composePath: composePath,
		composeFile: composeFile,
		list:        l,
	}
}

func makeItems(f *compose.File) []list.Item {
	items := make([]list.Item, 0, len(f.Services))
	for name, svc := range f.Services {
		items = append(items, serviceItem{name: name, service: svc})
	}
	return items
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-6)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("doccompose"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(fmt.Sprintf("%s  •  %d services", m.composePath, len(m.composeFile.Services))))
	b.WriteString("\n")
	b.WriteString(m.list.View())
	b.WriteString(helpStyle.Render("↑/↓ navigate  •  q quit"))

	return b.String()
}
