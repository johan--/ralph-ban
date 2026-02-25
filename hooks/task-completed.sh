#!/usr/bin/env bash
# TaskCompleted: validate card state before allowing task completion.
# Exit 2 + stderr = block completion with feedback. Exit 0 = allow.
trap 'exit 0' ERR
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/board-state.sh"

if ! db_exists; then exit 0; fi

# Read task context from stdin JSON.
input=""
if [ ! -t 0 ]; then
  input=$(cat 2>/dev/null || true)
fi
teammate=$(echo "$input" | jq -r '.teammate_name // ""' 2>/dev/null || true)
[ -z "$teammate" ] && exit 0

# Workers should move cards to review before completing their task.
# Cards still in doing mean the worker skipped a step.
doing=$("$BL" list --assigned-to "$teammate" --json 2>/dev/null \
  | jq -r 'select(.status == "doing") | "\(.id): \(.title)"' 2>/dev/null || true)

if [ -n "$doing" ]; then
  echo "Cards still in doing — move them to review before completing:" >&2
  echo "$doing" >&2
  exit 2
fi
exit 0
