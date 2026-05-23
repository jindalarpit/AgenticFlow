package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agenticflow/agenticflow/internal/cli"
	"github.com/agenticflow/agenticflow/internal/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Control the local agent runtime daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the local agent runtime daemon",
	Long: `Start the daemon process that polls for tasks and executes them
using local agent CLIs (Claude, Codex, Gemini, etc.).

Use --foreground to run in the current terminal with signal handling.
Without --foreground, prints instructions for running in the background.`,
	RunE: runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	Long:  `Gracefully stop the running daemon by sending SIGTERM. If the daemon does not exit within 30 seconds, SIGKILL is sent.`,
	RunE:  runDaemonStop,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Long:  `Display the daemon's running state, PID, uptime, detected agents, and heartbeat status.`,
	RunE:  runDaemonStatus,
}

func init() {
	daemonStartCmd.Flags().Bool("foreground", false, "Run in the foreground instead of background")

	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
}

func runDaemonStart(cmd *cobra.Command, _ []string) error {
	foreground, _ := cmd.Flags().GetBool("foreground")

	// Load daemon configuration with overrides from flags.
	overrides := daemon.Overrides{}
	if serverURL, _ := cmd.Root().Flags().GetString("server-url"); serverURL != "" {
		overrides.ServerURL = &serverURL
	}

	cfg, err := daemon.LoadConfig(overrides)
	if err != nil {
		return fmt.Errorf("load daemon config: %w", err)
	}

	// Check if daemon is already running.
	pid, err := daemon.ReadPIDFile()
	if err != nil {
		return fmt.Errorf("check PID file: %w", err)
	}
	if pid != 0 && isProcessAlive(pid) {
		return fmt.Errorf("daemon is already running (PID %d)", pid)
	}

	if !foreground {
		// Go doesn't easily fork. Print instructions for background execution.
		fmt.Fprintln(os.Stderr, "Background daemonization is not directly supported in Go.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "To run the daemon in the background, use one of:")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  af daemon start --foreground &")
		fmt.Fprintln(os.Stderr, "  nohup af daemon start --foreground > ~/.agenticflow/daemon.log 2>&1 &")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Or use --foreground to run in the current terminal:")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  af daemon start --foreground")
		return nil
	}

	// Run in foreground with signal handling.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	d := daemon.New(cfg, logger)

	// Wire up the real HTTP client for server communication.
	cliCfg := cli.LoadConfig()
	if cliCfg.Token != "" && cfg.ServerURL != "" {
		client := daemon.NewRealHTTPClient(cfg.ServerURL, cliCfg.Token, cfg.DaemonID)
		d.SetClient(client)
	} else {
		logger.Warn("no token or server URL configured, daemon will run without server communication")
	}

	// Set up context with signal handling for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig.String())
		cancel()
	}()

	logger.Info("starting daemon in foreground mode",
		"daemon_id", cfg.DaemonID,
		"device_name", cfg.DeviceName,
	)

	if err := d.Run(ctx); err != nil {
		return fmt.Errorf("daemon run: %w", err)
	}

	return nil
}

func runDaemonStop(_ *cobra.Command, _ []string) error {
	pid, err := daemon.ReadPIDFile()
	if err != nil {
		return fmt.Errorf("read PID file: %w", err)
	}
	if pid == 0 {
		fmt.Fprintln(os.Stderr, "No running daemon found (no PID file).")
		return nil
	}

	// Check if the process is actually running.
	if !isProcessAlive(pid) {
		fmt.Fprintf(os.Stderr, "Daemon process (PID %d) is not running. Cleaning up stale PID file.\n", pid)
		// Clean up stale PID file.
		if rmErr := removePIDFile(); rmErr != nil {
			return fmt.Errorf("remove stale PID file: %w", rmErr)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "Stopping daemon (PID %d)...\n", pid)

	// Send SIGTERM for graceful shutdown.
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to PID %d: %w", pid, err)
	}

	// Wait up to 30 seconds for the process to exit.
	const stopTimeout = 30 * time.Second
	const pollInterval = 500 * time.Millisecond

	deadline := time.Now().Add(stopTimeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			fmt.Fprintln(os.Stderr, "Daemon stopped gracefully.")
			// Clean up PID file if still present.
			_ = removePIDFile()
			return nil
		}
		time.Sleep(pollInterval)
	}

	// Process didn't exit in time — send SIGKILL.
	fmt.Fprintf(os.Stderr, "Daemon did not stop within %s, sending SIGKILL...\n", stopTimeout)
	if err := process.Signal(syscall.SIGKILL); err != nil {
		// Process may have exited between our check and the kill.
		if !isProcessAlive(pid) {
			fmt.Fprintln(os.Stderr, "Daemon stopped.")
			_ = removePIDFile()
			return nil
		}
		return fmt.Errorf("send SIGKILL to PID %d: %w", pid, err)
	}

	// Wait briefly for SIGKILL to take effect.
	time.Sleep(1 * time.Second)
	if !isProcessAlive(pid) {
		fmt.Fprintln(os.Stderr, "Daemon killed.")
	} else {
		fmt.Fprintf(os.Stderr, "Warning: process %d may still be running.\n", pid)
	}

	// Remove PID file.
	_ = removePIDFile()
	return nil
}

func runDaemonStatus(_ *cobra.Command, _ []string) error {
	pid, err := daemon.ReadPIDFile()
	if err != nil {
		return fmt.Errorf("read PID file: %w", err)
	}

	if pid == 0 || !isProcessAlive(pid) {
		fmt.Println("Status:  stopped")
		fmt.Println("PID:     -")
		fmt.Println("Uptime:  -")

		// Still show detected agents even when stopped.
		printDetectedAgents()
		fmt.Println("Heartbeat: unknown (daemon not running)")
		return nil
	}

	// Daemon is running.
	fmt.Println("Status:  running")
	fmt.Printf("PID:     %d\n", pid)

	// Attempt to get uptime from process start time.
	uptime := getProcessUptime(pid)
	if uptime > 0 {
		fmt.Printf("Uptime:  %s\n", formatDuration(uptime))
	} else {
		fmt.Println("Uptime:  unknown")
	}

	// Detect agents.
	printDetectedAgents()

	// Heartbeat status — since we can't query the running daemon's internal state
	// directly, we report based on whether the daemon is running.
	fmt.Println("Heartbeat: active (daemon running)")

	return nil
}

// printDetectedAgents runs agent detection and prints the results.
func printDetectedAgents() {
	agents := daemon.DetectAgents(nil)
	if len(agents) == 0 {
		fmt.Println("Agents:  none detected")
		return
	}

	fmt.Printf("Agents:  %d detected\n", len(agents))
	for name, entry := range agents {
		fmt.Printf("  - %s (version: %s, path: %s)\n", name, entry.Version, entry.Path)
	}
}

// isProcessAlive checks if a process with the given PID is currently running.
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check if alive.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// removePIDFile removes the daemon PID file.
func removePIDFile() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	pidPath := home + "/.agenticflow/daemon.pid"
	if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// getProcessUptime attempts to determine how long a process has been running.
// On macOS/Linux, this uses /proc or sysctl. Returns 0 if unable to determine.
func getProcessUptime(pid int) time.Duration {
	// Try reading /proc/<pid>/stat for Linux.
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	if _, err := os.Stat(statPath); err == nil {
		// On Linux, we could parse the start time from /proc/pid/stat.
		// For simplicity and portability, we use the PID file modification time
		// as a proxy for daemon start time.
		return getUptimeFromPIDFile()
	}

	// Fallback: use PID file modification time as proxy.
	return getUptimeFromPIDFile()
}

// getUptimeFromPIDFile uses the PID file's modification time as a proxy for
// when the daemon started.
func getUptimeFromPIDFile() time.Duration {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0
	}
	pidPath := home + "/.agenticflow/daemon.pid"
	info, err := os.Stat(pidPath)
	if err != nil {
		return 0
	}
	return time.Since(info.ModTime())
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", h, m)
}
