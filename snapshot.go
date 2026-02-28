package main

import (
	"encoding/json"
	"io"

	beadslite "github.com/kylesnowschwartz/beads-lite"
)

// snapshotOutput is the complete board snapshot. No rendered view is embedded —
// use --format ascii for that. The total field counts cards across all columns.
type snapshotOutput struct {
	Columns []exportColumn `json:"columns"`
	Total   int            `json:"total"`
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

	columns := make([]exportColumn, numColumns)
	total := 0
	for i := columnIndex(0); i < numColumns; i++ {
		cards := make([]exportCard, 0, len(buckets[i]))
		for _, item := range buckets[i] {
			c, ok := item.(card)
			if !ok {
				continue
			}
			cards = append(cards, exportCard{
				ID:       c.issue.ID,
				Title:    c.issue.Title,
				Status:   string(c.issue.Status),
				Priority: c.issue.Priority,
				Type:     string(c.issue.Type),
				Assignee: c.issue.AssignedTo,
			})
		}
		columns[i] = exportColumn{
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
func writeSnapshotASCII(store *beadslite.Store, width, height int, w io.Writer) error {
	b, err := newBoardForExport(store, width, height)
	if err != nil {
		return err
	}

	_, err = io.WriteString(w, b.View().Content+"\n")
	return err
}
