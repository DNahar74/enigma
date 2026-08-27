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
	"github.com/DNahar74/enigma/core/reader"
	"github.com/DNahar74/enigma/core/render"
)

var asciiArt = `
███████╗███╗   ██╗██╗ ██████╗ ███╗   ███╗ █████╗ 
██╔════╝████╗  ██║██║██╔════╝ ████╗ ████║██╔══██╗
█████╗  ██╔██╗ ██║██║██║  ███╗██╔████╔██║███████║
██╔══╝  ██║╚██╗██║██║██║   ██║██║╚██╔╝██║██╔══██║
███████╗██║ ╚████║██║╚██████╔╝██║ ╚═╝ ██║██║  ██║
╚══════╝╚═╝  ╚═══╝╚═╝ ╚═════╝ ╚═╝     ╚═╝╚═╝  ╚═╝
`

type sessionState int

const (
	stateHome sessionState = iota
	stateSearching
	stateResults
	stateDetail
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true)
	indieStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)

	activeTabBorder = lipgloss.Border{
		Top: "─", Bottom: " ", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┘", BottomRight: "└",
	}
	inactiveTabBorder = lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┴", BottomRight: "┴",
	}

	activeTabStyle   = lipgloss.NewStyle().Border(activeTabBorder, true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().Border(inactiveTabBorder, true).Foreground(lipgloss.Color("240")).Padding(0, 1)
	windowStyle      = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderTop(false)
)

type searchResultMsg struct {
	tabIndex int
	results  []plugin.ScoredResult
	err      error
}

type readerResultMsg struct {
	tabIndex int
	content  string
	err      error
}

type TabType int

const (
	TabTypeSearch TabType = iota
	TabTypeReader
)

type Tab struct {
	Type  TabType
	Title string

	// Search
	searchState sessionState
	textInput   textinput.Model
	queryStr    string
	parsedQuery query.Query
	results     []plugin.ScoredResult
	err         error
	cursor      int
	detailView  viewport.Model

	// Reader
	url        string
	readerView viewport.Model
	loading    bool
}

type Model struct {
	pipeline  *pipeline.Pipeline
	tabs      []*Tab
	activeTab int
	width     int
	height    int
}

func newSearchTab() *Tab {
	ti := textinput.New()
	ti.Placeholder = "Enter your search query..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	return &Tab{
		Type:        TabTypeSearch,
		Title:       "Search",
		searchState: stateHome,
		textInput:   ti,
	}
}

func newReaderTab(url string) *Tab {
	return &Tab{
		Type:    TabTypeReader,
		Title:   "Web",
		url:     url,
		loading: true,
	}
}

func New(p *pipeline.Pipeline) Model {
	return Model{
		pipeline:  p,
		tabs:      []*Tab{newSearchTab()},
		activeTab: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			tab := m.tabs[m.activeTab]
			if tab.Type == TabTypeSearch && tab.searchState == stateDetail {
				tab.searchState = stateResults
				return m, nil
			}
			return m, tea.Quit
		}

		switch msg.String() {
		case "tab", "right":
			if len(m.tabs) > 1 {
				m.activeTab = (m.activeTab + 1) % len(m.tabs)
			}
			return m, nil
		case "shift+tab", "left":
			if len(m.tabs) > 1 {
				m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			}
			return m, nil
		case "ctrl+w", "x":
			// Close current tab, unless it's the only one
			if len(m.tabs) > 1 {
				m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
				if m.activeTab >= len(m.tabs) {
					m.activeTab = len(m.tabs) - 1
				}
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		for _, t := range m.tabs {
			if t.Type == TabTypeReader {
				t.readerView.Width = m.width - 4
				t.readerView.Height = m.height - 6
			} else if t.Type == TabTypeSearch {
				t.detailView.Width = m.width - 4
				t.detailView.Height = m.height - 6
			}
		}

	case searchResultMsg:
		if msg.tabIndex < len(m.tabs) {
			tab := m.tabs[msg.tabIndex]
			if msg.err != nil {
				tab.err = msg.err
				tab.searchState = stateHome
			} else {
				tab.results = msg.results
				tab.cursor = 0
				tab.searchState = stateResults
			}
		}
		return m, nil

	case readerResultMsg:
		if msg.tabIndex < len(m.tabs) {
			tab := m.tabs[msg.tabIndex]
			tab.loading = false
			if msg.err != nil {
				tab.err = msg.err
				tab.readerView.SetContent(fmt.Sprintf("Error fetching page: %v", msg.err))
			} else {
				tab.readerView.SetContent(msg.content)
			}
		}
		return m, nil
	}

	tab := m.tabs[m.activeTab]
	var cmd tea.Cmd

	if tab.Type == TabTypeSearch {
		switch tab.searchState {
		case stateHome:
			switch msg := msg.(type) {
			case tea.KeyMsg:
				if msg.Type == tea.KeyEnter {
					qStr := strings.TrimSpace(tab.textInput.Value())
					if qStr != "" {
						q, err := query.Parse(qStr)
						if err != nil {
							tab.err = err
							break
						}
						tab.queryStr = qStr
						tab.parsedQuery = q
						tab.searchState = stateSearching
						cmds = append(cmds, m.performSearch(m.activeTab, qStr))
					}
				}
			}
			tab.textInput, cmd = tab.textInput.Update(msg)
			cmds = append(cmds, cmd)

		case stateResults:
			switch msg := msg.(type) {
			case tea.KeyMsg:
				switch msg.String() {
				case "up", "k":
					if tab.cursor > 0 {
						tab.cursor--
					}
				case "down", "j":
					if tab.cursor < len(tab.results)-1 {
						tab.cursor++
					}
				case "enter":
					if len(tab.results) > 0 {
						tab.searchState = stateDetail
						tab.detailView = viewport.New(m.width-4, m.height-6)
						tab.detailView.SetContent(m.detailViewContent(tab))
						tab.detailView.GotoTop()
					}
				case "s":
					tab.searchState = stateHome
					tab.textInput.SetValue("")
				case "r":
					// Open in Web Reader Tab
					if len(tab.results) > 0 {
						targetURL := tab.results[tab.cursor].Result.URL
						if targetURL != "" && strings.HasPrefix(targetURL, "http") {
							newTab := newReaderTab(targetURL)
							newTab.readerView = viewport.New(m.width-4, m.height-6)
							m.tabs = append(m.tabs, newTab)
							m.activeTab = len(m.tabs) - 1
							cmds = append(cmds, m.performFetch(m.activeTab, targetURL, m.width-8))
						}
					}
				}
			}
		case stateDetail:
			tab.detailView, cmd = tab.detailView.Update(msg)
			cmds = append(cmds, cmd)
		}
	} else if tab.Type == TabTypeReader {
		tab.readerView, cmd = tab.readerView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) performSearch(tabIndex int, qStr string) tea.Cmd {
	return func() tea.Msg {
		q, _ := query.Parse(qStr)
		results, err := m.pipeline.Execute(context.Background(), q)
		return searchResultMsg{tabIndex: tabIndex, results: results, err: err}
	}
}

func (m Model) performFetch(tabIndex int, url string, width int) tea.Cmd {
	return func() tea.Msg {
		content, err := reader.FetchAndRender(context.Background(), url, width)
		return readerResultMsg{tabIndex: tabIndex, content: content, err: err}
	}
}

func (m Model) detailViewContent(tab *Tab) string {
	if tab.cursor < 0 || tab.cursor >= len(tab.results) {
		return "Invalid selection."
	}
	r := tab.results[tab.cursor]
	return render.Result(r, tab.parsedQuery, tab.cursor+1, true)
}

func (m Model) View() string {
	var renderedTabs []string
	for i, tab := range m.tabs {
		title := tab.Title
		if len(title) > 12 {
			title = title[:10] + ".."
		}
		if i == m.activeTab {
			renderedTabs = append(renderedTabs, activeTabStyle.Render(title))
		} else {
			renderedTabs = append(renderedTabs, inactiveTabStyle.Render(title))
		}
	}
	tabRow := lipgloss.JoinHorizontal(lipgloss.Bottom, renderedTabs...)

	// Right align instructions
	instructions := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Tab/Shift+Tab: switch | Ctrl+W: close tab")
	headerRow := lipgloss.JoinHorizontal(lipgloss.Bottom, tabRow, "  ", instructions)

	var content string
	tab := m.tabs[m.activeTab]

	if tab.err != nil {
		content = fmt.Sprintf("Error: %v\nPress Esc to quit.", tab.err)
	} else if tab.Type == TabTypeSearch {
		switch tab.searchState {
		case stateHome:
			content = fmt.Sprintf("%s\n\n%s\n\n(Press Enter to search, Esc to quit)",
				indieStyle.Render(asciiArt),
				tab.textInput.View(),
			)
		case stateSearching:
			content = fmt.Sprintf("Searching for %q...\n", tab.queryStr)
		case stateResults:
			if len(tab.results) == 0 {
				content = "No results found.\nPress 's' to search again, Esc to quit."
			} else {
				var b strings.Builder
				b.WriteString(titleStyle.Render(fmt.Sprintf("Results for %q (Enter: View Details, 'r': Read Web, 's': New Search)", tab.queryStr)) + "\n\n")
				start := 0
				if tab.cursor > 10 {
					start = tab.cursor - 10
				}
				for i := start; i < len(tab.results) && i < start+20; i++ {
					r := tab.results[i]
					cursor := " "
					title := r.Result.Title
					if tab.cursor == i {
						cursor = ">"
						title = selectedStyle.Render(title)
					}
					emoji := "🌐"
					if r.Result.SourcePlugin == "local" {
						emoji = "📝"
					}
					b.WriteString(fmt.Sprintf("%s %d. %s %s\n", cursor, i+1, emoji, title))
				}
				content = b.String()
			}
		case stateDetail:
			content = fmt.Sprintf("%s\n\n%s",
				titleStyle.Render("Detail View (Press Esc to return)"),
				tab.detailView.View(),
			)
		}
	} else if tab.Type == TabTypeReader {
		if tab.loading {
			content = fmt.Sprintf("Fetching %s...\n\n(This might take a moment to parse)", tab.url)
		} else {
			content = tab.readerView.View()
		}
	}

	contentBox := windowStyle.
		Width(m.width-2).
		Height(m.height-3).
		Padding(1, 2).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, headerRow, contentBox)
}
