package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	beadslite "github.com/kylesnowschwartz/beads-lite"
)

// label returns a human-readable description of the filter, e.g. "priority=P1".
// Test-only helper for assertions.
func (f activeFilter) label() string {
	if f.field == filterNone {
		return "none"
	}
	return fmt.Sprintf("%s=%s", f.field, f.value)
}

func makeFilterIssue(id string, priority int, issueType beadslite.IssueType, assignedTo string) *beadslite.Issue {
	return &beadslite.Issue{
		ID:         id,
		Title:      "Test " + id,
		Priority:   priority,
		Type:       issueType,
		AssignedTo: assignedTo,
	}
}

func TestActiveFilterLabel(t *testing.T) {
	tests := []struct {
		filter activeFilter
		want   string
	}{
		{activeFilter{field: filterNone}, "none"},
		{activeFilter{field: filterPriority, value: "P0"}, "priority=P0"},
		{activeFilter{field: filterType, value: "bug"}, "type=bug"},
		{activeFilter{field: filterAssignee, value: "alice"}, "assignee=alice"},
	}
	for _, tt := range tests {
		got := tt.filter.label()
		if got != tt.want {
			t.Errorf("label() = %q, want %q", got, tt.want)
		}
	}
}

func TestActiveFilterMatches(t *testing.T) {
	issue := makeFilterIssue("x1", 2, beadslite.IssueTypeBug, "alice")

	tests := []struct {
		name   string
		filter activeFilter
		want   bool
	}{
		{"no filter passes everything", activeFilter{field: filterNone}, true},
		{"matching priority", activeFilter{field: filterPriority, value: "P2"}, true},
		{"non-matching priority", activeFilter{field: filterPriority, value: "P0"}, false},
		{"matching type", activeFilter{field: filterType, value: "bug"}, true},
		{"non-matching type", activeFilter{field: filterType, value: "task"}, false},
		{"matching assignee", activeFilter{field: filterAssignee, value: "alice"}, true},
		{"non-matching assignee", activeFilter{field: filterAssignee, value: "bob"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.matches(issue)
			if got != tt.want {
				t.Errorf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextFilterCycles(t *testing.T) {
	issues := []*beadslite.Issue{
		makeFilterIssue("a", 1, beadslite.IssueTypeTask, "alice"),
		makeFilterIssue("b", 2, beadslite.IssueTypeBug, ""),
	}

	// Start at no filter, cycle forward until we return to no filter.
	start := activeFilter{field: filterNone}
	f := start
	visited := map[string]int{}
	maxSteps := 20

	for i := 0; i < maxSteps; i++ {
		f = nextFilter(f, issues)
		label := f.label()
		visited[label]++
		if visited[label] > 1 {
			t.Fatalf("cycle revisited %q at step %d", label, i)
		}
		if f.field == filterNone {
			// Completed one full cycle
			return
		}
	}
	t.Fatalf("did not return to no-filter after %d steps", maxSteps)
}

func TestNextFilterSkipsMissingData(t *testing.T) {
	// No P0 issues — P0 filter step should not appear.
	issues := []*beadslite.Issue{
		makeFilterIssue("a", 3, beadslite.IssueTypeTask, ""),
	}
	steps := buildFilterSteps(issues)
	for _, s := range steps {
		if s.field == filterPriority && s.value == "P0" {
			t.Errorf("P0 filter should not appear when no P0 issues exist")
		}
	}
}

func TestPrevFilter(t *testing.T) {
	issues := []*beadslite.Issue{
		makeFilterIssue("a", 1, beadslite.IssueTypeTask, ""),
	}
	steps := buildFilterSteps(issues)
	if len(steps) < 2 {
		t.Skip("not enough steps to test prevFilter")
	}

	// Going next then prev should return to the start.
	start := activeFilter{field: filterNone}
	next := nextFilter(start, issues)
	back := prevFilter(next, issues)
	if back.field != start.field || back.value != start.value {
		t.Errorf("prevFilter after nextFilter = %v, want %v", back, start)
	}

	// Same for second step.
	second := nextFilter(next, issues)
	backToNext := prevFilter(second, issues)
	if backToNext.field != next.field || backToNext.value != next.value {
		t.Errorf("prevFilter from second step = %v, want %v", backToNext, next)
	}
}

func TestApplyFilterToItems(t *testing.T) {
	p1bug := card{issue: makeFilterIssue("a", 1, beadslite.IssueTypeBug, "")}
	p2task := card{issue: makeFilterIssue("b", 2, beadslite.IssueTypeTask, "alice")}
	items := []list.Item{p1bug, p2task}

	t.Run("no filter returns all", func(t *testing.T) {
		out := applyFilterToItems(items, activeFilter{field: filterNone})
		if len(out) != 2 {
			t.Errorf("got %d items, want 2", len(out))
		}
	})

	t.Run("priority filter", func(t *testing.T) {
		out := applyFilterToItems(items, activeFilter{field: filterPriority, value: "P1"})
		if len(out) != 1 {
			t.Errorf("got %d items, want 1", len(out))
		}
	})

	t.Run("type filter", func(t *testing.T) {
		out := applyFilterToItems(items, activeFilter{field: filterType, value: "bug"})
		if len(out) != 1 {
			t.Errorf("got %d items, want 1", len(out))
		}
	})

	t.Run("assignee filter", func(t *testing.T) {
		out := applyFilterToItems(items, activeFilter{field: filterAssignee, value: "alice"})
		if len(out) != 1 {
			t.Errorf("got %d items, want 1", len(out))
		}
	})

	t.Run("no match returns empty slice", func(t *testing.T) {
		out := applyFilterToItems(items, activeFilter{field: filterAssignee, value: "nobody"})
		if out == nil {
			t.Error("got nil, want empty slice")
		}
		if len(out) != 0 {
			t.Errorf("got %d items, want 0", len(out))
		}
	})
}

func TestFilterStepLabel(t *testing.T) {
	tests := []struct {
		filter activeFilter
		want   string
	}{
		{activeFilter{field: filterNone}, "all"},
		{activeFilter{field: filterPriority, value: "P1"}, "P1"},
		{activeFilter{field: filterType, value: "bug"}, "bug"},
		{activeFilter{field: filterAssignee, value: "alice"}, "alice"},
	}
	for _, tt := range tests {
		got := filterStepLabel(tt.filter)
		if got != tt.want {
			t.Errorf("filterStepLabel(%v) = %q, want %q", tt.filter, got, tt.want)
		}
	}
}

func TestFilterCycleViewContainsActive(t *testing.T) {
	issues := []*beadslite.Issue{
		makeFilterIssue("a", 1, beadslite.IssueTypeTask, ""),
		makeFilterIssue("b", 2, beadslite.IssueTypeBug, ""),
		makeFilterIssue("c", 3, beadslite.IssueTypeFeature, "alice"),
	}

	t.Run("no filter shows all label in brackets", func(t *testing.T) {
		view := filterCycleView(activeFilter{field: filterNone}, issues, 7)
		if !containsString(view, "[all]") {
			t.Errorf("expected [all] in view, got: %q", stripAnsi(view))
		}
	})

	t.Run("priority filter shows bracketed value", func(t *testing.T) {
		f := activeFilter{field: filterPriority, value: "P1"}
		view := filterCycleView(f, issues, 7)
		if !containsString(view, "[P1]") {
			t.Errorf("expected [P1] in view, got: %q", stripAnsi(view))
		}
	})

	t.Run("shows hint text", func(t *testing.T) {
		view := filterCycleView(activeFilter{field: filterNone}, issues, 7)
		if !containsString(view, "f/F cycle") {
			t.Errorf("expected hint text in view, got: %q", stripAnsi(view))
		}
	})

	t.Run("shows ellipsis when window is smaller than step count", func(t *testing.T) {
		// With 7 issues to get many steps and maxVisible=3, ellipsis should appear
		manyIssues := []*beadslite.Issue{
			makeFilterIssue("a", 0, beadslite.IssueTypeTask, ""),
			makeFilterIssue("b", 1, beadslite.IssueTypeBug, ""),
			makeFilterIssue("c", 2, beadslite.IssueTypeFeature, ""),
			makeFilterIssue("d", 3, beadslite.IssueTypeEpic, "alice"),
		}
		// Active at P3 (last priority step) so there's content to the left
		f := activeFilter{field: filterPriority, value: "P3"}
		view := filterCycleView(f, manyIssues, 3)
		if !containsString(view, "…") {
			t.Errorf("expected ellipsis in view when window is truncated, got: %q", stripAnsi(view))
		}
	})
}

// containsString checks whether s contains substr, ignoring ANSI escape sequences.
func containsString(s, substr string) bool {
	return strings.Contains(s, substr) || strings.Contains(stripAnsi(s), substr)
}

// stripAnsi removes ANSI escape sequences from s for plain-text comparison in tests.
func stripAnsi(s string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
