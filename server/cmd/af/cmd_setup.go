package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agenticflow/agenticflow/internal/cli"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure the CLI, authenticate, and start the daemon",
	Long: `Configures the CLI to connect to an AgenticFlow server, then
authenticates via browser and starts the agent daemon.

Steps:
  1. Prompt for server URL (default: http://localhost:8080)
  2. Open browser for authentication
  3. Start the daemon`,
	RunE: runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	// Step 1: Prompt for server URL.
	cfg := cli.LoadConfig()

	defaultURL := "http://localhost:8080"
	if cfg.ServerURL != "" {
		defaultURL = cfg.ServerURL
	}

	fmt.Fprintf(os.Stderr, "Server URL [%s]: ", defaultURL)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	serverURL := defaultURL
	if line != "" {
		serverURL = line
	}

	// Validate the URL.
	if err := cli.ValidateField("server_url", serverURL); err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	cfg.ServerURL = serverURL
	if err := cli.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Server URL set to: %s\n", serverURL)

	// Step 2: Authenticate.
	fmt.Fprintln(os.Stderr, "\nStarting authentication...")
	if err := runLoginBrowser(cmd); err != nil {
		return err
	}

	// Step 3: Start daemon.
	fmt.Fprintln(os.Stderr, "\nStarting daemon...")
	fmt.Fprintln(os.Stderr, "→ Run 'af daemon start' to start the background daemon.")

	fmt.Fprintln(os.Stderr, "\n✓ Setup complete!")
	return nil
}
