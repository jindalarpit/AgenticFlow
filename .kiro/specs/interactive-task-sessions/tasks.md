# Implementation Plan: Interactive Task Sessions

## Overview

This plan implements bidirectional stdin communication between the Web UI and CLI processes spawned by the daemon. The implementation proceeds bottom-up: database schema changes first, then daemon stdin pipe management, input detection, server API endpoints, session state management, and finally the Web UI components. Each group builds on the previous, with property-based tests validating correctness properties from the design.

## Tasks

- [ ] 1. Database schema and query changes
  - [ ] 1.1 Create migration to add stdin stream support and unique constraint
    - Add migration file to `server/migrations/` that:
      - Updates the CHECK constraint on `task_message.stream` to include `'stdin'`
      - Creates unique index `idx_task_message_task_sequence` on `task_message(task_id, sequence)`
    - _Requirements: 8.3, 8.6_

  - [ ] 1.2 Add sqlc queries for idempotent message insert and daemon lookup
    - Add `CreateTaskMessageIdempotent` query using `ON CONFLICT (task_id, sequence) DO NOTHING`
    - Add `GetTaskDaemonID` query to resolve daemon_id for a running task
    - Run `sqlc generate` to produce Go code in `server/pkg/db/generated/`
    - _Requirements: 8.2, 8.6, 2.2_

- [ ] 2. Daemon: Stdin Pipe Manager
  - [ ] 2.1 Implement StdinPipeManager in `server/internal/daemon/stdin.go`
    - Create `StdinPipeManager` struct with per-task pipe map and RWMutex
    - Implement `Register(taskID, pipe)` to store a pipe for a task
    - Implement `Write(taskID, text)` with per-task mutex serialization and newline appending
    - Implement `Close(taskID)` to close pipe and remove from map
    - Implement `EnsureNewline(text)` helper function
    - _Requirements: 1.1, 1.2, 1.3, 1.5, 1.6_

  - [ ]* 2.2 Write property test: Newline Appending Preserves Content
    - **Property 1: Newline Appending Preserves Content**
    - Test file: `server/internal/daemon/stdin_property_test.go`
    - For any input text, `EnsureNewline` result ends with exactly one `\n`, contains original text as prefix, and has correct length
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 1.5**

  - [ ]* 2.3 Write property test: Stdin Write Serialization
    - **Property 11: Stdin Write Serialization**
    - Test file: `server/internal/daemon/stdin_property_test.go`
    - For any set of concurrent writes to the same task, each input appears as a contiguous block with no interleaving
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 1.6**

  - [ ]* 2.4 Write property test: Task Input Isolation
    - **Property 9: Task Input Isolation**
    - Test file: `server/internal/daemon/stdin_property_test.go`
    - For concurrent tasks on the same daemon, input for task A is written exclusively to task A's pipe
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 9.1, 9.2**

- [ ] 3. Daemon: Input Detector
  - [ ] 3.1 Implement InputDetector in `server/internal/daemon/inputdetect.go`
    - Create `InputDetector` struct with configurable patterns and inactivity timeout
    - Implement `OnOutput(content)` that checks prompt patterns and resets inactivity timer
    - Implement `matchesPromptPattern(content)` for suffix-based pattern matching
    - Implement `signalWaiting()` and `Stop()` lifecycle methods
    - Support configurable additional patterns and timeout (min 3s, max 60s, default 10s)
    - _Requirements: 5.1, 5.2, 5.4, 5.5, 5.6_

  - [ ]* 3.2 Write property test: Prompt Pattern Detection
    - **Property 4: Prompt Pattern Detection**
    - Test file: `server/internal/daemon/inputdetect_property_test.go`
    - For any output ending with a recognized pattern, detector signals waiting; for non-matching output, no signal from pattern matching
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 5.1**

  - [ ]* 3.3 Write property test: Input Detection State Cleared on Output
    - **Property 5: Input Detection State Cleared on Output**
    - Test file: `server/internal/daemon/inputdetect_property_test.go`
    - For any task in waiting state, new output clears the waiting state
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 5.4**

- [ ] 4. Daemon: ExecEnv and task_input event handling
  - [ ] 4.1 Modify ExecEnv to create stdin pipe on process spawn
    - Add `RunWithStdin` method to `server/internal/daemon/execenv/execenv.go`
    - Returns `io.WriteCloser` (stdin pipe) alongside exit code and error
    - Update task execution flow to call `RunWithStdin` and register pipe with `StdinPipeManager`
    - _Requirements: 1.1, 1.2, 1.3_

  - [ ] 4.2 Implement WebSocket `task_input` event handler in daemon
    - Add `handleTaskInput(taskID, text)` to `server/internal/daemon/daemon.go`
    - Write input to stdin pipe via `StdinPipeManager.Write()`
    - On success: report stdin message to server with stream type "stdin"
    - On failure: log warning and report failure to server
    - Close stdin pipe on task terminal status
    - _Requirements: 3.1, 3.2, 3.4, 3.5, 1.4_

  - [ ] 4.3 Integrate InputDetector with task output streaming
    - Create InputDetector per task when spawning CLI process
    - Call `InputDetector.OnOutput(content)` on each stdout/stderr chunk
    - Wire `onWaiting` callback to POST input state "waiting" to server
    - Wire `onCleared` callback to POST input state "cleared" to server
    - Call `InputDetector.Stop()` on task completion
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

- [ ] 5. Checkpoint - Daemon implementation complete
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 6. Server: Task Input API and Session State
  - [ ] 6.1 Implement `POST /api/tasks/{id}/input` handler in `server/internal/handler/task_input.go`
    - Parse and validate JSON body (text field)
    - Validate text: non-empty, ≤ 10,000 characters (return 400 on failure)
    - Load task from DB, verify user ownership (return 403 on failure)
    - Verify task status == "running" (return 409 if not)
    - Resolve daemon_id, check daemon online (return 502 if offline)
    - Send `task_input` event to daemon via `Hub.SendToDaemon()`
    - Return HTTP 202 with confirmation payload
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 9.3_

  - [ ]* 6.2 Write property test: Input Text Validation
    - **Property 2: Input Text Validation**
    - Test file: `server/internal/handler/task_input_property_test.go`
    - For any string, server accepts iff trimmed is non-empty and ≤ 10,000 chars; rejects with 400 otherwise
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 2.5, 2.6**

  - [ ]* 6.3 Write property test: Non-Running Task Input Rejection
    - **Property 3: Non-Running Task Input Rejection**
    - Test file: `server/internal/handler/task_input_property_test.go`
    - For any task not in "running" status, input submission returns 409 regardless of text content
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 2.3**

  - [ ]* 6.4 Write property test: Task Ownership Authorization
    - **Property 10: Task Ownership Authorization**
    - Test file: `server/internal/handler/task_input_property_test.go`
    - For any (user, task) pair where user doesn't own the task, input is rejected with authorization error
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 9.3**

  - [ ] 6.5 Implement SessionStateManager in `server/internal/service/session_state.go`
    - Create in-memory `SessionStateManager` with RWMutex-protected map
    - Implement `SetState(taskID, state)` with WebSocket broadcast on change
    - Implement `GetState(taskID)` returning current state (default "idle")
    - Implement `ClearState(taskID)` for terminal task transitions
    - _Requirements: 7.1, 7.2, 7.3, 7.4_

  - [ ]* 6.6 Write property test: Session State Machine Validity
    - **Property 6: Session State Machine Validity**
    - Test file: `server/internal/service/session_state_property_test.go`
    - For any sequence of events, resulting state is always one of "idle", "producing_output", or "waiting_for_input"
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 7.1, 7.2, 7.3, 7.4**

  - [ ] 6.7 Implement daemon input-state endpoint in `server/internal/handler/daemon.go`
    - Add `POST /api/daemon/tasks/{taskId}/input-state` handler
    - Parse state ("waiting" or "cleared") from request body
    - Update SessionStateManager and broadcast WebSocket events (`input_requested` / `input_cleared`)
    - _Requirements: 5.3, 7.2, 7.3_

  - [ ] 6.8 Update ReportTaskMessages handler to accept "stdin" stream type
    - Modify stream validation in `server/internal/handler/daemon.go` to accept "stdin"
    - Use `CreateTaskMessageIdempotent` query for duplicate prevention
    - Broadcast `task_output` event with stream field set to "stdin"
    - _Requirements: 6.1, 6.2, 8.2, 8.3, 8.6_

  - [ ] 6.9 Register new routes in `server/cmd/server/router.go`
    - Add `POST /api/tasks/{id}/input` to protected routes
    - Add `POST /api/daemon/tasks/{taskId}/input-state` to daemon routes
    - _Requirements: 2.1_

- [ ] 7. Server: Message persistence and retrieval
  - [ ] 7.1 Update `GET /api/tasks/{id}/messages` to include stdin stream
    - Ensure query returns all stream types (stdout, stderr, stdin) ordered by sequence number
    - Verify response includes stream field for each message
    - _Requirements: 8.4, 8.5_

  - [ ]* 7.2 Write property test: Message Ordering by Sequence Number
    - **Property 7: Message Ordering by Sequence Number**
    - Test file: `server/internal/handler/task_messages_property_test.go`
    - For any set of messages with distinct sequence numbers, API returns them in ascending order regardless of stream type
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 6.4, 8.4, 8.5**

  - [ ]* 7.3 Write property test: Duplicate Message Idempotency
    - **Property 8: Duplicate Message Idempotency**
    - Test file: `server/internal/handler/task_messages_property_test.go`
    - For any message submitted multiple times (same task_id + sequence), exactly one copy is stored
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 8.6**

  - [ ]* 7.4 Write property test: Stdin Message Reporting Round-Trip
    - **Property 12: Stdin Message Reporting Round-Trip**
    - Test file: `server/internal/handler/task_messages_property_test.go`
    - For any input written to stdin pipe, daemon reports message with stream "stdin" and server persists and broadcasts it
    - Use `pgregory.net/rapid` with minimum 100 iterations
    - **Validates: Requirements 3.4, 6.1, 6.2**

- [ ] 8. Checkpoint - Server implementation complete
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 9. Web UI: Task Input Component and Hooks
  - [ ] 9.1 Create `useSendTaskInput` mutation hook in `web/src/hooks/useTaskInput.ts`
    - Implement React Query mutation calling `POST /api/tasks/{id}/input`
    - Accept `{ taskId, text }` params, return typed response
    - _Requirements: 4.2_

  - [ ] 9.2 Create `useSessionState` hook in `web/src/hooks/useSessionState.ts`
    - Listen for WebSocket events: `input_requested`, `input_cleared`, `session_state_changed`
    - Track and return current `SessionState` ("idle" | "producing_output" | "waiting_for_input")
    - Filter events by taskId
    - _Requirements: 7.5, 5.3_

  - [ ] 9.3 Create `TaskInput` component in `web/src/components/TaskInput.tsx`
    - Render input field + submit button when task is running
    - Hide when task is in terminal status
    - Highlight input field (yellow border/ring) when `isWaitingForInput` is true
    - Disable submit button while mutation is in-flight
    - On success: clear input field
    - On error: preserve text, show error message
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7_

  - [ ] 9.4 Integrate TaskInput into TaskDetail page at `web/src/pages/TaskDetail.tsx`
    - Import and render `TaskInput` below terminal output area
    - Pass `taskId`, `isRunning`, and `isWaitingForInput` props from `useSessionState`
    - _Requirements: 4.1_

- [ ] 10. Web UI: Stdin message rendering
  - [ ] 10.1 Update terminal output rendering to handle "stdin" stream type
    - Add distinct visual style for stdin messages (different color/prefix, e.g., green with ">" prefix)
    - Render stdin messages interleaved with stdout/stderr by sequence number
    - Update `TaskMessage` TypeScript interface to include `"stdin"` in stream union type
    - _Requirements: 6.3, 6.4, 4.3_

  - [ ] 10.2 Add "Waiting for input..." badge to task status area
    - Show visual indicator when session state is "waiting_for_input"
    - Clear badge when state transitions away from waiting
    - _Requirements: 7.5_

- [ ] 11. Checkpoint - Web UI implementation complete
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 12. Integration wiring and WebSocket event plumbing
  - [ ] 12.1 Wire `Hub.SendToDaemon` for `task_input` events
    - Ensure the WebSocket hub can route `task_input` events to the correct daemon connection by daemon_id
    - Add `task_input` to the set of recognized server-to-daemon event types
    - _Requirements: 2.2, 3.3_

  - [ ] 12.2 Wire session state updates on task output and terminal transitions
    - On new task output received by server: set session state to "producing_output"
    - On task terminal status transition: call `SessionStateManager.ClearState()`
    - _Requirements: 7.3, 7.4_

  - [ ] 12.3 Update daemon task completion flow to close stdin pipe
    - Call `StdinPipeManager.Close(taskID)` when task reaches terminal status
    - Ensure InputDetector is stopped on task completion
    - _Requirements: 1.3, 5.5_

- [ ] 13. Final checkpoint - Full integration verified
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document using `pgregory.net/rapid`
- Unit tests validate specific examples and edge cases
- The daemon already has a WebSocket connection to the server hub — no new transport needed
- Session state is in-memory only (not persisted to DB) per design decision
- The existing `task_message` table is reused with "stdin" as a new valid stream type

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2"] },
    { "id": 2, "tasks": ["2.1", "3.1"] },
    { "id": 3, "tasks": ["2.2", "2.3", "2.4", "3.2", "3.3"] },
    { "id": 4, "tasks": ["4.1", "4.2", "4.3"] },
    { "id": 5, "tasks": ["6.1", "6.5", "6.7", "6.8", "6.9"] },
    { "id": 6, "tasks": ["6.2", "6.3", "6.4", "6.6", "7.1"] },
    { "id": 7, "tasks": ["7.2", "7.3", "7.4", "12.1"] },
    { "id": 8, "tasks": ["9.1", "9.2"] },
    { "id": 9, "tasks": ["9.3", "10.1", "10.2"] },
    { "id": 10, "tasks": ["9.4", "12.2", "12.3"] }
  ]
}
```
