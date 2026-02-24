package main

import (
	"testing"

	beadslite "github.com/kylesnowschwartz/beads-lite"
)

func TestCardImplementsListItem(t *testing.T) {
	issue := &beadslite.Issue{
		ID:    "bl-test",
		Title: "Test Card",
	}
	c := card{issue: issue}

	if c.Title() != "Test Card" {
		t.Errorf("Title() = %q, want %q", c.Title(), "Test Card")
	}
	if c.Description() != "bl-test" {
		t.Errorf("Description() = %q, want %q", c.Description(), "bl-test")
	}
	if c.FilterValue() != "Test Card" {
		t.Errorf("FilterValue() = %q, want %q", c.FilterValue(), "Test Card")
	}
}
