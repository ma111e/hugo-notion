package tui

import (
	"context"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jomei/notionapi"
)

// KeyMap defines all keybindings
type KeyMap struct {
	Toggle key.Binding
	Quit   key.Binding
	Start  key.Binding
}

var DefaultKeyMap = KeyMap{
	Toggle: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "toggle selection"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q/esc", "quit"),
	),
	Start: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "start sync"),
	),
}

// Item represents a Notion page/database in the selection list
type Item struct {
	title    string
	id       string
	itemType string
	selected bool
}

func (i Item) Title() string {
	checkbox := "[ ]"
	if i.selected {
		checkbox = "[✓]"
	}
	return checkbox + " " + i.title
}

func (i Item) Description() string {
	return i.itemType
}

func (i Item) FilterValue() string {
	return i.title
}

// SelectionModel represents the selection screen
type SelectionModel struct {
	list     list.Model
	items    []Item
	selected map[string]bool
	client   *notionapi.Client
	pageID   string
	done     bool
	err      error
	keymap   KeyMap
	spinner  spinner.Model
	loading  bool
}

func NewSelectionModel(client *notionapi.Client, pageID string) SelectionModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	delegate := list.NewDefaultDelegate()
	items := []list.Item{}
	l := list.New(items, delegate, 0, 0)
	l.Title = "Select Pages to Sync"
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)

	return SelectionModel{
		list:     l,
		items:    []Item{},
		selected: make(map[string]bool),
		client:   client,
		pageID:   pageID,
		keymap:   DefaultKeyMap,
		spinner:  s,
		loading:  true,
	}
}

func (m SelectionModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchPages,
	)
}

func (m SelectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 4)
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var spinnerCmd tea.Cmd
			m.spinner, spinnerCmd = m.spinner.Update(msg)
			return m, spinnerCmd
		}
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil // Ignore keyboard input while loading
		}

		switch {
		case key.Matches(msg, m.keymap.Quit):
			m.done = true
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Start):
			if len(m.selected) > 0 {
				m.done = true
				return m, tea.Quit
			}
			return m, nil

		case key.Matches(msg, m.keymap.Toggle):
			if len(m.list.Items()) > 0 {
				i := m.list.SelectedItem().(Item)
				i.selected = !i.selected
				m.selected[i.id] = i.selected
				m.updateListItem(i)
			}
			return m, nil
		}

	case []Item:
		m.loading = false
		listItems := make([]list.Item, len(msg))
		for i, item := range msg {
			listItems[i] = item
		}
		m.items = msg
		m.list.SetItems(listItems)
		return m, nil

	case error:
		m.loading = false
		m.err = msg
		return m, nil
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m SelectionModel) View() string {
	if m.done {
		return ""
	}

	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left,
			"Loading pages from Notion...",
			m.spinner.View(),
		)
	}

	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Render("Error: " + m.err.Error())
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.list.View(),
		"\nspace: toggle selection • enter: start sync • q/esc: quit",
	)
}

func (m *SelectionModel) updateListItem(item Item) {
	for i, listItem := range m.list.Items() {
		if listItem.(Item).id == item.id {
			m.list.SetItem(i, item)
			break
		}
	}
}

func (m SelectionModel) fetchPages() tea.Msg {
	pagination := notionapi.Pagination{PageSize: 100}
	resp, err := m.client.Block.GetChildren(context.Background(), notionapi.BlockID(m.pageID), &pagination)
	if err != nil {
		return err
	}

	items := make([]Item, 0)
	for _, block := range resp.Results {
		switch b := block.(type) {
		case *notionapi.ChildPageBlock:
			items = append(items, Item{
				title:    b.ChildPage.Title,
				id:       string(b.ID),
				itemType: "page",
			})
		case *notionapi.ChildDatabaseBlock:
			items = append(items, Item{
				title:    b.ChildDatabase.Title,
				id:       string(b.ID),
				itemType: "database",
			})
		}
	}

	if len(items) == 0 {
		return error(nil)
	}

	return items
}

func (m SelectionModel) GetSelectedPages() []string {
	selected := make([]string, 0)
	for id, isSelected := range m.selected {
		if isSelected {
			selected = append(selected, id)
		}
	}
	return selected
}
