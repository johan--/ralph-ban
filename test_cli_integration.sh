#!/usr/bin/env bash
# CLI integration tests for ralph-ban + beads-lite.
# Exercises the full pipeline: bl CLI -> SQLite -> JSONL output.
# Run from ralph-ban root: bash test_cli_integration.sh
set -euo pipefail

# Use the system bl binary (built by `just build-bl`).
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BL="${BL:-/usr/local/bin/bl}"
if [ ! -x "$BL" ]; then
  echo "Building beads-lite..."
  (cd "$SCRIPT_DIR/../beads-lite" && go build -o "$BL" ./cmd/bl)
fi
alias bl="$BL"
# Also export as function so subshells use it
bl() { "$BL" "$@"; }
export -f bl 2>/dev/null || true

PASS=0
FAIL=0
TEST_DIR=""

# --- Helpers ---

setup() {
  TEST_DIR=$(mktemp -d)
  cd "$TEST_DIR"
  # Isolate from parent environment — BL_ROOT leaks from ./ralph-ban claude
  unset BL_ROOT
  bl init >/dev/null 2>&1
}

teardown() {
  cd /
  rm -rf "$TEST_DIR"
}

assert_contains() {
  local haystack="$1" needle="$2" msg="$3"
  if echo "$haystack" | grep -q "$needle"; then
    PASS=$((PASS + 1))
  else
    FAIL=$((FAIL + 1))
    echo "FAIL: $msg"
    echo "  expected to contain: $needle"
    echo "  got: $haystack"
  fi
}

assert_not_contains() {
  local haystack="$1" needle="$2" msg="$3"
  if echo "$haystack" | grep -q "$needle"; then
    FAIL=$((FAIL + 1))
    echo "FAIL: $msg"
    echo "  expected NOT to contain: $needle"
    echo "  got: $haystack"
  else
    PASS=$((PASS + 1))
  fi
}

assert_eq() {
  local got="$1" want="$2" msg="$3"
  if [ "$got" = "$want" ]; then
    PASS=$((PASS + 1))
  else
    FAIL=$((FAIL + 1))
    echo "FAIL: $msg"
    echo "  got:  $got"
    echo "  want: $want"
  fi
}

extract_id() {
  echo "$1" | grep -o 'bl-[a-z0-9]*' | head -1
}

# --- Tests ---

test_create_defaults_to_todo() {
  setup
  local out id json_out
  out=$(bl create "Test Task")
  id=$(extract_id "$out")
  json_out=$(bl show "$id" --json)
  assert_contains "$json_out" '"status":"todo"' "create defaults to todo status"
  teardown
}

test_create_with_priority_and_type() {
  setup
  local out id json_out
  out=$(bl create "Bug Fix" --priority 0 --type bug)
  id=$(extract_id "$out")
  json_out=$(bl show "$id" --json)
  assert_contains "$json_out" '"priority":0' "priority P0 preserved"
  assert_contains "$json_out" '"issue_type":"bug"' "type bug preserved"
  teardown
}

test_all_five_statuses_valid() {
  setup
  local out id
  out=$(bl create "Status Test")
  id=$(extract_id "$out")

  for status in backlog todo doing review done; do
    bl update "$id" --status "$status" >/dev/null 2>&1
    local json_out
    json_out=$(bl show "$id" --json)
    assert_contains "$json_out" "\"status\":\"$status\"" "update to $status works"
  done
  teardown
}

test_old_statuses_rejected() {
  setup
  local out id
  out=$(bl create "Reject Old")
  id=$(extract_id "$out")

  for status in open in_progress closed; do
    if bl update "$id" --status "$status" >/dev/null 2>&1; then
      FAIL=$((FAIL + 1))
      echo "FAIL: old status '$status' should be rejected"
    else
      PASS=$((PASS + 1))
    fi
  done
  teardown
}

test_claim_sets_doing() {
  setup
  local out id json_out
  out=$(bl create "Claimable")
  id=$(extract_id "$out")
  bl claim "$id" --agent test-agent >/dev/null
  json_out=$(bl show "$id" --json)
  assert_contains "$json_out" '"status":"doing"' "claim sets status to doing"
  assert_contains "$json_out" '"assigned_to":"test-agent"' "claim sets assigned_to"
  teardown
}

test_unclaim_resets_to_todo() {
  setup
  local out id json_out
  out=$(bl create "Unclaim Me")
  id=$(extract_id "$out")
  bl claim "$id" --agent test-agent >/dev/null
  bl unclaim "$id" >/dev/null
  json_out=$(bl show "$id" --json)
  assert_contains "$json_out" '"status":"todo"' "unclaim resets to todo"
  teardown
}

test_close_sets_done() {
  setup
  local out id json_out
  out=$(bl create "Close Me")
  id=$(extract_id "$out")
  bl close "$id" >/dev/null
  json_out=$(bl show "$id" --json)
  assert_contains "$json_out" '"status":"done"' "close sets status to done"
  assert_contains "$json_out" '"resolution":"done"' "close default resolution is done"
  teardown
}

test_close_with_wontfix() {
  setup
  local out id json_out
  out=$(bl create "Wontfix")
  id=$(extract_id "$out")
  bl close "$id" --resolution wontfix >/dev/null
  json_out=$(bl show "$id" --json)
  assert_contains "$json_out" '"resolution":"wontfix"' "close with wontfix resolution"
  teardown
}

test_ready_excludes_backlog() {
  setup
  local out id
  out=$(bl create "Todo Task")
  id=$(extract_id "$out")
  # Create a backlog item
  bl update "$id" --status backlog >/dev/null

  local ready_out
  ready_out=$(bl ready --json 2>/dev/null || true)
  assert_not_contains "$ready_out" "$id" "backlog items not in ready"
  teardown
}

test_ready_includes_todo_doing_review() {
  setup
  local out1 out2 out3 id1 id2 id3
  out1=$(bl create "Todo Item")
  id1=$(extract_id "$out1")

  out2=$(bl create "Doing Item")
  id2=$(extract_id "$out2")
  bl update "$id2" --status doing >/dev/null

  out3=$(bl create "Review Item")
  id3=$(extract_id "$out3")
  bl update "$id3" --status review >/dev/null

  local ready_out
  ready_out=$(bl ready --json)
  assert_contains "$ready_out" "$id1" "todo item in ready"
  assert_contains "$ready_out" "$id2" "doing item in ready"
  assert_contains "$ready_out" "$id3" "review item in ready"
  teardown
}

test_ready_excludes_done() {
  setup
  local out id
  out=$(bl create "Done Task")
  id=$(extract_id "$out")
  bl close "$id" >/dev/null

  local ready_out
  ready_out=$(bl ready --json 2>/dev/null || true)
  assert_not_contains "$ready_out" "$id" "done items not in ready"
  teardown
}

test_list_json_all_statuses() {
  setup
  local ids=()
  for status in backlog todo doing review done; do
    local out id
    out=$(bl create "${status} card")
    id=$(extract_id "$out")
    bl update "$id" --status "$status" >/dev/null
    ids+=("$id")
  done

  local json_out
  json_out=$(bl list --json)
  for id in "${ids[@]}"; do
    assert_contains "$json_out" "$id" "list --json contains $id"
  done

  # Verify correct status strings in output
  assert_contains "$json_out" '"status":"backlog"' "list --json has backlog status"
  assert_contains "$json_out" '"status":"todo"' "list --json has todo status"
  assert_contains "$json_out" '"status":"doing"' "list --json has doing status"
  assert_contains "$json_out" '"status":"review"' "list --json has review status"
  assert_contains "$json_out" '"status":"done"' "list --json has done status"
  teardown
}

test_list_filter_by_new_statuses() {
  setup
  bl create "Backlog Card" >/dev/null
  local doing_out
  doing_out=$(bl create "Doing Card")
  local doing_id
  doing_id=$(extract_id "$doing_out")
  bl update "$doing_id" --status doing >/dev/null

  local filtered
  filtered=$(bl list --status doing --json)
  assert_contains "$filtered" "$doing_id" "filter by doing shows doing card"

  local count
  count=$(echo "$filtered" | wc -l | tr -d ' ')
  assert_eq "$count" "1" "filter by doing returns exactly 1 result"
  teardown
}

test_export_import_roundtrip() {
  setup
  # Create diverse dataset
  local out1 out2 out3 id1 id2 id3
  out1=$(bl create "Export A" --priority 0 --type bug)
  id1=$(extract_id "$out1")
  out2=$(bl create "Export B" --priority 1 --type feature)
  id2=$(extract_id "$out2")
  bl update "$id2" --status doing >/dev/null
  out3=$(bl create "Export C")
  id3=$(extract_id "$out3")
  bl close "$id3" >/dev/null

  # Export
  bl export backup.jsonl >/dev/null

  # Verify export file has new statuses
  local backup
  backup=$(cat backup.jsonl)
  assert_contains "$backup" '"status":"todo"' "export contains todo"
  assert_contains "$backup" '"status":"doing"' "export contains doing"
  assert_contains "$backup" '"status":"done"' "export contains done"

  # Reimport into fresh DB
  rm -rf .beads-lite
  bl init >/dev/null 2>&1
  local import_out
  import_out=$(bl import backup.jsonl)
  assert_contains "$import_out" "3 created" "import creates 3 issues"

  # Verify round-trip integrity
  local list_out
  list_out=$(bl list --json)
  assert_contains "$list_out" "$id1" "round-trip preserves id1"
  assert_contains "$list_out" "$id2" "round-trip preserves id2"
  assert_contains "$list_out" "$id3" "round-trip preserves id3"
  teardown
}

test_delete_removes_from_json() {
  setup
  local out id
  out=$(bl create "Delete Me")
  id=$(extract_id "$out")
  bl delete "$id" --confirm >/dev/null

  local list_out
  list_out=$(bl list --json 2>/dev/null || true)
  assert_not_contains "$list_out" "$id" "deleted issue not in list"
  teardown
}

test_status_transition_full_journey() {
  setup
  local out id
  out=$(bl create "Full Journey")
  id=$(extract_id "$out")

  # backlog -> todo -> doing -> review -> done
  bl update "$id" --status backlog >/dev/null
  local json
  json=$(bl show "$id" --json)
  assert_contains "$json" '"status":"backlog"' "journey: backlog"

  bl update "$id" --status todo >/dev/null
  json=$(bl show "$id" --json)
  assert_contains "$json" '"status":"todo"' "journey: todo"

  bl update "$id" --status doing >/dev/null
  json=$(bl show "$id" --json)
  assert_contains "$json" '"status":"doing"' "journey: doing"

  bl update "$id" --status review >/dev/null
  json=$(bl show "$id" --json)
  assert_contains "$json" '"status":"review"' "journey: review"

  bl update "$id" --status done >/dev/null
  json=$(bl show "$id" --json)
  assert_contains "$json" '"status":"done"' "journey: done"
  teardown
}

test_format_string_widths() {
  setup
  # Verify list output uses the new shorter status format
  local out
  bl create "Format Test" >/dev/null
  out=$(bl list)
  # Status should be left-aligned in 7 chars (longest: "backlog")
  # The format is: id  status  P#  type  title
  assert_contains "$out" "todo" "list shows todo status"
  assert_not_contains "$out" "open" "list does not show old 'open' status"
  teardown
}

# --- ralph-ban --dump tests ---

RALPH_BAN="${RALPH_BAN:-/tmp/ralph-ban-test}"
if [ ! -x "$RALPH_BAN" ]; then
  echo "Building ralph-ban..."
  (cd "$SCRIPT_DIR" && go build -o "$RALPH_BAN" .)
fi

test_dump_empty_board() {
  setup
  local out
  out=$(BEADS_LITE_DB="$TEST_DIR/.beads-lite/beads.db" "$RALPH_BAN" --dump 2>/dev/null)
  assert_contains "$out" '"columns"' "dump outputs columns field"
  assert_contains "$out" '"view"' "dump outputs view field"
  assert_contains "$out" '"Backlog"' "dump contains Backlog column title"
  assert_contains "$out" '"To Do"' "dump contains To Do column title"
  assert_contains "$out" '"Done"' "dump contains Done column title"
  # All columns should have empty cards arrays
  local card_count
  card_count=$(echo "$out" | jq '[.columns[].cards | length] | add')
  assert_eq "$card_count" "0" "dump empty board has 0 total cards"
  teardown
}

test_dump_cards_in_correct_columns() {
  setup
  local out1 out2 out3 id1 id2 id3
  out1=$(bl create "Dump Todo")
  id1=$(extract_id "$out1")

  out2=$(bl create "Dump Doing")
  id2=$(extract_id "$out2")
  bl update "$id2" --status doing >/dev/null

  out3=$(bl create "Dump Done")
  id3=$(extract_id "$out3")
  bl close "$id3" >/dev/null

  local dump
  dump=$(BEADS_LITE_DB="$TEST_DIR/.beads-lite/beads.db" "$RALPH_BAN" --dump 2>/dev/null)

  # Verify cards land in correct columns by index
  local todo_ids doing_ids done_ids
  todo_ids=$(echo "$dump" | jq -r '.columns[1].cards[].id')
  doing_ids=$(echo "$dump" | jq -r '.columns[2].cards[].id')
  done_ids=$(echo "$dump" | jq -r '.columns[4].cards[].id')

  assert_contains "$todo_ids" "$id1" "dump: todo card in To Do column"
  assert_contains "$doing_ids" "$id2" "dump: doing card in Doing column"
  assert_contains "$done_ids" "$id3" "dump: done card in Done column"
  teardown
}

test_dump_view_contains_card_titles() {
  setup
  bl create "Visible In View" >/dev/null

  local dump view
  dump=$(BEADS_LITE_DB="$TEST_DIR/.beads-lite/beads.db" "$RALPH_BAN" --dump 2>/dev/null)
  view=$(echo "$dump" | jq -r '.view')

  assert_contains "$view" "Visible In View" "dump view renders card title"
  assert_contains "$view" "Backlog" "dump view renders Backlog header"
  assert_contains "$view" "To Do" "dump view renders To Do header"
  teardown
}

test_dump_width_height_respected() {
  setup
  local dump
  dump=$(BEADS_LITE_DB="$TEST_DIR/.beads-lite/beads.db" "$RALPH_BAN" --dump --width 80 --height 30 2>/dev/null)

  local w h
  w=$(echo "$dump" | jq '.width')
  h=$(echo "$dump" | jq '.height')
  assert_eq "$w" "80" "dump width is 80"
  assert_eq "$h" "30" "dump height is 30"
  teardown
}

test_dump_narrow_width_pans() {
  setup
  # At 48 chars, minColumnWidth=24 means 2 visible columns
  local dump pan_offset
  dump=$(BEADS_LITE_DB="$TEST_DIR/.beads-lite/beads.db" "$RALPH_BAN" --dump --width 48 2>/dev/null)
  pan_offset=$(echo "$dump" | jq '.pan_offset')
  assert_eq "$pan_offset" "0" "dump narrow width starts at pan_offset 0"

  # View should show first columns but not all five
  local view
  view=$(echo "$dump" | jq -r '.view')
  assert_contains "$view" "Backlog" "narrow dump shows Backlog"
  teardown
}

test_dump_card_fields_preserved() {
  setup
  local out id
  out=$(bl create "Bug Card" --priority 0 --type bug)
  id=$(extract_id "$out")
  bl update "$id" --status review >/dev/null

  local dump card_json
  dump=$(BEADS_LITE_DB="$TEST_DIR/.beads-lite/beads.db" "$RALPH_BAN" --dump 2>/dev/null)
  # Review column is index 3
  card_json=$(echo "$dump" | jq ".columns[3].cards[] | select(.id == \"$id\")")

  local title priority type status
  title=$(echo "$card_json" | jq -r '.title')
  priority=$(echo "$card_json" | jq '.priority')
  type=$(echo "$card_json" | jq -r '.type')
  status=$(echo "$card_json" | jq -r '.status')

  assert_eq "$title" "Bug Card" "dump preserves card title"
  assert_eq "$priority" "0" "dump preserves priority"
  assert_eq "$type" "bug" "dump preserves type"
  assert_eq "$status" "review" "dump preserves status"
  teardown
}

test_dump_valid_single_line_json() {
  setup
  bl create "JSON Check" >/dev/null

  local dump line_count
  dump=$(BEADS_LITE_DB="$TEST_DIR/.beads-lite/beads.db" "$RALPH_BAN" --dump 2>/dev/null)
  line_count=$(echo "$dump" | wc -l | tr -d ' ')
  assert_eq "$line_count" "1" "dump is exactly one line"

  # Verify it's valid JSON
  if echo "$dump" | jq . >/dev/null 2>&1; then
    PASS=$((PASS + 1))
  else
    FAIL=$((FAIL + 1))
    echo "FAIL: dump output is not valid JSON"
  fi
  teardown
}

# --- Run all tests ---

echo "=== ralph-ban CLI Integration Tests ==="
echo ""

test_create_defaults_to_todo
test_create_with_priority_and_type
test_all_five_statuses_valid
test_old_statuses_rejected
test_claim_sets_doing
test_unclaim_resets_to_todo
test_close_sets_done
test_close_with_wontfix
test_ready_excludes_backlog
test_ready_includes_todo_doing_review
test_ready_excludes_done
test_list_json_all_statuses
test_list_filter_by_new_statuses
test_export_import_roundtrip
test_delete_removes_from_json
test_status_transition_full_journey
test_format_string_widths
test_dump_empty_board
test_dump_cards_in_correct_columns
test_dump_view_contains_card_titles
test_dump_width_height_respected
test_dump_narrow_width_pans
test_dump_card_fields_preserved
test_dump_valid_single_line_json

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
