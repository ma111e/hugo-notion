package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/ma111e/hugo-notion/internal/sync"
	"os"
	"strings"
	"time"
)

type syncModel struct {
	table     table.Model
	results   []sync.SyncResult
	quitting  bool
	lastSync  time.Time
	isLoading bool
	spinner   spinner.Model
	styles    statusStyles
}

type statusStyles struct {
	created lipgloss.Style
	updated lipgloss.Style
	skipped lipgloss.Style
	error   lipgloss.Style
	deleted lipgloss.Style
}

func NewSyncModel() syncModel {
	width, _, _ := term.GetSize(os.Stdout.Fd())
	if width == 0 {
		width = 120
	}

	pathWidth := width - 65
	if pathWidth < 40 {
		pathWidth = 40
	}

	columns := []table.Column{
		{Title: "Page", Width: 30},
		{Title: "Status", Width: 10},
		{Title: "Path", Width: pathWidth},
		{Title: "Last Updated", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	t.SetStyles(s)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Define status styles
	styles := statusStyles{
		created: lipgloss.NewStyle().Foreground(lipgloss.Color("2")),  // Cyan
		updated: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),  // Green
		skipped: lipgloss.NewStyle().Foreground(lipgloss.Color("3")),  // Yellow
		error:   lipgloss.NewStyle().Foreground(lipgloss.Color("1")),  // Red
		deleted: lipgloss.NewStyle().Foreground(lipgloss.Color("13")), // Purple
	}

	return syncModel{
		table:     t,
		results:   make([]sync.SyncResult, 0),
		lastSync:  time.Now(),
		isLoading: true,
		spinner:   sp,
		styles:    styles,
	}
}

func (m syncModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m syncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.isLoading {
			var spinnerCmd tea.Cmd
			m.spinner, spinnerCmd = m.spinner.Update(msg)
			return m, spinnerCmd
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

	case []sync.SyncResult:
		m.results = msg
		m.isLoading = false
		m.lastSync = time.Now()
		m.updateTable()
		return m, tea.Quit
	}

	return m, cmd
}

func (m syncModel) View() string {
	var s strings.Builder
	s.WriteString("\n 📝 Notion Sync Status\n\n")

	if m.isLoading && len(m.results) == 0 {
		s.WriteString(fmt.Sprintf("%s Syncing Notion pages...\n", m.spinner.View()))
	} else {
		s.WriteString(m.table.View())
		s.WriteString("\n\nStatus Legend:\n")
		s.WriteString(fmt.Sprintf("  %s Created: New page added\n", m.styles.created.Render("●")))
		s.WriteString(fmt.Sprintf("  %s Updated: Page content changed\n", m.styles.updated.Render("●")))
		s.WriteString(fmt.Sprintf("  %s Skipped: Page content unchanged\n", m.styles.skipped.Render("●")))
		s.WriteString(fmt.Sprintf("  %s Error: Failed to process page\n", m.styles.error.Render("●")))
		s.WriteString(fmt.Sprintf("  %s Deleted: Page removed\n", m.styles.deleted.Render("●")))
		s.WriteString(fmt.Sprintf("\nLast sync: %s", m.lastSync.Format("15:04:05")))
		if m.isLoading {
			s.WriteString(fmt.Sprintf("\n%s Still syncing...", m.spinner.View()))
		}
	}

	s.WriteString("\n")
	return s.String()
}

func (m *syncModel) updateTable() {
	rows := make([]table.Row, len(m.results))
	for i, r := range m.results {
		// Get the appropriate style for the status
		var style lipgloss.Style

		switch strings.ToLower(r.Status) {
		case "created":
			style = m.styles.created
			style.Bold(true)
		case "updated":
			style = m.styles.updated
			style.Bold(true)
		case "skipped":
			style = m.styles.skipped
		case "error", "delete error":
			style = m.styles.error
		case "deleted":
			style = m.styles.deleted
			style.Bold(true)

		default:
			style = lipgloss.NewStyle()
		}

		// Apply the style to both the page title and status
		styledTitle := style.Render(r.PageTitle)
		styledStatus := style.Render(r.Status)

		rows[i] = table.Row{
			styledTitle,
			styledStatus,
			r.Path,
			r.LastUpdated.Format("2006-01-02 15:04:05"),
		}
	}
	m.table.SetRows(rows)
}
