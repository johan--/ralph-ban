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

## Agent Roles

Three agent types coordinate work through the kanban board:

- **Orchestrator** (`agents/orchestrator.md`) — Coordinates the pipeline. Assesses
  the board, spawns workers and reviewers, monitors progress, gates merges behind
  human approval. Never implements or reviews code directly.
- **Worker** (`agents/worker.md`) — Implements a single card in an isolated worktree.
  Runs tests, commits, moves card to review, reports back.
- **Reviewer** (`agents/reviewer.md`) — Reviews a single card in an isolated worktree.
  Runs tests, checks against review checklist, reports approve/reject.

### Workflow phases

```
ASSESS -> SPAWN -> MONITOR -> REVIEW -> HUMAN APPROVAL -> MERGE
```

The orchestrator drives this pipeline. Workers and reviewers are spawned into
isolated worktrees for parallel execution. Nothing merges to main without
explicit human approval.

### Status flow

```
Backlog -> To Do -> Doing -> Review -> Done
```

Cards move right as work progresses. The orchestrator owns status transitions
and card closure. Workers move cards to Review; only the orchestrator closes them.

## Hooks

Six hooks inject board state and enforce workflow gates:
- **SessionStart** — Board snapshot, suggests highest-priority task
- **UserPromptSubmit** — Diffs board since last prompt, review queue nudges, stall detection
- **Stop** — Blocks exit on uncommitted changes (all agents), claimed cards, active work (orchestrator)
- **TeammateIdle** — Prevents workers from going idle while they own active cards
- **TaskCompleted** — Blocks task completion if worker's cards are still in doing
- **PreCompact** — Re-injects board state before context compression

## Agent Frontmatter

Workers and reviewers have `maxTurns` and `permissionMode: bypassPermissions` set in their YAML frontmatter. Claude Code enforces these natively — no CLI flags needed.

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
