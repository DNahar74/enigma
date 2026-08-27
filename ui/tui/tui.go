package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/DNahar74/enigma/core/pipeline"
	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
	"github.com/DNahar74/enigma/core/render"
)

type sessionState int

const (
	stateSearch sessionState = iota
	stateSearching
	stateResults
	stateDetail
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)
)

type searchResultMsg struct {
	results []plugin.ScoredResult
	err     error
}

type Model struct {
	pipeline *pipeline.Pipeline

	state     sessionState
	textInput textinput.Model
	viewport  viewport.Model

	queryStr    string
	parsedQuery query.Query

	results []plugin.ScoredResult
	err     error

	cursor int

	width  int
	height int
}

func New(p *pipeline.Pipeline) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter your search query..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	return Model{
		pipeline:  p,
		state:     stateSearch,
		textInput: ti,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.state == stateDetail {
				m.state = stateResults
				return m, nil
			}
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport = viewport.New(msg.Width, msg.Height-4)
		if m.state == stateDetail {
			m.viewport.SetContent(m.detailView())
		}

	case searchResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateSearch
			return m, nil
		}
		m.results = msg.results
		m.cursor = 0
		m.state = stateResults
		return m, nil
	}

	switch m.state {
	case stateSearch:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEnter {
				qStr := strings.TrimSpace(m.textInput.Value())
				if qStr != "" {
					q, err := query.Parse(qStr)
					if err != nil {
						m.err = err
						return m, nil
					}
					m.queryStr = qStr
					m.parsedQuery = q
					m.state = stateSearching
					return m, m.performSearch(qStr)
				}
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd

	case stateSearching:
		// Just waiting for searchResultMsg
		return m, nil

	case stateResults:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.results)-1 {
					m.cursor++
				}
			case "enter":
				if len(m.results) > 0 {
					m.state = stateDetail
					m.viewport.SetContent(m.detailView())
					m.viewport.GotoTop()
				}
			case "s":
				// Back to search
				m.state = stateSearch
				m.textInput.SetValue("")
			}
		}

	case stateDetail:
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) performSearch(qStr string) tea.Cmd {
	return func() tea.Msg {
		q, _ := query.Parse(qStr)
		results, err := m.pipeline.Execute(context.Background(), q)
		return searchResultMsg{results: results, err: err}
	}
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress Esc to quit.", m.err)
	}

	switch m.state {
	case stateSearch:
		return fmt.Sprintf(
			"Enigma Search\n\n%s\n\n(Press Enter to search, Esc to quit)",
			m.textInput.View(),
		)

	case stateSearching:
		return fmt.Sprintf("Searching for %q...\n", m.queryStr)

	case stateResults:
		if len(m.results) == 0 {
			return "No results found.\nPress 's' to search again, Esc to quit."
		}

		var b strings.Builder
		b.WriteString(titleStyle.Render(fmt.Sprintf("Results for %q (Press Enter to view, 's' to search again)", m.queryStr)) + "\n\n")

		start := 0
		if m.cursor > 10 {
			start = m.cursor - 10
		}

		for i := start; i < len(m.results) && i < start+20; i++ {
			r := m.results[i]
			cursor := " "
			title := r.Result.Title

			if m.cursor == i {
				cursor = ">"
				title = selectedStyle.Render(title)
			}

			emoji := "🌐"
			if r.Result.SourcePlugin == "local" {
				emoji = "📝"
			}

			b.WriteString(fmt.Sprintf("%s %d. %s %s\n", cursor, i+1, emoji, title))
		}

		return b.String()

	case stateDetail:
		return fmt.Sprintf("%s\n\n%s",
			titleStyle.Render("Detail View (Press Esc to return)"),
			m.viewport.View(),
		)
	}

	return "Unknown state."
}

func (m Model) detailView() string {
	if m.cursor < 0 || m.cursor >= len(m.results) {
		return "Invalid selection."
	}
	r := m.results[m.cursor]

	return render.Result(r, m.parsedQuery, m.cursor+1, true)
}
