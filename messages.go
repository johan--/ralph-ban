package main

import beadslite "github.com/kylesnowschwartz/beads-lite"

// moveMsg signals that a card should move to a different column.
// The board intercepts this and routes it to the target column.
type moveMsg struct {
	card   card
	source columnIndex
	target columnIndex
}

// saveMsg carries a created or edited issue back from the form to the board.
type saveMsg struct {
	issue *beadslite.Issue
}

// deleteMsg requests deletion of a card from the current column.
type deleteMsg struct {
	id string
}

// priorityMsg signals a priority change on the selected card.
type priorityMsg struct {
	card  card
	delta int // -1 = higher priority (toward P0), +1 = lower (toward P4)
}

// errMsg carries an error from async operations (persistence, refresh).
type errMsg struct {
	err error
}

// refreshMsg carries fresh issue data from a periodic SQLite poll.
// blockedIDs is the set of issue IDs that have at least one unresolved blocker —
// i.e. they depend on an issue that is not yet done.
type refreshMsg struct {
	issues     []*beadslite.Issue
	blockedIDs map[string]bool
}

// closeMsg carries a card closure request from the resolution picker to the board.
// The resolution is chosen by the user before the move to Done is finalized.
type closeMsg struct {
	card       card
	source     columnIndex
	resolution beadslite.Resolution
}
