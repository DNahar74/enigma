package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
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

// Theme Colors
var (
	colorNeonPink = lipgloss.Color("#FF00FF")
	colorCyan     = lipgloss.Color("#00FFFF")
	colorPurple   = lipgloss.Color("#8A2BE2")
	colorDark     = lipgloss.Color("#1E1E1E")
	colorGray     = lipgloss.Color("#444444")
	colorText     = lipgloss.Color("#E0E0E0")
)

// Styles
var (
	indieStyle = lipgloss.NewStyle().Foreground(colorNeonPink).Bold(true)

	activeTabStyle = lipgloss.NewStyle().
			Border(lipgloss.Border{Top: "─", Bottom: " ", Left: "│", Right: "│", TopLeft: "╭", TopRight: "╮", BottomLeft: "┘", BottomRight: "└"}, true).
			Foreground(colorCyan).
			Bold(true).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Border(lipgloss.Border{Top: "─", Bottom: "─", Left: "│", Right: "│", TopLeft: "╭", TopRight: "╮", BottomLeft: "┴", BottomRight: "┴"}, true).
				Foreground(colorGray).
				Padding(0, 1)

	windowStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorGray).
			BorderTop(false)

	searchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPurple).
			Padding(1, 2)

	statusBarText  = lipgloss.NewStyle().Foreground(colorDark).Background(colorPurple).Padding(0, 1)
	statusBarStyle = lipgloss.NewStyle().Foreground(colorGray).Padding(0, 1)

	readerHeaderStyle = lipgloss.NewStyle().Background(colorCyan).Foreground(colorDark).Padding(0, 1).Bold(true)
)

type sessionState int

const (
	stateHome sessionState = iota
	stateSearching
	stateResults
	stateDetail
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

// list item implementation
type item struct {
	res plugin.ScoredResult
}

func (i item) Title() string       { return i.res.Result.Title }
func (i item) Description() string { return i.res.Result.URL }
func (i item) FilterValue() string { return i.res.Result.Title + " " + i.res.Result.URL }

type Tab struct {
	Type  TabType
	Title string

	// Search
	searchState sessionState
	textInput   textinput.Model
	queryStr    string
	parsedQuery query.Query
	list        list.Model
	err         error
	detailView  viewport.Model
	detailRes   plugin.ScoredResult

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
	ti.Placeholder = "Enter a query to begin..."
	ti.Prompt = "🔍 "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colorCyan)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorNeonPink)
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	del := list.NewDefaultDelegate()
	del.Styles.SelectedTitle = del.Styles.SelectedTitle.Foreground(colorNeonPink).BorderForeground(colorNeonPink)
	del.Styles.SelectedDesc = del.Styles.SelectedDesc.Foreground(colorPurple).BorderForeground(colorNeonPink)

	l := list.New([]list.Item{}, del, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(true)
	l.DisableQuitKeybindings()

	return &Tab{
		Type:        TabTypeSearch,
		Title:       "Search",
		searchState: stateHome,
		textInput:   ti,
		list:        l,
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
		case tea.KeyCtrlC:
			return m, tea.Quit
		}

		// Handle global tab navigation
		switch msg.String() {
		case "tab":
			if len(m.tabs) > 1 {
				m.activeTab = (m.activeTab + 1) % len(m.tabs)
			}
			return m, nil
		case "shift+tab":
			if len(m.tabs) > 1 {
				m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			}
			return m, nil
		case "ctrl+w":
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
				t.readerView.Height = m.height - 9 // adjust for tab, borders, and status bar
			} else if t.Type == TabTypeSearch {
				t.list.SetSize(m.width-4, m.height-7)
				t.detailView.Width = m.width - 4
				t.detailView.Height = m.height - 9
			}
		}

	case searchResultMsg:
		if msg.tabIndex < len(m.tabs) {
			tab := m.tabs[msg.tabIndex]
			if msg.err != nil {
				tab.err = msg.err
				tab.searchState = stateHome
			} else {
				var items []list.Item
				for _, r := range msg.results {
					items = append(items, item{res: r})
				}
				tab.list.SetItems(items)
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

	// Route message to active tab
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
				} else if msg.Type == tea.KeyEsc {
					return m, tea.Quit
				}
			}
			tab.textInput, cmd = tab.textInput.Update(msg)
			cmds = append(cmds, cmd)

		case stateResults:
			isFiltering := tab.list.FilterState() == list.Filtering

			switch msg := msg.(type) {
			case tea.KeyMsg:
				if !isFiltering {
					switch msg.String() {
					case "esc", "s":
						tab.searchState = stateHome
						tab.textInput.SetValue("")
					case "enter":
						if selectedItem, ok := tab.list.SelectedItem().(item); ok {
							tab.searchState = stateDetail
							tab.detailRes = selectedItem.res
							tab.detailView = viewport.New(m.width-4, m.height-9)
							tab.detailView.SetContent(render.Result(tab.detailRes, tab.parsedQuery, tab.list.Index()+1, true))
							tab.detailView.GotoTop()
						}
					case "r":
						if selectedItem, ok := tab.list.SelectedItem().(item); ok {
							targetURL := selectedItem.res.Result.URL
							if targetURL != "" && strings.HasPrefix(targetURL, "http") {
								newTab := newReaderTab(targetURL)
								newTab.readerView = viewport.New(m.width-4, m.height-9)
								m.tabs = append(m.tabs, newTab)
								m.activeTab = len(m.tabs) - 1
								cmds = append(cmds, m.performFetch(m.activeTab, targetURL, m.width-8))
							}
						}
					}
				}
			}
			tab.list, cmd = tab.list.Update(msg)
			cmds = append(cmds, cmd)

		case stateDetail:
			switch msg := msg.(type) {
			case tea.KeyMsg:
				if msg.String() == "esc" {
					tab.searchState = stateResults
				}
			}
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
		q, err := query.Parse(qStr)
		if err != nil {
			return searchResultMsg{tabIndex: tabIndex, err: err}
		}
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

	// Create the header row consisting of the tabs
	headerRow := lipgloss.JoinHorizontal(lipgloss.Bottom, tabRow)

	var content string
	var statusLeft string
	var statusRight string

	tab := m.tabs[m.activeTab]

	if tab.err != nil {
		content = fmt.Sprintf("Error: %v\n\nPress Ctrl+W to close this tab.", tab.err)
		statusLeft = "ERROR"
		statusRight = "Ctrl+W: close"
	} else if tab.Type == TabTypeSearch {
		switch tab.searchState {
		case stateHome:
			// Center the ASCII and Input
			art := indieStyle.Render(asciiArt)
			input := searchBoxStyle.Render(tab.textInput.View())
			centeredBlock := lipgloss.JoinVertical(lipgloss.Center, art, "\n", input)

			// Try to vertically center if space allows
			padTop := (m.height - 15) / 2
			if padTop > 0 {
				centeredBlock = strings.Repeat("\n", padTop) + centeredBlock
			}

			content = lipgloss.PlaceHorizontal(m.width-4, lipgloss.Center, centeredBlock)
			statusLeft = "HOME"
			statusRight = "Enter: search • Esc: quit"

		case stateSearching:
			content = lipgloss.Place(m.width-4, m.height-7, lipgloss.Center, lipgloss.Center, fmt.Sprintf("Searching for %q...", tab.queryStr))
			statusLeft = "SEARCHING"

		case stateResults:
			content = tab.list.View()
			statusLeft = "RESULTS"
			if tab.list.FilterState() == list.Filtering {
				statusRight = "Enter/Esc: finish filter"
			} else {
				statusRight = "/: filter • Enter: view • r: read web • s/Esc: new search"
			}

		case stateDetail:
			header := readerHeaderStyle.Render(fmt.Sprintf(" %s ", tab.detailRes.Result.Title))
			content = fmt.Sprintf("%s\n\n%s", header, tab.detailView.View())
			statusLeft = "DETAIL"
			statusRight = "Esc: back to results • j/k: scroll"
		}
	} else if tab.Type == TabTypeReader {
		if tab.loading {
			content = lipgloss.Place(m.width-4, m.height-7, lipgloss.Center, lipgloss.Center, fmt.Sprintf("Fetching %s...\n\n(Downloading assets & formatting)", tab.url))
			statusLeft = "LOADING"
		} else {
			header := readerHeaderStyle.Render(fmt.Sprintf(" %s ", tab.url))
			content = fmt.Sprintf("%s\n\n%s", header, tab.readerView.View())
			statusLeft = "WEB"
			statusRight = fmt.Sprintf("j/k: scroll • ↓ %3.0f%%", tab.readerView.ScrollPercent()*100)
		}
	}

	contentBox := windowStyle.
		Width(m.width-2).
		Height(m.height-4). // Adjust to leave room for tabs and status bar
		Padding(1, 1).
		Render(content)

	// Global Status Bar
	sbLeft := statusBarText.Render(statusLeft)
	sbRight := statusBarStyle.Render(statusRight)
	sbMiddleWidth := m.width - lipgloss.Width(sbLeft) - lipgloss.Width(sbRight)
	if sbMiddleWidth < 0 {
		sbMiddleWidth = 0
	}
	sbMiddle := statusBarStyle.Render(strings.Repeat(" ", sbMiddleWidth))

	globalStatusBar := lipgloss.JoinHorizontal(lipgloss.Top, sbLeft, sbMiddle, sbRight)

	return lipgloss.JoinVertical(lipgloss.Left, headerRow, contentBox, globalStatusBar)
}
