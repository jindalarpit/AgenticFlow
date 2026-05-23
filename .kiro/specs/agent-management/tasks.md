# Implementation Plan: Agent Management

## Overview

This plan implements the full agent management system for AgenticFlow, replacing the basic `custom_agent` table with a rich agent lifecycle. Implementation proceeds in layers: database migration first, then server-side handlers and services, daemon-side execution logic, Web UI pages, WebSocket events, and finally property-based tests for all 15 correctness properties.

## Tasks

- [x] 1. Database migration and schema setup
  - [x] 1.1 Create migration file `002_agent_table.up.sql`
    - Create the `agent` table with all fields, constraints, and indexes as defined in the design
    - Add `agent_id` column to the `task` table with foreign key reference
    - Add `token_prefix` column to `personal_access_token` table if not present
    - Migrate existing `custom_agent` data to the new `agent` table
    - Drop the `custom_agent` table after migration
    - _Requirements: 17.1, 17.2, 6.1, 16.1_

  - [x] 1.2 Create migration file `002_agent_table.down.sql`
    - Recreate `custom_agent` table with original schema
    - Migrate data back from `agent` table (best effort)
    - Remove `agent_id` from `task` table
    - Drop `agent` table
    - _Requirements: 17.3_

  - [x] 1.3 Add sqlc queries for agent operations
    - Add `CreateAgent`, `GetAgent`, `ListAgentsByUser`, `UpdateAgent`, `DeleteAgent` queries
    - Add `CountActiveTasksForAgent`, `GetAgentByName`, `ClaimPendingTaskForRuntime` queries
    - Run `sqlc generate` to produce type-safe Go code in `pkg/db/generated/`
    - _Requirements: 6.1, 8.1, 8.2, 15.1, 16.1_

- [x] 2. Server: PAT handler and auth
  - [x] 2.1 Implement PAT creation handler (`internal/handler/personal_access_token.go`)
    - Generate cryptographically random token with `af_` prefix (3 chars + 64 hex chars)
    - Store SHA-256 hash, token prefix (first 12 chars), name, creation timestamp, expiry
    - Validate name (non-empty, max 64 chars) and expiry option (30, 90, 365 days, or null)
    - Return full token value exactly once in response
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

  - [x] 2.2 Implement PAT list handler
    - Return all non-revoked PATs for authenticated user (max 100)
    - Include name, prefix, creation date, last-used timestamp, expiry date
    - Sort by creation date descending
    - _Requirements: 2.1, 2.4_

  - [x] 2.3 Implement PAT revocation handler
    - Delete token record from database on revocation request
    - Invalidate cached authentication state within same request-response cycle
    - Return success for non-existent or non-owned token IDs (idempotent)
    - _Requirements: 3.1, 3.3, 3.4_

  - [x] 2.4 Implement PAT authentication middleware update
    - Validate bearer tokens with `af_` prefix via SHA-256 hash lookup
    - Use in-memory cache with 5-minute TTL for token lookups
    - Update `last_used_at` timestamp asynchronously within 60 seconds
    - _Requirements: 2.3, 3.4_

  - [x] 2.5 Implement user registration handler (`internal/handler/auth.go`)
    - Validate email format (one `@`, domain with `.`, max 254 chars)
    - Validate password (8-128 chars) and name (1-128 chars after trim)
    - Create user with hashed password, return PAT with 90-day expiry
    - Reject duplicate emails with conflict error
    - _Requirements: 5.2, 5.3, 5.4, 5.5, 5.6, 5.7_

  - [x] 2.6 Write property tests for PAT generation (Property 1)
    - **Property 1: PAT Generation Structural Integrity**
    - Verify token format `af_` + 64 hex chars, hash = SHA-256(token), prefix = first 12 chars, expiry matches requested duration
    - **Validates: Requirements 1.1, 1.2, 1.3**

  - [x] 2.7 Write property tests for PAT name validation (Property 2)
    - **Property 2: PAT Name Validation**
    - Verify acceptance iff non-empty after trim and ≤64 chars; rejection for empty/whitespace-only
    - **Validates: Requirements 1.4, 1.5**

  - [x] 2.8 Write property tests for registration input validation (Property 5)
    - **Property 5: Registration Input Validation**
    - Verify acceptance iff email has one `@` + domain with `.` + ≤254 chars, password 8-128 chars, trimmed name 1-128 chars
    - **Validates: Requirements 5.2, 5.5, 5.6, 5.7**

- [x] 3. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Server: Agent CRUD handler
  - [x] 4.1 Implement agent creation handler (`internal/handler/agent.go`)
    - Validate all fields: name regex, description length, instructions length, model length
    - Validate custom_env (max 20 pairs, key 1-64 chars, value 1-1024 chars)
    - Validate custom_args (max 20 items, each max 256 chars)
    - Validate max_concurrent_tasks (1-20), visibility (private/shared)
    - Verify runtime_id exists, reject duplicate names per user
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8, 8.1, 8.5_

  - [x] 4.2 Implement agent list handler
    - Return all agents visible to user (owned + shared) with derived status
    - Include full agent response with status field
    - _Requirements: 8.2, 10.1_

  - [x] 4.3 Implement agent get, update, and delete handlers
    - Get: return single agent by ID with derived status
    - Update: validate changed fields, persist, broadcast `agent_updated` event
    - Delete: prevent new task assignments, allow in-flight tasks to complete, remove record
    - _Requirements: 8.3, 8.4, 8.6_

  - [x] 4.4 Implement default "Nexus" agent creation logic
    - On first runtime registration for a user with no agents, create "Nexus" agent
    - Set description "Your local AI coding agent", bind to first runtime, max_concurrent=1, visibility=private
    - Skip if "Nexus" already exists; log and continue on DB error (non-blocking)
    - _Requirements: 7.1, 7.2, 7.3, 7.4_

  - [x] 4.5 Write property tests for agent name validation (Property 6)
    - **Property 6: Agent Name Validation**
    - Verify acceptance iff name matches `^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`
    - **Validates: Requirements 6.2, 6.3**

  - [x] 4.6 Write property tests for agent custom env validation (Property 8)
    - **Property 8: Agent Custom Env Validation**
    - Verify acceptance iff ≤20 pairs, each key 1-64 chars, each value 1-1024 chars
    - **Validates: Requirements 6.5**

  - [x] 4.7 Write property tests for agent configuration round-trip (Property 7)
    - **Property 7: Agent Configuration Round-Trip**
    - Create agent with valid config, retrieve it, verify all fields match input
    - **Validates: Requirements 6.1, 8.1**

  - [x] 4.8 Write property tests for agent visibility filtering (Property 13)
    - **Property 13: Agent Visibility Filtering**
    - Verify list returns owned agents + shared agents from others; never private agents from others
    - **Validates: Requirements 8.2**

- [x] 5. Server: Agent status service and concurrency
  - [x] 5.1 Implement agent status derivation service (`internal/service/agent_status.go`)
    - Derive status: offline if runtime daemon offline, working if active tasks > 0, idle otherwise
    - Enforce priority order: offline > working > idle
    - Expose `DeriveStatus` and `ReconcileAndBroadcast` methods
    - _Requirements: 9.1, 9.2, 9.3, 9.6_

  - [x] 5.2 Implement concurrency enforcement in task claim logic
    - Skip agent when active tasks equal max_concurrent_tasks during task polling
    - Resume assignment when active count drops below limit
    - Enforce independently per agent (not coupled to daemon global cap)
    - _Requirements: 15.1, 15.2, 15.3_

  - [x] 5.3 Write property tests for agent status derivation (Property 9)
    - **Property 9: Agent Status Derivation**
    - Verify status computation for all combinations of runtime status and active task count
    - **Validates: Requirements 9.1, 9.2, 9.3, 9.6**

  - [x] 5.4 Write property tests for agent concurrency enforcement (Property 12)
    - **Property 12: Agent Concurrency Enforcement**
    - Verify no assignment when running count = max; resume when below; independent per agent
    - **Validates: Requirements 15.1, 15.2, 15.3**

- [x] 6. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 7. Daemon: Runtime_Brief construction and provider injection
  - [x] 7.1 Implement Runtime_Brief construction (`internal/daemon/brief.go`)
    - Build markdown document with agent name, instructions, and workspace context
    - Return empty string when instructions are empty
    - Sanitize agent name for markdown safety
    - _Requirements: 13.2, 13.9_

  - [x] 7.2 Implement provider-specific injection (`internal/daemon/inject.go`)
    - Route brief via `--append-system-prompt` for claude/pi
    - Route brief via `--prompt` for opencode
    - Prepend brief + `\n\n---\n\n` delimiter to prompt for openclaw/kiro/kimi
    - Set `developerInstructions` field for codex
    - Write `AGENTS.md` file for hermes and unknown providers
    - Skip injection entirely when instructions are empty
    - _Requirements: 13.3, 13.4, 13.5, 13.6, 13.7, 13.8, 13.9_

  - [x] 7.3 Implement environment resolution (`internal/daemon/execenv/env.go`)
    - Merge daemon, agent, and task env vars with precedence: task > agent > daemon
    - Block keys: HOME, PATH, USER, SHELL, TERM, and any `AF_` prefix
    - Log warning for blocked keys, skip without setting
    - _Requirements: 14.1, 14.2, 14.5, 14.6_

  - [x] 7.4 Implement custom args and model injection
    - Append agent custom_args after daemon-wide default arguments
    - Pass model override via provider-appropriate mechanism (--model flag or equivalent)
    - Fall back to daemon-wide model if agent model is empty
    - _Requirements: 14.3, 14.4_

  - [x] 7.5 Extend task claim response handling
    - Parse `TaskAgentData` from task claim response
    - Wire agent config (instructions, env, args, model) into task execution pipeline
    - Handle missing agent config gracefully (backward compatible, prompt-only execution)
    - _Requirements: 13.1, 14.6_

  - [x] 7.6 Write property tests for Runtime_Brief construction (Property 14)
    - **Property 14: Runtime_Brief Construction Completeness**
    - Verify brief contains agent name, instructions, workspace context when non-empty; empty when instructions empty
    - **Validates: Requirements 13.2, 13.9**

  - [x] 7.7 Write property tests for provider-specific injection (Property 10)
    - **Property 10: Provider-Specific Runtime_Brief Injection**
    - Verify correct injection mechanism per provider type; no injection when brief is empty
    - **Validates: Requirements 13.3, 13.4, 13.5, 13.6, 13.7, 13.8, 13.9**

  - [x] 7.8 Write property tests for environment variable resolution (Property 11)
    - **Property 11: Environment Variable Resolution with Blocked Keys**
    - Verify merge precedence (task > agent > daemon) and blocked key filtering
    - **Validates: Requirements 14.1, 14.2, 14.5**

  - [x] 7.9 Write property tests for custom args ordering (Property 15)
    - **Property 15: Custom Args Ordering**
    - Verify daemon defaults appear before agent custom_args in final invocation
    - **Validates: Requirements 14.3**

- [x] 8. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Web UI: Settings page and token management
  - [x] 9.1 Create Settings page with Tokens tab (`web/src/pages/Settings.tsx`)
    - Add Settings page accessible from main navigation at `/settings`
    - Implement tab navigation with "Tokens" tab
    - _Requirements: 18.1, 18.2_

  - [x] 9.2 Implement TokenManagement component (`web/src/components/TokenManagement.tsx`)
    - Display token list with name, masked prefix (first 4 chars + "••••••••"), dates, revoke button
    - Implement create form with name input and expiry dropdown (30d, 90d, 365d, Never)
    - Show empty state when no tokens exist
    - Visually distinguish expired tokens
    - _Requirements: 18.3, 18.4, 2.2, 2.4, 2.5_

  - [x] 9.3 Implement token creation modal with copy functionality
    - Display full unmasked token in modal after creation with copy-to-clipboard button
    - Show warning that token won't be shown again
    - Keep modal open until user explicitly closes it
    - Show visual confirmation on copy for at least 2 seconds
    - Handle clipboard write failure with error message, keep token visible
    - _Requirements: 4.1, 4.2, 4.3, 4.4_

  - [x] 9.4 Implement token revocation confirmation dialog
    - Show confirmation dialog identifying token name before revocation
    - Require explicit user confirmation before sending revocation request
    - _Requirements: 3.2_

- [x] 10. Web UI: Login and Registration
  - [x] 10.1 Update Login page with registration toggle (`web/src/pages/Login.tsx`)
    - Add toggle/link to switch between sign-in and registration forms
    - Implement registration form with email, password, name fields
    - Add inline validation matching server-side rules
    - On successful registration, store returned PAT and redirect to dashboard
    - _Requirements: 5.1, 5.2_

- [x] 11. Web UI: Agent pages
  - [x] 11.1 Implement Agent List page (`web/src/pages/AgentList.tsx`)
    - Display agent cards at route `/agents` with name, description, runtime, status badge, model
    - Color-code status badges (idle=green, working=amber, offline=gray)
    - Add "Create Agent" button navigating to `/agents/new`
    - Update status badges in real-time via WebSocket events
    - _Requirements: 10.1, 10.2, 10.3, 10.4_

  - [x] 11.2 Implement Agent Creation page (`web/src/pages/AgentForm.tsx`)
    - Create form at `/agents/new` with all fields: name, description, instructions, runtime dropdown, model, env key-value editor, args array editor, max concurrent tasks, visibility toggle
    - Filter runtime dropdown to online runtimes only
    - Implement inline validation matching server-side rules
    - Navigate to agent list on successful creation
    - _Requirements: 11.1, 11.2, 11.3, 11.4_

  - [x] 11.3 Implement Agent Detail/Edit page (`web/src/pages/AgentDetail.tsx`)
    - Display agent detail at `/agents/:id` with inline editing capability
    - Show recent task history (last 10 tasks) with status, prompt preview, duration, completion time
    - Display read-only mode for shared agents not owned by current user
    - Send update request on save, show success confirmation
    - _Requirements: 12.1, 12.2, 12.3, 12.4_

  - [x] 11.4 Implement task delegation agent selector
    - Add agent dropdown to task creation form showing available agents (idle or below concurrency limit)
    - Display selected agent's name, runtime type, and model as confirmation
    - _Requirements: 19.1, 19.4_

- [x] 12. WebSocket events for agent status
  - [x] 12.1 Add agent WebSocket events to server hub (`internal/realtime/`)
    - Broadcast `agent_created`, `agent_updated`, `agent_deleted`, `agent_status_changed` events
    - Trigger status recomputation on daemon connect/disconnect within 2 seconds
    - Trigger status recomputation on task status transitions within 2 seconds
    - _Requirements: 9.4, 9.5, 10.4_

  - [x] 12.2 Implement WebSocket event handlers in Web UI
    - Invalidate React Query `['agents']` cache on agent events
    - Update agent status badges in real-time without full page refresh
    - Handle reconnection by refetching agent data
    - _Requirements: 10.4, 9.4, 9.5_

- [x] 13. Integration wiring and task-agent association
  - [x] 13.1 Wire task creation to agent resolution
    - Store `agent_id` on task record when task targets a specific agent
    - Resolve agent's bound runtime for task enqueuing
    - Accept task as "pending" if agent's runtime is offline
    - _Requirements: 16.1, 19.2, 19.3_

  - [x] 13.2 Wire agent display in task views
    - Display agent name on task detail page
    - Display agent task history on agent detail page
    - _Requirements: 16.2, 16.3_

  - [x] 13.3 Wire CLI login with PAT (`af login --token`)
    - Validate token against server via Authorization header
    - Store token in `~/.agenticflow/config.json` on success
    - _Requirements: 1.7_

- [x] 14. Remaining property-based tests
  - [x] 14.1 Write property tests for token list completeness (Property 3)
    - **Property 3: Token List Completeness and Ordering**
    - Verify list returns exactly non-revoked tokens with correct fields, sorted by creation date descending
    - **Validates: Requirements 2.1**

  - [x] 14.2 Write property tests for token revocation immediacy (Property 4)
    - **Property 4: Token Revocation Immediacy**
    - Verify revoked tokens are rejected on auth; non-existent IDs return success
    - **Validates: Requirements 3.1, 3.3, 3.4**

- [x] 15. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests use the [rapid](https://github.com/flyingmutant/rapid) library for Go with minimum 100 iterations
- Server code is Go (Chi router, pgx/v5, sqlc), Web UI is TypeScript (Vite + React)
- All 15 correctness properties from the design document are covered by property test tasks
- Unit tests validate specific examples and edge cases alongside property tests

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2"] },
    { "id": 1, "tasks": ["1.3"] },
    { "id": 2, "tasks": ["2.1", "2.5"] },
    { "id": 3, "tasks": ["2.2", "2.3", "2.4", "2.6", "2.7", "2.8"] },
    { "id": 4, "tasks": ["4.1", "4.4", "5.1"] },
    { "id": 5, "tasks": ["4.2", "4.3", "4.5", "4.6", "5.2"] },
    { "id": 6, "tasks": ["4.7", "4.8", "5.3", "5.4"] },
    { "id": 7, "tasks": ["7.1", "7.3"] },
    { "id": 8, "tasks": ["7.2", "7.4", "7.5", "7.6", "7.8", "7.9"] },
    { "id": 9, "tasks": ["7.7"] },
    { "id": 10, "tasks": ["9.1", "10.1", "11.1"] },
    { "id": 11, "tasks": ["9.2", "9.3", "9.4", "11.2", "11.3"] },
    { "id": 12, "tasks": ["11.4", "12.1"] },
    { "id": 13, "tasks": ["12.2", "13.1", "13.3"] },
    { "id": 14, "tasks": ["13.2", "14.1", "14.2"] }
  ]
}
```
