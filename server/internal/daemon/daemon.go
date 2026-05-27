// Package daemon provides the daemon runtime: detection, lifecycle, and execution.
package daemon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/agenticflow/agenticflow/internal/daemon/execenv"
)

// HTTPClient defines the interface for daemon-to-server HTTP communication.
// Using an interface allows tests to inject fakes without a real server.
type HTTPClient interface {
	// Register registers the daemon and its agent runtimes with the server.
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	// Deregister notifies the server that the daemon is going offline.
	Deregister(ctx context.Context, req DeregisterRequest) error
	// Heartbeat sends a heartbeat signal to the server.
	Heartbeat(ctx context.Context, req HeartbeatRequest) error
	// PollTasks polls the server for available tasks.
	PollTasks(ctx context.Context, req PollRequest) (*PollResponse, error)
	// StartTask notifies the server that a task has started execution.
	StartTask(ctx context.Context, taskID string) error
	// CompleteTask reports successful task completion with output and exit code.
	CompleteTask(ctx context.Context, taskID string, output string, exitCode int) error
	// FailTask reports task failure with error message and exit code.
	FailTask(ctx context.Context, taskID string, errMsg string, exitCode int) error
	// ReportMessages sends streaming output messages to the server.
	ReportMessages(ctx context.Context, taskID string, messages []TaskMessage) error
	// ReportInputState notifies the server of input detection state changes.
	// State is "waiting" when the CLI is waiting for input, "cleared" when output resumes.
	ReportInputState(ctx context.Context, taskID string, state string) error
	// ReportStageCompletion reports a workflow stage completion to the server.
	// POST /api/daemon/tasks/{taskId}/stages/{stageName}/complete
	ReportStageCompletion(ctx context.Context, taskID, stageName, outputContent string) error
	// CompleteTaskConversational reports conversational task completion with session tracking.
	// POST /api/daemon/tasks/{id}/complete with session_id and work_dir fields.
	CompleteTaskConversational(ctx context.Context, taskID, output, sessionID, workDir string) error
}

// RegisterRequest is the payload sent to POST /api/daemon/register.
type RegisterRequest struct {
	DaemonID   string            `json:"daemon_id"`
	DeviceName string            `json:"device_name"`
	Agents     map[string]string `json:"agents"` // provider -> version
}

// RegisterResponse is the response from POST /api/daemon/register.
type RegisterResponse struct {
	RuntimeIDs map[string]string `json:"runtime_ids"` // provider -> runtime_id
}

// DeregisterRequest is the payload sent to POST /api/daemon/deregister.
type DeregisterRequest struct {
	DaemonID string `json:"daemon_id"`
}

// HeartbeatRequest is the payload sent to POST /api/daemon/heartbeat.
type HeartbeatRequest struct {
	DaemonID    string   `json:"daemon_id"`
	Runtimes    []string `json:"runtimes"`
	ActiveTasks int64    `json:"active_tasks"`
}

// PollRequest is the payload sent to GET /api/daemon/tasks/poll.
type PollRequest struct {
	DaemonID   string   `json:"daemon_id"`
	RuntimeIDs []string `json:"runtime_ids"`
}

// PollResponse is the response from GET /api/daemon/tasks/poll.
type PollResponse struct {
	TaskID       string            `json:"task_id,omitempty"`
	AgentType    string            `json:"agent_type,omitempty"`
	Prompt       string            `json:"prompt,omitempty"`
	Model        string            `json:"model,omitempty"`
	ArgsTemplate string            `json:"args_template,omitempty"`
	EnvVars      map[string]string `json:"env_vars,omitempty"`
	Agent        *TaskAgentData    `json:"agent,omitempty"`

	// Stage-related fields (present only for staged tasks).
	CurrentStage  *StageInfo   `json:"current_stage,omitempty"`
	PriorStages   []PriorStage `json:"prior_stages,omitempty"`
	WorkspaceMode string       `json:"workspace_mode,omitempty"`
	WorkspacePath string       `json:"workspace_path,omitempty"`
	StageFeedback string       `json:"stage_feedback,omitempty"`

	// Conversational task fields (present only for conversational tasks).
	DeliverableType string           `json:"deliverable_type,omitempty"`
	PriorSessionID  string           `json:"prior_session_id,omitempty"`
	PriorContext    []string         `json:"prior_context,omitempty"`
	PriorWorkDir    string           `json:"prior_work_dir,omitempty"`
	WorkspaceConfig *WorkspaceConfig `json:"workspace_config,omitempty"`
}

// WorkspaceConfig holds workspace configuration for execution-type conversational tasks.
type WorkspaceConfig struct {
	GitRepoURL         string `json:"git_repo_url,omitempty"`
	LocalDirectoryPath string `json:"local_directory_path"`
}

// TaskMessage represents a single streaming output message from a task.
type TaskMessage struct {
	Sequence  int    `json:"sequence"`
	Stream    string `json:"stream"` // "stdout" or "stderr"
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// Daemon is the local agent runtime that manages lifecycle, heartbeats,
// task polling, and agent process execution.
type Daemon struct {
	cfg    Config
	logger *slog.Logger
	client HTTPClient

	// runtimes maps runtime_id -> provider name after registration.
	runtimes map[string]string

	// activeTasks tracks the number of currently executing tasks.
	activeTasks atomic.Int64

	// stdinManager manages stdin pipes for active tasks, enabling
	// bidirectional communication with CLI processes.
	stdinManager *StdinPipeManager

	// sequences tracks per-task sequence counters shared between the
	// streamingReporter (stdout/stderr) and handleTaskInput (stdin).
	sequences *sequenceTracker

	// healthServer is the local HTTP health endpoint server.
	healthServer *HealthServer

	// stopCh is closed when Stop() is called to signal shutdown.
	stopCh chan struct{}
	// done is closed when Run() exits.
	done chan struct{}

	// mu protects runtimes map.
	mu sync.RWMutex

	// startTime records when the daemon started for uptime calculation.
	startTime time.Time

	// heartbeatRetryDelay is the delay between heartbeat retries.
	// Defaults to 5s if zero. Overridable for testing.
	heartbeatRetryDelay time.Duration

	// consecutiveHeartbeatFailures tracks how many consecutive heartbeat
	// intervals have failed (all retries exhausted). This is NOT the retry
	// count within a single interval — it counts interval-level failures.
	// Reset to 0 on any successful heartbeat.
	consecutiveHeartbeatFailures int
}

// New creates a new Daemon instance with the given configuration and logger.
func New(cfg Config, logger *slog.Logger) *Daemon {
	return &Daemon{
		cfg:          cfg,
		logger:       logger,
		runtimes:     make(map[string]string),
		stdinManager: NewStdinPipeManager(),
		sequences:    newSequenceTracker(),
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
	}
}

// SetClient sets the HTTP client used for server communication.
// This must be called before Run(). It allows injection of test doubles.
func (d *Daemon) SetClient(client HTTPClient) {
	d.client = client
}

// Run starts the daemon lifecycle:
//  1. Clean stale PID file
//  2. Write PID file
//  3. Start health server
//  4. Register with server
//  5. Start heartbeat loop
//  6. Start poll loop
//  7. Start GC loop
//  8. Block until ctx is cancelled or Stop() is called
//  9. Stop health server
//  10. Deregister on shutdown
//  11. Remove PID file
func (d *Daemon) Run(ctx context.Context) error {
	defer close(d.done)

	d.startTime = time.Now()

	// Clean stale PID file from a previous crash.
	if err := d.CleanStalePIDFile(); err != nil {
		d.logger.Warn("failed to clean stale PID file", "error", err)
	}

	// Write our PID file.
	if err := d.WritePIDFile(); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}
	defer d.RemovePIDFile()

	// Start health server before entering main loop.
	// This binds the port early to detect another running daemon.
	healthCfg := HealthServerConfig{
		Port: d.cfg.HealthPort,
		ShutdownCallback: func() {
			d.Stop()
		},
	}
	d.healthServer = NewHealthServer(healthCfg, d)
	if err := d.healthServer.Start(); err != nil {
		return fmt.Errorf("start health server: %w", err)
	}
	d.logger.Info("health server started", "addr", d.healthServer.Addr())
	defer func() {
		if err := d.healthServer.Stop(); err != nil {
			d.logger.Warn("failed to stop health server", "error", err)
		}
		d.logger.Debug("health server stopped")
	}()

	agentNames := make([]string, 0, len(d.cfg.Agents))
	for name := range d.cfg.Agents {
		agentNames = append(agentNames, name)
	}
	d.logger.Info("starting daemon",
		"daemon_id", d.cfg.DaemonID,
		"device_name", d.cfg.DeviceName,
		"agents", agentNames,
		"server_url", d.cfg.ServerURL,
	)

	// Register with server.
	if err := d.register(ctx); err != nil {
		d.logger.Warn("initial registration failed, will retry on heartbeat", "error", err)
	}

	// Start background loops.
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		d.heartbeatLoop(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		d.pollLoop(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		d.gcLoop(ctx)
	}()

	d.logger.Info("daemon running",
		"poll_interval", d.cfg.PollInterval,
		"heartbeat_interval", d.cfg.HeartbeatInterval,
	)

	// Block until context is cancelled or Stop() is called.
	select {
	case <-ctx.Done():
		d.logger.Info("daemon shutting down (context cancelled)")
	case <-d.stopCh:
		d.logger.Info("daemon shutting down (stop requested)")
	}

	// Deregister from server.
	d.deregister()

	// Wait for background loops to finish (they check ctx/stopCh).
	wg.Wait()

	d.logger.Info("daemon stopped")
	return nil
}

// Stop initiates a graceful shutdown of the daemon with a 30-second timeout.
// It signals the run loop to exit and waits for completion.
func (d *Daemon) Stop() {
	d.logger.Info("stop requested, initiating graceful shutdown")

	// Signal the run loop to exit.
	select {
	case <-d.stopCh:
		// Already stopped.
		return
	default:
		close(d.stopCh)
	}

	// Wait for Run() to complete with a 30s timeout.
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	select {
	case <-d.done:
		// Clean shutdown completed.
	case <-timer.C:
		d.logger.Warn("graceful shutdown timed out after 30s")
	}
}

// Done returns a channel that is closed when the daemon has fully stopped.
func (d *Daemon) Done() <-chan struct{} {
	return d.done
}

// ActiveTasks returns the number of currently executing tasks.
func (d *Daemon) ActiveTasks() int64 {
	return d.activeTasks.Load()
}

// Uptime returns the duration since the daemon started.
func (d *Daemon) Uptime() time.Duration {
	if d.startTime.IsZero() {
		return 0
	}
	return time.Since(d.startTime)
}

// RuntimeIDs returns a copy of the registered runtime IDs.
func (d *Daemon) RuntimeIDs() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ids := make([]string, 0, len(d.runtimes))
	for id := range d.runtimes {
		ids = append(ids, id)
	}
	return ids
}

// StdinManager returns the daemon's StdinPipeManager for writing input
// to running task processes.
func (d *Daemon) StdinManager() *StdinPipeManager {
	return d.stdinManager
}

// ConsecutiveHeartbeatFailures returns the current count of consecutive
// heartbeat interval failures. Useful for testing and monitoring.
func (d *Daemon) ConsecutiveHeartbeatFailures() int {
	return d.consecutiveHeartbeatFailures
}

// register sends a registration request to the server.
func (d *Daemon) register(ctx context.Context) error {
	if d.client == nil {
		d.logger.Debug("no HTTP client configured, skipping registration")
		return nil
	}

	agents := make(map[string]string, len(d.cfg.Agents))
	for name, entry := range d.cfg.Agents {
		agents[name] = entry.Version
	}

	req := RegisterRequest{
		DaemonID:   d.cfg.DaemonID,
		DeviceName: d.cfg.DeviceName,
		Agents:     agents,
	}

	resp, err := d.client.Register(ctx, req)
	if err != nil {
		return fmt.Errorf("register with server: %w", err)
	}

	d.mu.Lock()
	d.runtimes = make(map[string]string, len(resp.RuntimeIDs))
	for provider, runtimeID := range resp.RuntimeIDs {
		d.runtimes[runtimeID] = provider
	}
	d.mu.Unlock()

	d.logger.Info("registered with server", "runtime_count", len(resp.RuntimeIDs))
	return nil
}

// deregister notifies the server that the daemon is going offline.
func (d *Daemon) deregister() {
	if d.client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := DeregisterRequest{
		DaemonID: d.cfg.DaemonID,
	}

	if err := d.client.Deregister(ctx, req); err != nil {
		d.logger.Warn("failed to deregister from server", "error", err)
	} else {
		d.logger.Info("deregistered from server")
	}
}

// heartbeatLoop sends periodic heartbeats to the server.
// On failure, it retries up to 3 times with a 5-second delay between attempts.
func (d *Daemon) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(d.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.sendHeartbeat(ctx)
		}
	}
}

// sendHeartbeat sends a single heartbeat with retry logic.
// Retries up to 3 times with a 5-second delay on failure.
// Tracks consecutive interval failures and escalates log level:
// - < 3 consecutive failures: warning-level log
// - >= 3 consecutive failures: error-level log indicating connectivity loss
// Resets the counter on any successful heartbeat.
func (d *Daemon) sendHeartbeat(ctx context.Context) {
	if d.client == nil {
		return
	}

	const maxRetries = 3

	// retryDelay is the delay between heartbeat retries.
	// Default 5s, but can be overridden for testing via heartbeatRetryDelay field.
	retryDelay := d.heartbeatRetryDelay
	if retryDelay == 0 {
		retryDelay = 5 * time.Second
	}

	req := d.BuildHeartbeatRequest()

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := d.client.Heartbeat(ctx, req)
		if err == nil {
			d.logger.Debug("heartbeat sent successfully")
			// Reset consecutive failure counter on success.
			d.consecutiveHeartbeatFailures = 0
			return
		}

		d.logger.Warn("heartbeat failed",
			"attempt", attempt,
			"max_retries", maxRetries,
			"error", err,
		)

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return
			case <-d.stopCh:
				return
			case <-time.After(retryDelay):
				// Continue to next retry.
			}
		}
	}

	// All retries within this interval exhausted — increment consecutive failure counter.
	d.consecutiveHeartbeatFailures++

	if d.consecutiveHeartbeatFailures >= 3 {
		d.logger.Error("heartbeat connectivity loss detected",
			"consecutive_failures", d.consecutiveHeartbeatFailures,
			"daemon_id", d.cfg.DaemonID,
		)
	} else {
		d.logger.Warn("heartbeat failed after all retries",
			"consecutive_failures", d.consecutiveHeartbeatFailures,
			"daemon_id", d.cfg.DaemonID,
		)
	}
}

// pollLoop polls the server for available tasks at the configured interval.
func (d *Daemon) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.pollForTasks(ctx)
		}
	}
}

// Output truncation limits.
const (
	// maxStdoutBytes is the maximum size of stdout output reported to the server (1 MB).
	maxStdoutBytes = 1 * 1024 * 1024
	// maxStderrChars is the maximum number of characters of stderr kept (last 4096).
	maxStderrChars = 4096
	// maxLocalBufferBytes is the maximum local output buffer size on disconnect (5 MB).
	maxLocalBufferBytes = 5 * 1024 * 1024
)

// pollForTasks checks for available tasks if under the concurrency limit.
func (d *Daemon) pollForTasks(ctx context.Context) {
	// Skip polling if at max concurrent tasks.
	if d.activeTasks.Load() >= int64(d.cfg.MaxConcurrentTasks) {
		d.logger.Debug("at max concurrent tasks, skipping poll",
			"active", d.activeTasks.Load(),
			"max", d.cfg.MaxConcurrentTasks,
		)
		return
	}

	if d.client == nil {
		return
	}

	d.mu.RLock()
	runtimeIDs := make([]string, 0, len(d.runtimes))
	for id := range d.runtimes {
		runtimeIDs = append(runtimeIDs, id)
	}
	d.mu.RUnlock()

	if len(runtimeIDs) == 0 {
		return
	}

	req := PollRequest{
		DaemonID:   d.cfg.DaemonID,
		RuntimeIDs: runtimeIDs,
	}

	resp, err := d.client.PollTasks(ctx, req)
	if err != nil {
		d.logger.Debug("poll for tasks failed", "error", err)
		return
	}

	if resp == nil || resp.TaskID == "" {
		// No tasks available.
		return
	}

	d.logger.Info("task claimed", "task_id", resp.TaskID, "agent_type", resp.AgentType)

	// Execute the task in a goroutine so the poll loop continues.
	d.activeTasks.Add(1)
	go func() {
		defer d.activeTasks.Add(-1)
		d.executeTask(ctx, resp)
	}()
}

// executeTask runs a claimed task through the full lifecycle:
// start → execute → complete/fail, with output streaming and truncation.
func (d *Daemon) executeTask(ctx context.Context, task *PollResponse) {
	// Use the structured agent backend for supported types.
	if isStructuredBackendSupported(task.AgentType) {
		d.executeTaskStructured(ctx, task)
		return
	}

	taskID := task.TaskID
	logger := d.logger.With("task_id", taskID, "agent_type", task.AgentType)

	// Resolve the binary path for the agent.
	agentEntry, ok := d.cfg.Agents[task.AgentType]
	if !ok {
		logger.Error("no agent entry found for type", "agent_type", task.AgentType)
		d.reportTaskFailure(ctx, taskID, "agent type not found: "+task.AgentType, -1)
		return
	}

	// Resolve agent configuration from the claim response.
	// When Agent is present, use its config for instructions, env, args, model.
	// When Agent is nil (backward compat), execute with prompt only.
	prompt := task.Prompt
	model := task.Model
	envVars := task.EnvVars

	if task.Agent != nil {
		agentName := task.Agent.Name
		logger = logger.With("agent_id", task.Agent.ID, "agent_name", agentName)

		// Model resolution: agent-level model overrides poll-level model.
		if task.Agent.Model != "" {
			model = task.Agent.Model
		}

		// Environment resolution: merge daemon (empty here), agent, and task env
		// with proper precedence (task > agent > daemon) and blocked key filtering.
		mergedEnv := execenv.MergeEnv(nil, task.Agent.CustomEnv, task.EnvVars, logger)
		envVars = mergedEnv

		// Runtime_Brief construction and injection is handled after workspace setup
		// (below) since hermes/unknown providers need to write AGENTS.md to the workspace.
	} else {
		// Backward compatible: no agent config, use task-level env vars as-is.
		logger.Debug("no agent config in task claim, executing with prompt only")
	}

	// Create the execution environment.
	execTask := execenv.Task{
		ID:           taskID,
		AgentType:    task.AgentType,
		Prompt:       prompt,
		Model:        model,
		ArgsTemplate: task.ArgsTemplate,
		EnvVars:      envVars,
		CustomArgs:   resolveCustomArgs(task),
	}

	execCfg := execenv.Config{
		WorkspacesRoot: d.cfg.WorkspacesRoot,
		AgentTimeout:   d.cfg.AgentTimeout,
	}

	env, err := execenv.NewExecEnv(execTask, execCfg, agentEntry.Path, logger)
	if err != nil {
		logger.Error("failed to create exec environment", "error", err)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("failed to create exec environment: %v", err), -1)
		return
	}

	// Setup workspace.
	if err := env.Setup(); err != nil {
		logger.Error("workspace setup failed", "error", err)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("workspace setup failed: %v", err), -1)
		return
	}

	// Runtime_Brief injection: construct brief from agent instructions and inject
	// via the provider-appropriate mechanism. This must happen after workspace setup
	// since hermes/unknown providers write AGENTS.md to the workspace directory.
	if task.Agent != nil && task.Agent.Instructions != "" {
		brief := BuildRuntimeBrief(task.Agent.Name, task.Agent.Instructions, "")
		opts := &ExecOptions{}
		injectedPrompt, briefErr := InjectBrief(task.AgentType, brief, prompt, env.WorkspaceDir, opts)
		if briefErr != nil {
			logger.Warn("failed to inject runtime brief", "error", briefErr)
		} else {
			prompt = injectedPrompt
			env.Prompt = prompt
			if opts.SystemPrompt != "" {
				env.SystemPrompt = opts.SystemPrompt
			}
		}
	}

	// Schedule cleanup after retention period (24h default, handled by GC loop).
	// The GC loop already handles cleanup based on modification time.

	// Report task start to server.
	if err := d.client.StartTask(ctx, taskID); err != nil {
		logger.Warn("failed to report task start", "error", err)
		// Continue execution even if start report fails.
	}

	// Create streaming writers that both buffer locally and report to server.
	stdoutBuf := &truncatingBuffer{maxBytes: maxStdoutBytes}
	stderrBuf := &tailBuffer{maxChars: maxStderrChars}

	// Create InputDetector for this task to detect when the CLI is waiting for input.
	// The onWaiting callback POSTs "waiting" state to the server.
	// The onCleared callback POSTs "cleared" state to the server.
	detector := NewInputDetector(
		InputDetectorConfig{},
		func() {
			// onWaiting: notify server that CLI is waiting for input.
			if d.client != nil {
				if err := d.client.ReportInputState(ctx, taskID, "waiting"); err != nil {
					logger.Debug("failed to report input state waiting", "error", err)
				}
			}
		},
		func() {
			// onCleared: notify server that CLI is no longer waiting for input.
			if d.client != nil {
				if err := d.client.ReportInputState(ctx, taskID, "cleared"); err != nil {
					logger.Debug("failed to report input state cleared", "error", err)
				}
			}
		},
	)
	defer detector.Stop()

	// streamingWriter wraps a buffer and sends chunks to the server as task messages.
	// It also feeds output to the InputDetector for prompt pattern analysis.
	sequence := 0

	// Register the sequence counter with the tracker so handleTaskInput
	// can share the same counter for stdin messages, ensuring all streams
	// (stdout, stderr, stdin) use monotonically increasing sequence numbers.
	d.sequences.Register(taskID, &sequence)
	defer d.sequences.Remove(taskID)

	streamWriter := func(stream string, buf io.Writer) io.Writer {
		return &streamingReporter{
			inner:    buf,
			client:   d.client,
			ctx:      ctx,
			taskID:   taskID,
			stream:   stream,
			sequence: &sequence,
			logger:   logger,
			detector: detector,
		}
	}

	stdoutWriter := streamWriter("stdout", stdoutBuf)
	stderrWriter := streamWriter("stderr", stderrBuf)

	// Run the agent process with timeout and stdin pipe support.
	taskCtx, taskCancel := context.WithTimeout(ctx, d.cfg.AgentTimeout)
	defer taskCancel()

	stdinPipe, done, startErr := env.RunWithStdin(taskCtx, stdoutWriter, stderrWriter)
	if startErr != nil {
		logger.Error("failed to start agent process", "error", startErr)
		d.reportTaskFailure(ctx, taskID, fmt.Sprintf("failed to start agent process: %v", startErr), -1)
		return
	}

	// Register the stdin pipe with the manager for bidirectional communication.
	d.stdinManager.Register(taskID, stdinPipe)
	defer d.stdinManager.Close(taskID)

	// Wait for the process to complete.
	result := <-done
	exitCode := result.ExitCode
	runErr := result.Err

	// Determine outcome.
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if taskCtx.Err() == context.DeadlineExceeded {
		// Task timed out.
		logger.Warn("task timed out", "timeout", d.cfg.AgentTimeout)
		errMsg := fmt.Sprintf("task timed out after %s", d.cfg.AgentTimeout)
		if stderr != "" {
			errMsg += "\nstderr: " + stderr
		}
		d.reportTaskFailure(ctx, taskID, errMsg, exitCode)
		return
	}

	if runErr != nil && ctx.Err() != nil {
		// Daemon is shutting down — don't report, let server handle orphan.
		logger.Info("task interrupted by daemon shutdown")
		return
	}

	if exitCode == 0 && runErr == nil {
		// Success.
		logger.Info("task completed successfully")
		if err := d.client.CompleteTask(ctx, taskID, stdout, exitCode); err != nil {
			logger.Warn("failed to report task completion", "error", err)
			d.bufferOutput(taskID, stdout, stderr, exitCode, true)
		}
	} else {
		// Failure.
		logger.Info("task failed", "exit_code", exitCode, "error", runErr)
		errMsg := stderr
		if errMsg == "" && runErr != nil {
			errMsg = runErr.Error()
		}
		d.reportTaskFailure(ctx, taskID, errMsg, exitCode)
	}
}

// reportTaskFailure reports a task failure to the server, buffering locally on disconnect.
func (d *Daemon) reportTaskFailure(ctx context.Context, taskID string, errMsg string, exitCode int) {
	if d.client == nil {
		return
	}
	if err := d.client.FailTask(ctx, taskID, errMsg, exitCode); err != nil {
		d.logger.Warn("failed to report task failure, buffering locally",
			"task_id", taskID, "error", err)
		d.bufferOutput(taskID, "", errMsg, exitCode, false)
	}
}

// bufferOutput stores task output locally when the server is unreachable.
// This implements the 5 MB local buffer requirement for disconnect scenarios.
func (d *Daemon) bufferOutput(taskID string, stdout, stderr string, exitCode int, success bool) {
	// Calculate total size.
	totalSize := len(stdout) + len(stderr)
	if totalSize > maxLocalBufferBytes {
		// Truncate stdout first to fit within buffer limit.
		maxStdout := maxLocalBufferBytes - len(stderr)
		if maxStdout < 0 {
			maxStdout = 0
			stderr = stderr[len(stderr)-maxLocalBufferBytes:]
		}
		if len(stdout) > maxStdout {
			stdout = stdout[:maxStdout]
		}
	}

	// Write to local buffer file for later flush.
	bufferDir := filepath.Join(d.cfg.WorkspacesRoot, ".buffers")
	if err := os.MkdirAll(bufferDir, 0o755); err != nil {
		d.logger.Warn("failed to create buffer directory", "error", err)
		return
	}

	status := "failed"
	if success {
		status = "completed"
	}

	content := fmt.Sprintf("task_id=%s\nstatus=%s\nexit_code=%d\n---stdout---\n%s\n---stderr---\n%s",
		taskID, status, exitCode, stdout, stderr)

	bufferFile := filepath.Join(bufferDir, taskID+".buf")
	if err := os.WriteFile(bufferFile, []byte(content), 0o644); err != nil {
		d.logger.Warn("failed to write buffer file", "task_id", taskID, "error", err)
	} else {
		d.logger.Info("task output buffered locally", "task_id", taskID, "file", bufferFile)
	}
}

// streamingReporter is an io.Writer that sends output chunks to the server
// as task messages while also writing to an inner buffer for final reporting.
// When an InputDetector is attached, it feeds each output chunk to the detector
// for input-waiting detection.
type streamingReporter struct {
	inner    io.Writer
	client   HTTPClient
	ctx      context.Context
	taskID   string
	stream   string // "stdout" or "stderr"
	sequence *int
	logger   *slog.Logger
	detector *InputDetector // optional: input detection on output chunks
}

func (s *streamingReporter) Write(p []byte) (n int, err error) {
	// Write to inner buffer first.
	n, err = s.inner.Write(p)

	// Send to server as a task message (best-effort, don't block on failure).
	if len(p) > 0 && s.client != nil {
		*s.sequence++
		msg := TaskMessage{
			Sequence: *s.sequence,
			Stream:   s.stream,
			Content:  string(p),
		}
		if reportErr := s.client.ReportMessages(s.ctx, s.taskID, []TaskMessage{msg}); reportErr != nil {
			s.logger.Debug("failed to report task message", "error", reportErr)
		}
	}

	// Feed output to the input detector for prompt pattern analysis.
	if len(p) > 0 && s.detector != nil {
		s.detector.OnOutput(string(p))
	}

	return n, err
}

// truncatingBuffer is an io.Writer that captures output up to a maximum byte limit.
// Once the limit is reached, additional writes are silently discarded.
type truncatingBuffer struct {
	buf      bytes.Buffer
	maxBytes int
}

func (tb *truncatingBuffer) Write(p []byte) (n int, err error) {
	remaining := tb.maxBytes - tb.buf.Len()
	if remaining <= 0 {
		// Already at capacity — discard but report success.
		return len(p), nil
	}
	if len(p) > remaining {
		tb.buf.Write(p[:remaining])
		return len(p), nil
	}
	tb.buf.Write(p)
	return len(p), nil
}

func (tb *truncatingBuffer) String() string {
	return tb.buf.String()
}

// tailBuffer is an io.Writer that keeps only the last N characters of output.
// This is used for stderr to capture the most recent error context.
type tailBuffer struct {
	data     []byte
	maxChars int
}

func (tb *tailBuffer) Write(p []byte) (n int, err error) {
	tb.data = append(tb.data, p...)
	// Trim to last maxChars characters.
	s := string(tb.data)
	if len([]rune(s)) > tb.maxChars {
		runes := []rune(s)
		s = string(runes[len(runes)-tb.maxChars:])
		tb.data = []byte(s)
	}
	return len(p), nil
}

func (tb *tailBuffer) String() string {
	return string(tb.data)
}

// gcLoop periodically cleans up old workspace directories.
func (d *Daemon) gcLoop(ctx context.Context) {
	// GC runs every 30 minutes.
	const gcInterval = 30 * time.Minute

	ticker := time.NewTicker(gcInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.runGC()
		}
	}
}

// runGC cleans up old workspace directories that have exceeded the retention period.
func (d *Daemon) runGC() {
	workspacesRoot := d.cfg.WorkspacesRoot
	if workspacesRoot == "" {
		return
	}

	entries, err := os.ReadDir(workspacesRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			d.logger.Debug("gc: failed to read workspaces directory", "error", err)
		}
		return
	}

	// Default retention: 24 hours.
	const retentionPeriod = 24 * time.Hour
	cutoff := time.Now().Add(-retentionPeriod)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(workspacesRoot, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				d.logger.Warn("gc: failed to remove workspace", "path", path, "error", err)
			} else {
				d.logger.Debug("gc: removed old workspace", "path", path)
			}
		}
	}
}

// --- PID File Management ---

// pidFilePath returns the path to the daemon PID file.
func pidFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".agenticflow", "daemon.pid"), nil
}

// WritePIDFile writes the current process PID to ~/.agenticflow/daemon.pid.
func (d *Daemon) WritePIDFile() error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create PID file directory: %w", err)
	}

	pid := os.Getpid()
	data := []byte(strconv.Itoa(pid))

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}

	d.logger.Debug("wrote PID file", "path", path, "pid", pid)
	return nil
}

// ReadPIDFile reads the PID from ~/.agenticflow/daemon.pid.
// Returns 0 and nil error if the file does not exist.
func ReadPIDFile() (int, error) {
	path, err := pidFilePath()
	if err != nil {
		return 0, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read PID file: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		return 0, nil
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("parse PID file: %w", err)
	}

	return pid, nil
}

// CleanStalePIDFile checks if the PID file references a running process.
// If the process is not running, it removes the stale PID file.
func (d *Daemon) CleanStalePIDFile() error {
	pid, err := ReadPIDFile()
	if err != nil {
		return err
	}
	if pid == 0 {
		// No PID file exists.
		return nil
	}

	// Check if the process is still running.
	if isProcessRunning(pid) {
		return fmt.Errorf("daemon already running with PID %d", pid)
	}

	// Process is not running — remove stale PID file.
	d.logger.Info("cleaning stale PID file", "stale_pid", pid)
	return d.RemovePIDFile()
}

// RemovePIDFile removes the daemon PID file.
func (d *Daemon) RemovePIDFile() error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove PID file: %w", err)
	}

	d.logger.Debug("removed PID file", "path", path)
	return nil
}

// isProcessRunning checks if a process with the given PID is currently running.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds. Send signal 0 to check if alive.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// TaskStatusFromExitCode maps a process exit code to the task status string.
// Exit code 0 means "completed"; any non-zero exit code means "failed".
func TaskStatusFromExitCode(exitCode int) string {
	if exitCode == 0 {
		return "completed"
	}
	return "failed"
}

// resolveCustomArgs extracts custom arguments from the task's agent config.
// Returns nil if no agent config is present or custom_args is empty.
func resolveCustomArgs(task *PollResponse) []string {
	if task.Agent == nil || len(task.Agent.CustomArgs) == 0 {
		return nil
	}
	return task.Agent.CustomArgs
}

// --- DaemonStateProvider implementation ---
// These methods satisfy the DaemonStateProvider interface defined in health.go,
// allowing the HealthServer to query daemon state without tight coupling.

// GetDaemonID returns the daemon's unique identifier.
func (d *Daemon) GetDaemonID() string {
	return d.cfg.DaemonID
}

// GetDeviceName returns the machine's device name.
func (d *Daemon) GetDeviceName() string {
	return d.cfg.DeviceName
}

// GetServerURL returns the configured server URL.
func (d *Daemon) GetServerURL() string {
	return d.cfg.ServerURL
}

// GetCLIVersion returns the CLI version string.
func (d *Daemon) GetCLIVersion() string {
	return d.cfg.CLIVersion
}

// GetActiveTaskCount returns the number of currently executing tasks.
func (d *Daemon) GetActiveTaskCount() int64 {
	return d.activeTasks.Load()
}

// GetAgents returns the list of detected agent runtimes.
func (d *Daemon) GetAgents() []AgentInfo {
	agents := make([]AgentInfo, 0, len(d.cfg.Agents))
	for name, entry := range d.cfg.Agents {
		agents = append(agents, AgentInfo{
			Name:    name,
			Version: entry.Version,
			Path:    entry.Path,
		})
	}
	return agents
}

// GetStartTime returns when the daemon started.
func (d *Daemon) GetStartTime() time.Time {
	return d.startTime
}

// BuildHeartbeatRequest constructs the heartbeat payload from the current daemon state.
// This is extracted as a public method to enable property-based testing of payload completeness.
func (d *Daemon) BuildHeartbeatRequest() HeartbeatRequest {
	runtimeNames := make([]string, 0, len(d.cfg.Agents))
	for name := range d.cfg.Agents {
		runtimeNames = append(runtimeNames, name)
	}

	return HeartbeatRequest{
		DaemonID:    d.cfg.DaemonID,
		Runtimes:    runtimeNames,
		ActiveTasks: d.activeTasks.Load(),
	}
}
