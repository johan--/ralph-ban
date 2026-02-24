package main

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	beadslite "github.com/kylesnowschwartz/beads-lite"
)

// board is the root tea.Model for the kanban TUI.
type board struct {
	store    *beadslite.Store
	cols     [numColumns]column
	focused  columnIndex
	help     help.Model
	loaded   bool
	quitting bool
	err      error

	// Form state: when non-nil, the form overlay is active.
	form     *form
	formMode bool

	// Layout panning
	termWidth  int
	termHeight int
	panOffset  int // index of first visible column
}

func newBoard(store *beadslite.Store) *board {
	var cols [numColumns]column
	for i := columnIndex(0); i < numColumns; i++ {
		cols[i] = newColumn(i)
	}

	b := &board{
		store: store,
		cols:  cols,
		help:  help.New(),
	}
	return b
}

func (b *board) Init() tea.Cmd {
	return tea.Batch(
		b.loadFromStore(),
		tickRefresh(b.store),
	)
}

func (b *board) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If form is active, route all input there
	if b.formMode {
		return b.updateForm(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.termWidth = msg.Width
		b.termHeight = msg.Height
		b.loaded = true
		b.updatePan()
		b.resizeColumns()
		return b, nil

	case refreshMsg:
		b.applyRefresh(msg.issues)
		return b, tickRefresh(b.store)

	case moveMsg:
		return b, b.handleMove(msg)

	case deleteMsg:
		return b, persistDelete(b.store, msg.id)

	case saveMsg:
		return b, b.handleSave(msg)

	case errMsg:
		b.err = msg.err
		return b, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			b.quitting = true
			return b, tea.Quit

		case key.Matches(msg, keys.Left):
			b.moveFocus(-1)
			return b, nil

		case key.Matches(msg, keys.Right):
			b.moveFocus(1)
			return b, nil

		case key.Matches(msg, keys.New):
			b.openNewForm()
			return b, textinputBlink()

		case key.Matches(msg, keys.Edit):
			b.openEditForm()
			return b, textinputBlink()

		case key.Matches(msg, keys.Help):
			b.help.ShowAll = !b.help.ShowAll
			return b, nil
		}
	}

	// Forward remaining messages to the focused column
	cmd := b.cols[b.focused].Update(msg)
	return b, cmd
}

func (b *board) View() string {
	if b.quitting {
		return ""
	}
	if !b.loaded {
		return "Loading..."
	}
	if b.formMode && b.form != nil {
		return b.form.View()
	}

	// Build visible columns based on panning
	visible := b.visibleCount()
	var views []string
	for i := 0; i < visible && b.panOffset+i < int(numColumns); i++ {
		idx := b.panOffset + i
		views = append(views, b.cols[idx].View())
	}

	boardView := lipgloss.JoinHorizontal(lipgloss.Top, views...)

	// Position indicator
	indicator := b.positionIndicator()

	// Error display
	var errView string
	if b.err != nil {
		errView = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render("Error: " + b.err.Error())
	}

	helpView := b.help.View(keys)

	return lipgloss.JoinVertical(lipgloss.Left,
		boardView,
		indicator,
		errView,
		helpView,
	)
}

// loadFromStore returns a command that loads all issues and sets up column items.
func (b *board) loadFromStore() tea.Cmd {
	return func() tea.Msg {
		issues, err := b.store.ListIssues()
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{issues: issues}
	}
}

// applyRefresh partitions issues by status and updates column lists.
func (b *board) applyRefresh(issues []*beadslite.Issue) {
	buckets := partitionByStatus(issues)
	for i := columnIndex(0); i < numColumns; i++ {
		items := buckets[i]
		if items == nil {
			items = []list.Item{}
		}
		b.cols[i].SetItems(items)
	}
}

// handleMove inserts a card into the target column, shifts focus to follow it,
// and persists the status change.
func (b *board) handleMove(msg moveMsg) tea.Cmd {
	target := msg.target
	if target < 0 || target >= numColumns {
		return nil
	}

	msg.card.issue.Status = columnToStatus[target]

	// Add to target column
	items := b.cols[target].list.Items()
	items = append(items, msg.card)
	b.cols[target].SetItems(items)

	// Follow the card: shift focus to the target column and select it
	b.cols[b.focused].Blur()
	b.focused = target
	b.cols[b.focused].Focus()
	b.cols[target].list.Select(len(items) - 1)
	b.updatePan()
	b.resizeColumns()

	return persistMove(b.store, msg.card.issue.ID, target)
}

// handleSave processes a form submission (create or edit).
func (b *board) handleSave(msg saveMsg) tea.Cmd {
	b.formMode = false
	b.form = nil

	if msg.issue == nil {
		return nil
	}

	if msg.issue.ID == "" {
		// This shouldn't happen — NewIssue always sets an ID
		return nil
	}

	// Check if this is an edit (issue already exists in a column)
	for i := columnIndex(0); i < numColumns; i++ {
		for j, item := range b.cols[i].list.Items() {
			if c, ok := item.(card); ok && c.issue.ID == msg.issue.ID {
				// Update in place
				b.cols[i].list.SetItem(j, card{issue: msg.issue})
				return persistUpdate(b.store, msg.issue)
			}
		}
	}

	// New card: add to the appropriate column
	col := statusToColumn[msg.issue.Status]
	items := b.cols[col].list.Items()
	items = append(items, card{issue: msg.issue})
	b.cols[col].SetItems(items)
	return persistCreate(b.store, msg.issue)
}

// moveFocus shifts focus by delta columns (-1 or +1).
func (b *board) moveFocus(delta int) {
	next := int(b.focused) + delta
	if next < 0 || next >= int(numColumns) {
		return
	}

	b.cols[b.focused].Blur()
	b.focused = columnIndex(next)
	b.cols[b.focused].Focus()
	b.updatePan()
	b.resizeColumns()
}

// openNewForm switches to form mode for creating a new card.
func (b *board) openNewForm() {
	f := newForm(b.focused)
	f.width = b.termWidth
	f.height = b.termHeight
	b.form = &f
	b.formMode = true
}

// openEditForm switches to form mode for editing the selected card.
func (b *board) openEditForm() {
	cd, ok := b.cols[b.focused].SelectedCard()
	if !ok {
		return
	}
	f := editForm(cd.issue, b.focused)
	f.width = b.termWidth
	f.height = b.termHeight
	b.form = &f
	b.formMode = true
}

// updateForm routes messages to the form overlay.
func (b *board) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Back) {
			b.formMode = false
			b.form = nil
			return b, nil
		}
	case tea.WindowSizeMsg:
		b.termWidth = msg.Width
		b.termHeight = msg.Height
		if b.form != nil {
			b.form.width = msg.Width
			b.form.height = msg.Height
		}
		return b, nil
	case refreshMsg:
		// Still apply refreshes while form is open
		b.applyRefresh(msg.issues)
		return b, tickRefresh(b.store)
	case saveMsg:
		return b, b.handleSave(msg)
	}

	if b.form != nil {
		f, cmd := b.form.Update(msg)
		b.form = &f
		return b, cmd
	}
	return b, nil
}

// Layout panning

const minColumnWidth = 24

func (b *board) visibleCount() int {
	if b.termWidth == 0 {
		return int(numColumns)
	}
	count := b.termWidth / minColumnWidth
	if count < 1 {
		count = 1
	}
	if count > int(numColumns) {
		count = int(numColumns)
	}
	return count
}

// updatePan adjusts panOffset so the focused column is visible.
func (b *board) updatePan() {
	visible := b.visibleCount()
	focusIdx := int(b.focused)

	if focusIdx < b.panOffset {
		b.panOffset = focusIdx
	}
	if focusIdx >= b.panOffset+visible {
		b.panOffset = focusIdx - visible + 1
	}
	// Clamp
	maxOffset := int(numColumns) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if b.panOffset > maxOffset {
		b.panOffset = maxOffset
	}
}

// resizeColumns distributes terminal width evenly among visible columns.
func (b *board) resizeColumns() {
	visible := b.visibleCount()
	if visible == 0 {
		return
	}

	// Reserve space for help bar and position indicator
	colHeight := b.termHeight - 4
	if colHeight < 5 {
		colHeight = 5
	}

	// Each column's border (visible or hidden) adds 2 chars (left + right).
	// Subtract that so the total rendered width fits within termWidth.
	const borderWidth = 2
	colWidth := (b.termWidth / visible) - borderWidth

	for i := 0; i < visible && b.panOffset+i < int(numColumns); i++ {
		idx := b.panOffset + i
		b.cols[idx].SetSize(colWidth, colHeight)
	}
}

// positionIndicator shows which columns are visible: [< Backlog | *To Do* | Doing >]
func (b *board) positionIndicator() string {
	visible := b.visibleCount()
	if visible >= int(numColumns) {
		return "" // all visible, no indicator needed
	}

	var parts []string
	if b.panOffset > 0 {
		parts = append(parts, "<")
	} else {
		parts = append(parts, " ")
	}

	for i := 0; i < visible && b.panOffset+i < int(numColumns); i++ {
		idx := b.panOffset + i
		name := columnTitles[idx]
		if columnIndex(idx) == b.focused {
			name = "*" + name + "*"
		}
		parts = append(parts, name)
	}

	if b.panOffset+visible < int(numColumns) {
		parts = append(parts, ">")
	} else {
		parts = append(parts, " ")
	}

	indicator := ""
	for i, p := range parts {
		if i > 0 && i < len(parts)-1 {
			indicator += " | "
		}
		indicator += p
	}

	return lipgloss.NewStyle().
		Faint(true).
		Width(b.termWidth).
		Align(lipgloss.Center).
		Render("[" + indicator + "]")
}

// textinputBlink returns a command to start the text input cursor blinking.
func textinputBlink() tea.Cmd {
	return func() tea.Msg {
		return nil
	}
}
