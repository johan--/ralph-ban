package main

import (
	"encoding/json"
	"io"

	beadslite "github.com/kylesnowschwartz/beads-lite"
)

// snapshotCard is the JSON representation of a card in a snapshot.
// Includes assignee so consumers can see ownership at a glance.
type snapshotCard struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority int    `json:"priority"`
	Type     string `json:"type"`
	Assignee string `json:"assignee,omitempty"`
}

// snapshotColumn is the JSON representation of one kanban column,
// including the card list and current WIP count.
type snapshotColumn struct {
	Title    string         `json:"title"`
	WIP      int            `json:"wip"`
	Cards    []snapshotCard `json:"cards"`
}

// snapshotOutput is the complete board snapshot. No rendered view is embedded —
// use --format ascii for that. The total field counts cards across all columns.
type snapshotOutput struct {
	Columns []snapshotColumn `json:"columns"`
	Total   int              `json:"total"`
}

// writeSnapshot queries the store and writes structured JSON to w.
// No terminal simulation is needed — this is a pure data export.
func writeSnapshot(store *beadslite.Store, w io.Writer) error {
	issues, err := store.ListIssues()
	if err != nil {
		return err
	}

	// Partition issues into column buckets (no blocked-ID computation needed for snapshot).
	buckets := partitionByStatus(issues, nil)

	columns := make([]snapshotColumn, numColumns)
	total := 0
	for i := columnIndex(0); i < numColumns; i++ {
		cards := make([]snapshotCard, 0, len(buckets[i]))
		for _, item := range buckets[i] {
			c, ok := item.(card)
			if !ok {
				continue
			}
			cards = append(cards, snapshotCard{
				ID:       c.issue.ID,
				Title:    c.issue.Title,
				Status:   string(c.issue.Status),
				Priority: c.issue.Priority,
				Type:     string(c.issue.Type),
				Assignee: c.issue.AssignedTo,
			})
		}
		columns[i] = snapshotColumn{
			Title: columnTitles[i],
			WIP:   len(cards),
			Cards: cards,
		}
		total += len(cards)
	}

	out := snapshotOutput{
		Columns: columns,
		Total:   total,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// writeSnapshotASCII renders the board as plain text (ANSI stripped) and writes it to w.
// Reuses dumpBoard but extracts only the view field.
func writeSnapshotASCII(store *beadslite.Store, width, height int, w io.Writer) error {
	b := newBoard(store)

	msg := fetchRefresh(store)
	refresh, ok := msg.(refreshMsg)
	if !ok {
		if e, isErr := msg.(errMsg); isErr {
			return e.err
		}
		return nil
	}

	b.termWidth = width
	b.termHeight = height
	b.help.Width = width
	b.loaded = true
	b.applyRefresh(refresh)
	b.cols[b.focused].Focus()
	b.updatePan()
	b.resizeColumns()

	_, err := io.WriteString(w, b.View()+"\n")
	return err
}
