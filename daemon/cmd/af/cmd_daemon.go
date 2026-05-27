package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/agenticflow/agenticflow/daemon/internal/cli"
	"github.com/agenticflow/agenticflow/daemon/internal/daemon"
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
Without --foreground, spawns a background process and exits.`,
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
	daemonStartCmd.Flags().Int("health-port", 0, "Health endpoint port (default: 19514, env: AF_DAEMON_HEALTH_PORT)")
	daemonStatusCmd.Flags().String("output", "text", "Output format: text or json")

	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
}

func runDaemonStart(cmd *cobra.Command, _ []string) error {
	foreground, _ := cmd.Flags().GetBool("foreground")
	if foreground {
		return runDaemonForeground(cmd)
	}
	return runDaemonBackground(cmd)
}

// runDaemonBackground spawns the daemon as a detached child process.
// It checks the health endpoint first, resolves the current executable,
// opens the log file, applies platform-specific SysProcAttr, writes the
// PID file, releases the child handle, and polls until healthy or timeout.
func runDaemonBackground(cmd *cobra.Command) error {
	// Resolve health port from flag or default.
	healthPort := daemon.DefaultHealthPort
	if hp, _ := cmd.Flags().GetInt("health-port"); hp > 0 {
		healthPort = hp
	}

	// 1. Check health endpoint — if "running", error with existing PID.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	health := checkDaemonHealth(ctx, healthPort)
	if health["status"] == "running" {
		pid, _ := health["pid"].(float64)
		return fmt.Errorf("daemon is already running (pid %d). Use 'af daemon stop' first", int(pid))
	}

	// 2. Resolve current executable path.
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	// 3. Build child args: daemon start --foreground + forwarded flags.
	args := []string{"daemon", "start", "--foreground"}
	if serverURL, _ := cmd.Root().Flags().GetString("server-url"); serverURL != "" {
		args = append(args, "--server-url", serverURL)
	}
	if healthPort != daemon.DefaultHealthPort {
		args = append(args, "--health-port", strconv.Itoa(healthPort))
	}

	// 4. Ensure daemon directory exists and open log file.
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	daemonDir := filepath.Join(home, ".agenticflow")
	if err := os.MkdirAll(daemonDir, 0o755); err != nil {
		return fmt.Errorf("create daemon directory: %w", err)
	}

	logPath := filepath.Join(daemonDir, "daemon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", logPath, err)
	}

	// 5. Build the child command with platform-specific detachment.
	child := exec.Command(exePath, args...)
	child.Stdout = logFile
	child.Stderr = logFile
	child.SysProcAttr = daemonSysProcAttr(true)

	// 6. Start the child process. On Windows, retry without breakaway if
	// the parent's Job Object doesn't allow it.
	if err := child.Start(); err != nil {
		if isAccessDeniedSpawnErr(err) {
			// Retry without breakaway — exec.Cmd is not safe to Start()
			// twice, so build a fresh one.
			child = exec.Command(exePath, args...)
			child.Stdout = logFile
			child.Stderr = logFile
			child.SysProcAttr = daemonSysProcAttr(false)
			if err := child.Start(); err != nil {
				logFile.Close()
				return fmt.Errorf("start daemon (no breakaway): %w", err)
			}
		} else {
			logFile.Close()
			return fmt.Errorf("start daemon: %w", err)
		}
	}
	logFile.Close()

	pid := child.Process.Pid

	// 7. Release child process handle so the parent can exit independently.
	child.Process.Release()

	// 8. Write PID file.
	pidPath := filepath.Join(daemonDir, "daemon.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write PID file: %v\n", err)
	}

	// 9. Poll health endpoint until "running" or 15s timeout.
	deadline := time.Now().Add(15 * time.Second)
	started := false
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		hctx, hcancel := context.WithTimeout(context.Background(), 2*time.Second)
		health = checkDaemonHealth(hctx, healthPort)
		hcancel()
		if health["status"] == "running" {
			started = true
			break
		}
	}

	// 10. Print result.
	if !started {
		fmt.Fprintf(os.Stderr, "Warning: daemon may not have started successfully. Check logs:\n  %s\n", logPath)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Daemon started (pid %d, version %s)\n", pid, version)
	fmt.Fprintf(os.Stderr, "Logs: %s\n", logPath)
	return nil
}

// runDaemonForeground runs the daemon in the current process with signal handling.
func runDaemonForeground(cmd *cobra.Command) error {
	// Load daemon configuration with overrides from flags.
	cliVersion := version
	overrides := daemon.Overrides{
		CLIVersion: &cliVersion,
	}
	if serverURL, _ := cmd.Root().Flags().GetString("server-url"); serverURL != "" {
		overrides.ServerURL = &serverURL
	}
	if healthPort, _ := cmd.Flags().GetInt("health-port"); healthPort > 0 {
		overrides.HealthPort = &healthPort
	}

	cfg, err := daemon.LoadConfig(overrides)
	if err != nil {
		return fmt.Errorf("load daemon config: %w", err)
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
	healthPort := daemon.DefaultHealthPort

	// 1. Check health endpoint to determine if daemon is running.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	health := checkDaemonHealth(ctx, healthPort)
	if health["status"] != "running" {
		fmt.Fprintln(os.Stderr, "Daemon is not running.")
		return nil
	}

	pid, _ := health["pid"].(float64)
	if pid == 0 {
		return fmt.Errorf("could not determine daemon PID from health endpoint")
	}

	process, err := os.FindProcess(int(pid))
	if err != nil {
		return fmt.Errorf("find process %d: %w", int(pid), err)
	}

	// 2. Request graceful shutdown via POST /shutdown on the health endpoint.
	if err := requestDaemonShutdown(healthPort); err != nil {
		// Graceful shutdown request failed — fall back to SIGKILL.
		fmt.Fprintf(os.Stderr, "Graceful shutdown request failed: %v — falling back to forced kill.\n", err)
		if kerr := process.Kill(); kerr != nil {
			return fmt.Errorf("kill daemon (pid %d): %w", int(pid), kerr)
		}
		_ = removePIDFile()
		fmt.Fprintln(os.Stderr, "Daemon killed.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Stopping daemon (pid %d)...\n", int(pid))

	// 3. Poll health endpoint until daemon is gone — 10 cycles × 500ms = 5 seconds.
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		pctx, pcancel := context.WithTimeout(context.Background(), 1*time.Second)
		h := checkDaemonHealth(pctx, healthPort)
		pcancel()
		if h["status"] != "running" {
			_ = removePIDFile()
			fmt.Fprintln(os.Stderr, "Daemon stopped.")
			return nil
		}
	}

	// 4. Graceful shutdown didn't complete in time — fall back to SIGKILL.
	fmt.Fprintln(os.Stderr, "Daemon did not stop within 5s, sending SIGKILL...")
	if err := process.Signal(syscall.SIGKILL); err != nil {
		// Process may have exited between our check and the kill.
		if !isProcessAlive(int(pid)) {
			_ = removePIDFile()
			fmt.Fprintln(os.Stderr, "Daemon stopped.")
			return nil
		}
		return fmt.Errorf("send SIGKILL to PID %d: %w", int(pid), err)
	}

	// Wait briefly for SIGKILL to take effect.
	time.Sleep(1 * time.Second)
	if !isProcessAlive(int(pid)) {
		_ = removePIDFile()
		fmt.Fprintln(os.Stderr, "Daemon killed.")
	} else {
		fmt.Fprintf(os.Stderr, "Daemon is still stopping. It may be finishing a running task.\n")
	}

	return nil
}

// requestDaemonShutdown POSTs to the daemon's /shutdown endpoint to ask it
// to exit gracefully. Returns an error if the request could not be delivered
// (network error, non-2xx status, or the endpoint is unavailable).
func requestDaemonShutdown(healthPort int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/shutdown", healthPort)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

func runDaemonStatus(cmd *cobra.Command, _ []string) error {
	healthPort := daemon.DefaultHealthPort

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	health := checkDaemonHealth(ctx, healthPort)

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(health)
	}

	if health["status"] != "running" {
		fmt.Println("Status:  stopped")
		return nil
	}

	// Display running daemon info from health response.
	pid, _ := health["pid"].(float64)
	uptime, _ := health["uptime"].(string)
	activeTaskCount, _ := health["active_task_count"].(float64)

	fmt.Printf("Status:  running (pid %d, uptime %s)\n", int(pid), uptime)

	// Display detected runtimes (agents).
	if agents, ok := health["agents"].([]any); ok && len(agents) > 0 {
		fmt.Printf("Agents:  %d detected\n", len(agents))
		for _, a := range agents {
			if agent, ok := a.(map[string]any); ok {
				name, _ := agent["name"].(string)
				ver, _ := agent["version"].(string)
				path, _ := agent["path"].(string)
				fmt.Printf("  - %s (version: %s, path: %s)\n", name, ver, path)
			}
		}
	} else {
		fmt.Println("Agents:  none detected")
	}

	fmt.Printf("Tasks:   %d active\n", int(activeTaskCount))

	return nil
}

// checkDaemonHealth calls the daemon's local health endpoint on the given port.
func checkDaemonHealth(ctx context.Context, port int) map[string]any {
	addr := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return map[string]any{"status": "stopped"}
	}

	httpClient := &http.Client{Timeout: 2 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return map[string]any{"status": "stopped"}
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return map[string]any{"status": "stopped"}
	}
	return result
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
	pidPath := filepath.Join(home, ".agenticflow", "daemon.pid")
	if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
