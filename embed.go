package main

import "embed"

// pluginFS bundles the complete Claude Code plugin for extraction by `ralph-ban init`.
// The three embedded trees — plugin manifest, hook scripts, and agent definitions —
// form a self-contained plugin directory that `--plugin-dir` can load directly.
// The `all:` prefix is required for `.claude-plugin` because embed skips dot-prefixed
// directories by default.
//
// Agent source lives in `_agents/` (underscore prefix) to keep it out of Claude Code's
// agent discovery chain. extractPlugin remaps `_agents/` → `agents/` in the output
// so the plugin structure is correct.
//
//go:embed all:.claude-plugin _agents hooks
var pluginFS embed.FS
