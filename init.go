package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	beadslite "github.com/kylesnowschwartz/beads-lite"
)

const (
	ralphBanDir = ".ralph-ban"
	beadsDir    = ".beads-lite"
	dbName      = "beads.db"
)

// defaultConfig is written to .ralph-ban/config.json on init.
// WIP limits of 0 mean unlimited — these are sensible starting suggestions,
// not enforced policy. ProjectCommands is included with empty strings so users
// see the structure and can fill in their own build/test/lint commands without
// needing to know the JSON schema.
var defaultConfig = boardConfig{
	WIPLimits: map[string]int{
		"doing":  3,
		"review": 2,
	},
	ProjectCommands: ProjectCommands{},
}

// runInit bootstraps a new ralph-ban project in the current directory.
//
// It creates:
//   - .ralph-ban/           — TUI configuration directory
//   - .ralph-ban/config.json — WIP limits and other board configuration
//   - .beads-lite/           — beads-lite data directory
//   - .beads-lite/beads.db   — SQLite database (schema initialized)
//
// If .beads-lite/beads.db already exists, the existing database is adopted
// rather than replaced. This lets projects that already run `bl init` start
// using the TUI without disrupting their data.
//
// If --seed is passed, a small set of starter cards is created in Backlog so
// the board opens with something visible instead of empty columns.
func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	seedFlag := fs.Bool("seed", false, "create starter cards in Backlog")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ralph-ban init [flags]\n\nInitialize a new ralph-ban project in the current directory.\nCreates .ralph-ban/ (config) and .beads-lite/ (database).\n\nFlags:\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	seed := *seedFlag

	// --- Step 1: Create .ralph-ban/ config directory ---
	if err := os.MkdirAll(ralphBanDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create %s: %v\n", ralphBanDir, err)
		os.Exit(1)
	}

	configPath := filepath.Join(ralphBanDir, "config.json")
	configCreated := false
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := writeDefaultConfig(configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write config: %v\n", err)
			os.Exit(1)
		}
		configCreated = true
	}

	// --- Step 2: Create .beads-lite/ database ---
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create %s: %v\n", beadsDir, err)
		os.Exit(1)
	}

	dbPath := filepath.Join(beadsDir, dbName)
	dbExisted := fileExists(dbPath)

	store, err := beadslite.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// --- Step 3: Optionally seed starter cards ---
	seeded := 0
	if seed && !dbExisted {
		seeded, err = seedStarterCards(store)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to seed starter cards: %v\n", err)
			os.Exit(1)
		}
	}

	// --- Step 4: Install Claude Code plugin ---
	pluginInstalled := installPlugin()

	// --- Step 5: Report results ---
	fmt.Println("Initialized ralph-ban:")
	fmt.Printf("  %s/          board configuration\n", ralphBanDir)
	if configCreated {
		fmt.Printf("  %s  WIP limits: doing=3, review=2\n", configPath)
	} else {
		fmt.Printf("  %s  (already exists, kept as-is)\n", configPath)
	}
	fmt.Printf("  %s/       task database\n", beadsDir)
	if dbExisted {
		fmt.Printf("  %s     (existing database adopted)\n", dbPath)
	} else {
		fmt.Printf("  %s     (new database created)\n", dbPath)
		if seeded > 0 {
			fmt.Printf("  seeded %d starter cards in Backlog\n", seeded)
		}
	}
	if pluginInstalled {
		fmt.Println("  claude plugin  ralph-ban hooks + agents installed")
	} else {
		fmt.Println()
		fmt.Println("Claude Code plugin (optional):")
		fmt.Println("  claude plugin marketplace add kylesnowschwartz/ralph-ban")
		fmt.Println("  claude plugin install ralph-ban@ralph-ban")
	}

	fmt.Println()
	fmt.Println("Run 'ralph-ban' to open the board.")
	if !dbExisted {
		fmt.Println("Run 'bl create \"task title\"' to add cards from the CLI.")
	}
}

// installPlugin attempts to install the ralph-ban Claude Code plugin via the
// claude CLI. Both commands are idempotent — they succeed silently if the
// marketplace or plugin is already registered.
//
// Returns true if the claude CLI was found and both commands were attempted,
// false if claude is not on PATH (caller prints manual instructions instead).
func installPlugin() bool {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return false
	}

	// Add the marketplace (idempotent — silently succeeds if already added).
	addCmd := exec.Command(claudePath, "plugin", "marketplace", "add", "kylesnowschwartz/ralph-ban")
	addCmd.CombinedOutput() //nolint:errcheck // ignore error — may already exist

	// Install the plugin (idempotent — silently succeeds if already installed).
	installCmd := exec.Command(claudePath, "plugin", "install", "ralph-ban@ralph-ban", "--scope", "user")
	installCmd.CombinedOutput() //nolint:errcheck // ignore error — may already exist

	return true
}

// writeDefaultConfig serializes defaultConfig to the given path.
func writeDefaultConfig(path string) error {
	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	// Append newline for clean file ending.
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// fileExists returns true if path exists on disk (file or directory).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// starterCards defines the seed data created when running `ralph-ban init --seed`.
// These are placed in Backlog so the user triages them deliberately.
var starterCards = []struct {
	title       string
	description string
}{
	{
		title:       "Add your first task",
		description: "Press 'n' to create a new card, or run 'bl create \"task title\"' from the CLI.",
	},
	{
		title:       "Move a card to Todo",
		description: "Select a card and press 'l' (or right arrow) to move it right across columns.",
	},
	{
		title:       "Edit a card",
		description: "Select a card and press 'e' to open the edit form. Press Enter to save, Esc to cancel.",
	},
}

// seedStarterCards creates the starter cards in the store and returns how many were created.
func seedStarterCards(store *beadslite.Store) (int, error) {
	for i, sc := range starterCards {
		issue := beadslite.NewIssue(sc.title)
		issue.Status = beadslite.StatusBacklog
		issue.Priority = i // P0, P1, P2 so they sort top-to-bottom
		issue.Description = sc.description
		if err := store.CreateIssue(issue); err != nil {
			return i, fmt.Errorf("create starter card %q: %w", sc.title, err)
		}
	}
	return len(starterCards), nil
}
