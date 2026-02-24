package main

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	beadslite "github.com/kylesnowschwartz/beads-lite"
)

// form is a modal overlay for creating or editing a card.
// On submit it returns a saveMsg to the board.
type form struct {
	title       textinput.Model
	focusTitle  bool // true = editing title, false = done
	editing     *beadslite.Issue
	columnIndex columnIndex
	width       int
	height      int
}

func newForm(colIdx columnIndex) form {
	ti := textinput.New()
	ti.Placeholder = "Card title..."
	ti.Focus()
	ti.CharLimit = 120
	ti.Width = 40

	return form{
		title:       ti,
		focusTitle:  true,
		columnIndex: colIdx,
	}
}

func editForm(issue *beadslite.Issue, colIdx columnIndex) form {
	ti := textinput.New()
	ti.SetValue(issue.Title)
	ti.Focus()
	ti.CharLimit = 120
	ti.Width = 40

	return form{
		title:       ti,
		focusTitle:  true,
		editing:     issue,
		columnIndex: colIdx,
	}
}

func (f form) Init() tea.Cmd {
	return textinput.Blink
}

func (f form) Update(msg tea.Msg) (form, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Back):
			// Cancel: return empty saveMsg to signal cancellation
			return f, nil
		case msg.Type == tea.KeyEnter:
			return f, f.submit()
		}
	case tea.WindowSizeMsg:
		f.width = msg.Width
		f.height = msg.Height
	}

	var cmd tea.Cmd
	f.title, cmd = f.title.Update(msg)
	return f, cmd
}

func (f form) View() string {
	header := "New Card"
	if f.editing != nil {
		header = "Edit Card"
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(50)

	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(header),
		"",
		f.title.View(),
		"",
		lipgloss.NewStyle().Faint(true).Render("enter: save  esc: cancel"),
	)

	rendered := style.Render(content)

	// Center the form in the terminal
	return lipgloss.Place(f.width, f.height,
		lipgloss.Center, lipgloss.Center,
		rendered,
	)
}

// submit creates the appropriate issue and returns a saveMsg.
func (f form) submit() tea.Cmd {
	title := f.title.Value()
	if title == "" {
		return nil
	}

	return func() tea.Msg {
		if f.editing != nil {
			// Edit existing
			f.editing.Title = title
			return saveMsg{issue: f.editing}
		}
		// Create new
		issue := beadslite.NewIssue(title)
		issue.Status = columnToStatus[f.columnIndex]
		return saveMsg{issue: issue}
	}
}
