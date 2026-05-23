---
inclusion: fileMatch
fileMatchPattern: "**/daemon/**,**/cmd/af/**"
---

# Daemon Implementation Patterns

Reference: `/Users/arpit.jindal/workspace/opensource/multica/server/internal/daemon/`

## Config Resolution (from multica/server/internal/daemon/config.go)

Follow the exact same pattern as multica's `LoadConfig()`:

```go
// Resolution order: CLI flags > env vars > config file > defaults
func LoadConfig(overrides Overrides) (Config, error) {
    // 1. Read env var with envOrDefault()
    // 2. Apply override if non-zero
    // 3. Return final config
}
```

Environment variable naming: `AF_<SETTING>` (not `MULTICA_`)
- `AF_SERVER_URL` (default: `http://localhost:8080`)
- `AF_DAEMON_POLL_INTERVAL` (default: `3s`)
- `AF_DAEMON_HEARTBEAT_INTERVAL` (default: `15s`)
- `AF_AGENT_TIMEOUT` (default: `2h`)
- `AF_DAEMON_MAX_CONCURRENT_TASKS` (default: `5`)
- `AF_<AGENT>_PATH` — custom binary path per agent
- `AF_<AGENT>_MODEL` — model override per agent

## Agent Detection (from multica/server/internal/daemon/config.go)

Use the same `probe()` pattern:

```go
probe := func(envVar, defaultCmd, modelEnv string) (AgentEntry, bool) {
    cmd := envOrDefault(envVar, defaultCmd)
    if _, err := exec.LookPath(cmd); err == nil {
        return AgentEntry{Path: cmd, Model: os.Getenv(modelEnv)}, true
    }
    // Shell fallback for GUI-launched daemons (same as multica)
    if path, ok := getShellResolved()[cmd]; ok {
        return AgentEntry{Path: path, Model: os.Getenv(modelEnv)}, true
    }
    return AgentEntry{}, false
}
```

Supported agents (same list as multica):
- claude, codex, opencode, openclaw, hermes, gemini, pi, cursor-agent, copilot, kimi, kiro-cli

## Daemon Run Loop (from multica/server/internal/daemon/daemon.go)

The `Daemon.Run()` method must follow this exact sequence:

```go
func (d *Daemon) Run(ctx context.Context) error {
    // 1. Bind health port (detect another running daemon)
    // 2. Resolve auth (load PAT from config)
    // 3. Register runtimes with server
    // 4. Start background loops:
    //    - heartbeatLoop (every 15s)
    //    - pollLoop (every 3s, claim + execute tasks)
    //    - gcLoop (workspace cleanup)
    // 5. Deregister runtimes on shutdown (defer)
}
```

## Task Execution (from multica/server/internal/daemon/execenv/)

Each task runs in an isolated workspace:

```go
// 1. Create workspace dir: ~/.agenticflow/workspaces/<task-id>/
// 2. Set up environment (env vars, model override)
// 3. Build command args from template ({{prompt}}, {{workspace}}, {{model}})
// 4. Spawn agent CLI process with working dir = workspace
// 5. Stream stdout/stderr to server via WebSocket
// 6. On completion: report status + output to server
// 7. On timeout: SIGTERM → wait 10s → SIGKILL
```

## Registration Request (from multica/server/internal/handler/daemon.go)

The daemon register request must include:

```json
{
    "daemon_id": "machine-stable-uuid",
    "device_name": "hostname",
    "cli_version": "0.1.0",
    "runtimes": [
        {
            "name": "Claude (MacBook-Pro)",
            "type": "claude",
            "version": "1.0.0",
            "status": "online"
        }
    ]
}
```

## Heartbeat Pattern

- Send POST to `/api/daemon/heartbeat` every 15s
- Include runtime_ids in the heartbeat
- If server responds 404 for a runtime → re-register
- If server unreachable → retry 3× with 5s delay, log warning

## PID File Management

- Write PID to `~/.agenticflow/daemon.pid` on start
- Check for stale PID on startup (process not running → clean up)
- Remove PID file on graceful stop
- Force-kill + remove PID if stop times out (30s)
