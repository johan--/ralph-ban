package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// runClaude starts a Claude Code session with the ralph-ban plugin loaded
// and the operator role as the system prompt.
func runClaude(args []string) {
	fs := flag.NewFlagSet("claude", flag.ExitOnError)
	name := fs.String("name", "claude", "agent name (flows to hooks via CLAUDE_AGENT_NAME)")
	model := fs.String("model", "opus", "claude model (opus, sonnet, haiku)")
	autonomous := fs.Bool("autonomous", false, "skip permission prompts (dangerously-skip-permissions)")
	prompt := fs.String("prompt", "", "override the initial prompt sent to claude")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ralph-ban claude [flags]\n\nStart a Claude Code session with board operator role.\n\nFlags:\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	pluginDir, err := findPluginDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot locate plugin directory: %v\n", err)
		os.Exit(1)
	}

	operatorPrompt, err := readOperatorPrompt(pluginDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read operator prompt: %v\n", err)
		os.Exit(1)
	}

	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		fmt.Fprintf(os.Stderr, "claude not found in PATH. Install Claude Code first.\n")
		os.Exit(1)
	}

	claudeArgs := buildClaudeArgs(pluginDir, operatorPrompt, *model, *autonomous, *prompt)

	// Set agent name so hooks can identify this session.
	os.Setenv("CLAUDE_AGENT_NAME", *name)

	// Replace this process with claude for clean signal handling.
	if err := syscall.Exec(claudeBin, append([]string{"claude"}, claudeArgs...), os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to exec claude: %v\n", err)
		os.Exit(1)
	}
}

// buildClaudeArgs constructs the argument list for the claude binary.
func buildClaudeArgs(pluginDir, operatorPrompt, model string, autonomous bool, prompt string) []string {
	args := []string{
		"--plugin-dir", pluginDir,
		"--model", model,
		"--append-system-prompt", operatorPrompt,
	}

	if autonomous {
		args = append(args, "--dangerously-skip-permissions")
	}

	// Initial prompt: user override or default.
	if prompt == "" {
		prompt = "Check the board and start working on the highest-priority ready item."
	}
	args = append(args, "--prompt", prompt)

	return args
}

// findPluginDir locates the ralph-ban plugin directory by looking for
// .claude-plugin/plugin.json relative to the binary, then relative to cwd.
func findPluginDir() (string, error) {
	// Try relative to the binary location (works when built from this repo).
	binPath, err := os.Executable()
	if err == nil {
		binDir := filepath.Dir(binPath)
		candidate := filepath.Join(binDir, ".claude-plugin", "plugin.json")
		if _, err := os.Stat(candidate); err == nil {
			return binDir, nil
		}
	}

	// Try current working directory.
	cwd, err := os.Getwd()
	if err == nil {
		candidate := filepath.Join(cwd, ".claude-plugin", "plugin.json")
		if _, err := os.Stat(candidate); err == nil {
			return cwd, nil
		}
	}

	return "", fmt.Errorf("no .claude-plugin/plugin.json found near binary or in cwd")
}

// readOperatorPrompt reads agents/operator.md, strips YAML frontmatter,
// and returns the markdown body as the system prompt.
func readOperatorPrompt(pluginDir string) (string, error) {
	path := filepath.Join(pluginDir, "agents", "operator.md")
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var body strings.Builder
	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	frontmatterDone := false

	for scanner.Scan() {
		line := scanner.Text()

		if !frontmatterDone {
			if !inFrontmatter && line == "---" {
				inFrontmatter = true
				continue
			}
			if inFrontmatter && line == "---" {
				frontmatterDone = true
				continue
			}
			if inFrontmatter {
				continue
			}
		}

		body.WriteString(line)
		body.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	result := strings.TrimSpace(body.String())
	if result == "" {
		return "", fmt.Errorf("operator.md has no content after frontmatter")
	}
	return result, nil
}
