#!/usr/bin/env bash
# Shared heartbeat functions for agent liveness tracking.
# Sourced by board-sync.sh to write and check agent heartbeats.
#
# Heartbeat files live at $BL_ROOT/.ralph-ban/heartbeats/<agent-name>
# and contain a unix timestamp written by each agent on every
# UserPromptSubmit event. The orchestrator checks for stale files (>5
# minutes) to detect hung workers.
#
# Workers run in isolated worktrees, so BL_ROOT (the main repo root) is
# used for the directory — not the worktree's local .ralph-ban/.

HEARTBEAT_DIR="${_GIT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}/.ralph-ban/heartbeats"
HEARTBEAT_STALE_SECONDS="${HEARTBEAT_STALE_SECONDS:-300}"  # 5 minutes

# write_heartbeat writes the current unix timestamp for this agent.
# Uses CLAUDE_AGENT_NAME when set; falls back to "claude".
write_heartbeat() {
  local name="${CLAUDE_AGENT_NAME:-claude}"
  mkdir -p "$HEARTBEAT_DIR"
  date +%s >"$HEARTBEAT_DIR/$name"
}

# cleanup_heartbeats removes heartbeat files for agents that no longer
# have doing cards. Prevents false stall alerts for completed workers.
# Requires board JSONL on stdin (output of `bl list --json`).
cleanup_heartbeats() {
  local board_state="$1"

  if [ ! -d "$HEARTBEAT_DIR" ]; then
    return
  fi

  # Build the set of agents that currently own doing cards.
  local active_agents
  active_agents=$(echo "$board_state" | jq -r '
    select(.status == "doing" and .assigned_to != null and .assigned_to != "")
    | .assigned_to
  ' 2>/dev/null || true)

  # Remove heartbeat files for agents not in the active set.
  for hb_file in "$HEARTBEAT_DIR"/*; do
    [ -f "$hb_file" ] || continue
    local agent_name
    agent_name=$(basename "$hb_file")
    if ! echo "$active_agents" | grep -qxF "$agent_name"; then
      rm -f "$hb_file"
    fi
  done
}

# detect_stalled_heartbeats outputs warning lines for agents whose
# heartbeat timestamp is older than HEARTBEAT_STALE_SECONDS.
# Only fires if the agent also has a doing card (via board state).
detect_stalled_heartbeats() {
  local board_state="$1"

  if [ ! -d "$HEARTBEAT_DIR" ]; then
    return
  fi

  local now
  now=$(date +%s)

  for hb_file in "$HEARTBEAT_DIR"/*; do
    [ -f "$hb_file" ] || continue
    local agent_name last_seen elapsed
    agent_name=$(basename "$hb_file")
    last_seen=$(cat "$hb_file" 2>/dev/null || echo "0")
    # Skip files with non-numeric or missing timestamps.
    if ! [[ "$last_seen" =~ ^[0-9]+$ ]]; then
      continue
    fi
    elapsed=$((now - last_seen))

    if [ "$elapsed" -ge "$HEARTBEAT_STALE_SECONDS" ]; then
      # Only warn if the agent owns a doing card — a stale file for a
      # completed agent would have been cleaned up; this is a safety check.
      local doing_cards
      doing_cards=$(echo "$board_state" | jq -r --arg agent "$agent_name" '
        select(.status == "doing" and .assigned_to == $agent)
        | "\(.id): \(.title)"
      ' 2>/dev/null || true)

      if [ -n "$doing_cards" ]; then
        local minutes=$(( elapsed / 60 ))
        echo "WORKER STALLED: Agent '${agent_name}' last seen ${minutes}m ago. Doing cards:"
        echo "$doing_cards" | while IFS= read -r card; do
          echo "  ${card}"
        done
      fi
    fi
  done
}
