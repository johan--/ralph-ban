# ralph-ban

Go TUI kanban board backed by beads-lite's SQLite database.

## Architecture

- `board.go` — Root tea.Model, handles focus, layout panning, message routing
- `column.go` — Wraps bubbles/list per kanban column (Backlog, To Do, Doing, Review, Done)
- `card.go` — Adapts beads-lite Issue to list.Item
- `form.go` — Modal overlay for create/edit
- `store.go` — SQLite persistence commands, refresh polling
- `keys.go` — Vim-style keybindings
- `messages.go` — Message types for decoupled model communication

## Board-Driven Workflow

Hooks inject board state into your context automatically:
- **SessionStart** — Shows current board, suggests highest-priority task
- **UserPromptSubmit** — Diffs board since last prompt, reports card movements
- **Stop** — Blocks exit while Todo/Doing items remain

### Working with the board

1. Check the board context injected by hooks at session start
2. When starting work on a card: `bl claim <id> --agent claude`
3. When work is done: `bl update <id> --status review`
4. If blocked, note it and pick up the next Todo item
5. Don't stop while Todo/Doing cards exist unless the user says to

### Status flow

```
Backlog -> To Do -> Doing -> Review -> Done
```

## Development

This project uses a go.work workspace with `../beads-lite`. Changes to beads-lite types are immediately available.

```
go build ./...    # build
go run .          # run TUI (requires bl init first)
```

### Dependencies

- [bubbletea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [bubbles](https://github.com/charmbracelet/bubbles) — TUI components
- [lipgloss](https://github.com/charmbracelet/lipgloss) — TUI styling
- [beads-lite](https://github.com/kylesnowschwartz/beads-lite) — SQLite task tracker
