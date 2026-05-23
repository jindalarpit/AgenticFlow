# Requirements Document

## Introduction

Interactive Task Sessions extends AgenticFlow's task execution model from one-shot (fire-and-forget) to bidirectional. Currently, when a task is delegated to an agent CLI, the daemon spawns the process with a prompt, streams stdout/stderr to the web UI, and reports completion — but there is no mechanism to send input back to the running CLI process. This feature adds a bidirectional stdin pipe so users can respond to CLI questions (e.g., permission prompts, clarification requests) directly from the web UI during task execution.

## Glossary

- **Daemon**: The background process running on the user's local machine that spawns agent CLI processes and streams their output to the Server
- **Server**: The Go HTTP backend that manages task state, relays input/output between the Web_UI and Daemon, and broadcasts events via WebSocket
- **Web_UI**: The browser-based interface where users view streaming task output and submit input responses
- **CLI_Process**: The spawned agent CLI binary (e.g., claude, kiro, gemini) executing a task within an isolated workspace
- **Stdin_Pipe**: A writable pipe connected to the CLI_Process's standard input, allowing the Daemon to write user-provided text to the running process
- **Input_Request**: A signal indicating the CLI_Process is waiting for user input, detected by output pattern analysis or inactivity timeout
- **Task_Input**: A user-submitted text response sent from the Web_UI through the Server to the Daemon and written to the CLI_Process's Stdin_Pipe
- **Input_Detector**: The Daemon subsystem that analyzes CLI_Process output to determine when the process is waiting for user input
- **Session_State**: The runtime state tracking whether a task's CLI_Process is actively waiting for input, producing output, or has completed

## Requirements

### Requirement 1: Bidirectional Stdin Pipe

**User Story:** As a developer, I want the daemon to keep stdin open on spawned CLI processes, so that user responses can be written to the running process during task execution.

#### Acceptance Criteria

1. WHEN the Daemon spawns a CLI_Process for a task, THE Daemon SHALL create a Stdin_Pipe connected to the process's standard input
2. WHILE a task is in "running" status, THE Daemon SHALL keep the Stdin_Pipe open and writable
3. WHEN a task transitions to a terminal status (completed, failed, cancelled, or timeout), THE Daemon SHALL close the Stdin_Pipe
4. IF the Stdin_Pipe write fails due to a broken pipe (process exited), THEN THE Daemon SHALL log the failure and discard the input without crashing
5. WHEN the Daemon writes Task_Input to the Stdin_Pipe, THE Daemon SHALL append a newline character to the input text if the text does not already end with a newline
6. THE Daemon SHALL serialize concurrent writes to the same task's Stdin_Pipe so that input messages are written atomically without interleaving

### Requirement 2: Task Input API

**User Story:** As a developer, I want an API endpoint to send input to a running task, so that the web UI can relay my responses to the CLI process.

#### Acceptance Criteria

1. THE Server SHALL expose a `POST /api/tasks/{id}/input` endpoint that accepts a JSON body containing a `text` field
2. WHEN the Server receives a valid input request for a task in "running" status, THE Server SHALL relay the input text to the Daemon executing that task
3. IF the Server receives an input request for a task that is not in "running" status, THEN THE Server SHALL reject the request with HTTP 409 and an error message indicating the task is not running
4. IF the Server receives an input request for a task whose Daemon is offline, THEN THE Server SHALL reject the request with HTTP 502 and an error message indicating the daemon is unreachable
5. THE Server SHALL validate that the input text is non-empty and does not exceed 10,000 characters
6. IF the input text is empty or exceeds 10,000 characters, THEN THE Server SHALL reject the request with HTTP 400 and an error message indicating the validation failure
7. WHEN the Server successfully relays input to the Daemon, THE Server SHALL return HTTP 202 with a confirmation payload

### Requirement 3: Daemon Input Relay

**User Story:** As a developer, I want the daemon to receive input messages from the server and write them to the correct CLI process, so that my responses reach the running agent.

#### Acceptance Criteria

1. WHEN the Daemon receives an input message from the Server for a running task, THE Daemon SHALL write the input text to that task's Stdin_Pipe within 1 second of receipt
2. IF the Daemon receives an input message for a task that is not currently executing, THEN THE Daemon SHALL discard the message and log a warning
3. THE Daemon SHALL support receiving input messages via its existing polling mechanism or WebSocket connection to the Server
4. WHEN the Daemon writes input to the Stdin_Pipe, THE Daemon SHALL record the input as a task message with stream type "stdin" and broadcast it to the Server for display in the Web_UI
5. IF the Daemon fails to write input to the Stdin_Pipe, THEN THE Daemon SHALL report the failure to the Server with an error message indicating the write failed

### Requirement 4: Web UI Input Field

**User Story:** As a developer, I want an input field on the task detail page, so that I can type and send responses when the CLI is waiting for input.

#### Acceptance Criteria

1. WHILE a task is in "running" status, THE Web_UI SHALL display an input field below the terminal output area on the task detail page
2. WHEN the user submits text via the input field, THE Web_UI SHALL send the text to the Server via `POST /api/tasks/{id}/input` and clear the input field
3. WHEN the Web_UI receives confirmation that input was delivered, THE Web_UI SHALL display the submitted text in the terminal output area with a distinct visual style indicating it is user input
4. IF the input submission fails, THEN THE Web_UI SHALL display an error message near the input field indicating the failure reason and preserve the user's text in the input field
5. WHILE the task is in a terminal status (completed, failed, cancelled, or timeout), THE Web_UI SHALL hide the input field
6. THE Web_UI SHALL disable the submit button while an input submission is in-flight to prevent duplicate sends
7. WHEN the Input_Detector signals that the CLI_Process is waiting for input, THE Web_UI SHALL visually highlight the input field to draw the user's attention

### Requirement 5: Input Detection

**User Story:** As a developer, I want the system to detect when the CLI is waiting for input, so that I know when to provide a response.

#### Acceptance Criteria

1. WHEN the CLI_Process output ends with a recognized prompt pattern (a line ending with "?", ": ", "> ", or "$ "), THE Input_Detector SHALL signal an Input_Request state
2. WHEN the CLI_Process has not produced any output for a configurable inactivity threshold (default 10 seconds, minimum 3 seconds, maximum 60 seconds), THE Input_Detector SHALL signal a potential Input_Request state
3. WHEN the Input_Detector signals an Input_Request, THE Daemon SHALL notify the Server, which SHALL broadcast an "input_requested" WebSocket event to the Web_UI
4. WHEN the CLI_Process resumes producing output after an Input_Request was signaled, THE Input_Detector SHALL clear the Input_Request state and THE Server SHALL broadcast an "input_cleared" WebSocket event
5. IF the CLI_Process exits while an Input_Request is active, THEN THE Input_Detector SHALL clear the Input_Request state
6. THE Input_Detector SHALL support a configurable list of additional prompt patterns via daemon configuration

### Requirement 6: Input Message Streaming

**User Story:** As a developer, I want to see my submitted inputs in the task output stream, so that I have a complete record of the interaction.

#### Acceptance Criteria

1. WHEN the Daemon writes Task_Input to the Stdin_Pipe, THE Daemon SHALL report the input text to the Server as a task message with stream type "stdin"
2. WHEN the Server receives a task message with stream type "stdin", THE Server SHALL broadcast it via WebSocket as a "task_output" event with the stream field set to "stdin"
3. THE Web_UI SHALL render "stdin" messages in the terminal output area with a distinct visual style (different color or prefix) distinguishing them from stdout and stderr messages
4. THE Web_UI SHALL display "stdin" messages in chronological order interleaved with stdout and stderr messages based on their sequence number

### Requirement 7: Session State Management

**User Story:** As a developer, I want the system to track whether a task is waiting for input, so that the UI can reflect the current interaction state.

#### Acceptance Criteria

1. THE Server SHALL maintain a Session_State for each running task with possible values: "producing_output", "waiting_for_input", and "idle"
2. WHEN the Server receives an "input_requested" notification from the Daemon, THE Server SHALL update the task's Session_State to "waiting_for_input" and broadcast the state change via WebSocket
3. WHEN the Server receives new task output or an "input_cleared" notification, THE Server SHALL update the task's Session_State to "producing_output" and broadcast the state change via WebSocket
4. WHEN a task transitions to a terminal status, THE Server SHALL clear the Session_State for that task
5. THE Web_UI SHALL reflect the current Session_State by showing a visual indicator (e.g., "Waiting for input..." badge) when the state is "waiting_for_input"

### Requirement 8: Output and Input Persistence

**User Story:** As a developer, I want all task output (stdout, stderr) and my submitted inputs (stdin) to be persisted, so that I can review the full interaction history after task completion.

#### Acceptance Criteria

1. THE Server SHALL persist all task messages (stdout, stderr, and stdin streams) in the database as they are received from the Daemon
2. WHEN the Daemon streams stdout or stderr output from the CLI_Process, THE Server SHALL store each message chunk with its sequence number, stream type, content, and timestamp
3. THE Server SHALL persist task messages with stream type "stdin" in the same storage as stdout and stderr messages
4. WHEN the user views a completed task's output, THE Web_UI SHALL display all messages (stdout, stderr, and stdin) interleaved in their original chronological order
5. THE Server SHALL include all stream types (stdout, stderr, stdin) in the response to `GET /api/tasks/{id}/messages`, ordered by sequence number
6. IF the Server receives duplicate messages (same task ID and sequence number), THEN THE Server SHALL ignore the duplicate and preserve the original message

### Requirement 9: Concurrent Task Input Isolation

**User Story:** As a developer, I want input sent to one task to never reach another task's CLI process, so that concurrent tasks remain isolated.

#### Acceptance Criteria

1. WHEN the Daemon receives an input message, THE Daemon SHALL route it exclusively to the Stdin_Pipe of the task identified by the task ID in the message
2. IF the Daemon is executing multiple tasks concurrently, THEN input messages for each task SHALL be written only to that task's Stdin_Pipe with no cross-contamination between tasks
3. THE Server SHALL validate that the authenticated user owns the task before relaying input to the Daemon

