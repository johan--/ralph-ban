package main

import (
	"fmt"

	beadslite "github.com/kylesnowschwartz/beads-lite"
)

// card wraps a beads-lite Issue for display in a bubbles/list.
// Implements the list.Item and list.DefaultItem interfaces.
type card struct {
	issue   *beadslite.Issue
	blocked bool // true when this card has at least one unresolved blocker
}

func (c card) Title() string       { return c.issue.Title }
func (c card) FilterValue() string { return c.issue.Title }

// Description shows priority, type, ID, and assignee (if claimed) on the second line.
// Blocked cards get a lock prefix so the constraint is visible without opening the detail.
func (c card) Description() string {
	base := fmt.Sprintf("P%d %s · %s", c.issue.Priority, c.issue.Type, c.issue.ID)
	if c.issue.AssignedTo != "" {
		base += " @" + c.issue.AssignedTo
	}
	if c.blocked {
		return "[locked] " + base
	}
	return base
}
