package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agenticflow/agenticflow/daemon/internal/cli"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration as formatted JSON",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a CLI configuration value. Supported keys:
  server_url           — Server URL (http or https)
  poll_interval        — Task poll interval (e.g. 3s, 10s)
  heartbeat_interval   — Heartbeat interval (e.g. 15s, 30s)
  agent_timeout        — Agent execution timeout (e.g. 2h, 30m)
  max_concurrent_tasks — Max parallel tasks (1-100)`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
}

func runConfigShow(_ *cobra.Command, _ []string) error {
	cfg := cli.LoadConfig()

	path, err := cli.ConfigPath()
	if err != nil {
		path = "(unknown)"
	}

	fmt.Fprintf(os.Stderr, "Config file: %s\n\n", path)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	fmt.Fprintln(os.Stdout, string(data))
	return nil
}

func runConfigSet(_ *cobra.Command, args []string) error {
	key, value := args[0], args[1]

	// Validate the field before modifying config.
	if err := cli.ValidateField(key, value); err != nil {
		return fmt.Errorf("invalid value for %q: %w", key, err)
	}

	// Load, apply, and save.
	cfg := cli.LoadConfig()

	if err := cfg.ApplyField(key, value); err != nil {
		return fmt.Errorf("apply config: %w", err)
	}

	if err := cli.SaveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Set %s = %s\n", key, value)
	return nil
}
