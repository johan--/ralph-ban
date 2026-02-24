# ralph-ban: TUI kanban board backed by beads-lite

bin       := "ralph-ban"
bl_bin    := "/tmp/bl-test"
bl_src    := "../beads-lite"

# List available recipes
default:
    @just --list

# Build ralph-ban
build:
    go build -o {{ bin }} .

# Build the beads-lite CLI
build-bl:
    cd {{ bl_src }} && go build -o {{ bl_bin }} ./cmd/bl

# Build both binaries
build-all: build build-bl

# Run Go unit tests
test:
    go test ./... -count=1

# Run Go tests for beads-lite
test-bl:
    cd {{ bl_src }} && go test ./... -count=1

# Run CLI integration tests
test-cli: build-all
    bash test_cli_integration.sh

# Run hook script tests
test-hooks: build-bl
    bash test_hooks.sh

# Run all tests
test-all: test test-cli test-hooks

# Dump the TUI as JSON (no TTY needed)
dump *args: build
    ./{{ bin }} --dump {{ args }}

# Dump at a specific width (e.g. just dump-at 80)
dump-at width="120" height="40": build
    ./{{ bin }} --dump --width {{ width }} --height {{ height }}

# Dump and pretty-print the board structure
dump-board: build
    ./{{ bin }} --dump | jq '{focus, pan_offset, columns: [.columns[] | {title, cards: [.cards[] | {id, title, status}]}]}'

# Dump and display the rendered view text
dump-view width="120" height="40": build
    ./{{ bin }} --dump --width {{ width }} --height {{ height }} | jq -r '.view'

# Launch the interactive TUI
run: build
    ./{{ bin }}

# Create a scratch board in a temp dir and launch the TUI
scratch: build-all
    #!/usr/bin/env bash
    set -euo pipefail
    dir=$(mktemp -d)
    cd "$dir"
    {{ bl_bin }} init
    {{ bl_bin }} create "Example task" --priority 1 --type feature
    {{ bl_bin }} create "Fix something" --priority 0 --type bug
    {{ bl_bin }} create "Write docs" --priority 3 --type task
    echo "Scratch board at: $dir"
    echo "Run 'just clean-scratch $dir' when done."
    BEADS_LITE_DB="$dir/.beads-lite/beads.db" {{ justfile_directory() }}/{{ bin }}

# Remove a scratch directory
clean-scratch dir:
    rm -rf {{ dir }}

# Run go vet and check formatting
lint:
    go vet ./...
    @gofmt -l . | grep . && echo "gofmt: files need formatting" && exit 1 || true
