package main

import beadslite "github.com/kylesnowschwartz/beads-lite"

// card wraps a beads-lite Issue for display in a bubbles/list.
// Implements the list.Item and list.DefaultItem interfaces.
type card struct {
	issue *beadslite.Issue
}

func (c card) Title() string       { return c.issue.Title }
func (c card) Description() string { return c.issue.ID }
func (c card) FilterValue() string { return c.issue.Title }
