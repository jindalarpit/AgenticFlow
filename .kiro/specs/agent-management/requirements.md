# Requirements Document

## Introduction

This feature enhances AgenticFlow with a complete agent management system modeled after multica's patterns. It replaces the existing basic "Custom Agents" (CLI command wrappers) with a full agent lifecycle: rich agent configuration with instructions/system prompts, browser-based Personal Access Token management, user registration, prompt routing through agent instructions, and agent activity tracking. The system enables users to define agents with personas, bind them to detected runtimes, and have the daemon resolve and inject agent configuration (instructions, environment, arguments, model) when executing tasks.

## Glossary

- **Agent**: A user-defined AI agent configuration with a name, instructions (system prompt), runtime binding, model override, custom environment variables, custom arguments, and concurrency settings
- **Agent_Instructions**: A large text field containing the system prompt or persona definition that the Daemon injects into the CLI execution environment when running tasks for the agent
- **Runtime_Brief**: The constructed system prompt payload derived from Agent_Instructions that the Daemon passes to the CLI backend via the appropriate mechanism for each provider
- **PAT**: Personal Access Token — a bearer token with configurable expiry used for daemon-to-server and CLI-to-server authentication, prefixed with `af_`
- **Token_Prefix**: The first 12 characters of a PAT, stored for display purposes so users can identify tokens without exposing the full value
- **Nexus**: The default agent created automatically on first user setup, bound to the first detected runtime
- **Agent_Status**: A derived state (idle, working, offline) computed from the agent's bound runtime status and active task count
- **Visibility**: An access control setting on agents — "private" (only the owner can see and use) or "shared" (all users on the instance can see and use)
- **Server**: The Go HTTP backend that manages authentication, agent registry, task queue, and WebSocket connections
- **Daemon**: The background process running on the user's local machine that detects agent CLIs, registers runtimes, polls for tasks, and executes them
- **Web_UI**: The browser-based React SPA interface for managing agents, tokens, and delegating tasks
- **Agent_Runtime**: A detected AI CLI tool registered by the Daemon as available for task execution

## Requirements

### Requirement 1: Personal Access Token Creation

**User Story:** As a user, I want to create Personal Access Tokens from the Web UI with a name and configurable expiry, so that I can authenticate the CLI without going through the browser OAuth flow each time.

#### Acceptance Criteria

1. WHEN the user submits a PAT creation request with a name and expiry option, THE Server SHALL generate a cryptographically random token with the `af_` prefix (3-character prefix followed by 64 random hex characters), store its SHA-256 hash, and return the full token value exactly once in the response
2. THE Server SHALL support the following expiry options for PAT creation: 30 days, 90 days, 365 days, or no expiry (stored as a null expiry timestamp)
3. WHEN the Server creates a PAT, THE Server SHALL store the token prefix (first 12 characters), the token name, creation timestamp, and expiry timestamp
4. IF the user submits a PAT creation request with an empty or whitespace-only name, THEN THE Server SHALL reject the request with a validation error indicating the name is required
5. IF the user submits a PAT creation request with a name exceeding 64 characters, THEN THE Server SHALL reject the request with a validation error indicating the name exceeds the maximum length
6. IF the user submits a PAT creation request with a name identical to an existing non-revoked token name for the same user, THEN THE Server SHALL still create the token (duplicate names are permitted)
7. WHEN the user authenticates via `af login --token <token>`, THE CLI SHALL validate the token against the Server by sending it in the Authorization header, and if the Server responds with a successful authentication, store the token in `~/.agenticflow/config.json`

### Requirement 2: Personal Access Token Listing and Display

**User Story:** As a user, I want to view my existing tokens with their metadata, so that I can track which tokens are active and when they were last used.

#### Acceptance Criteria

1. WHEN the user requests the token list, THE Server SHALL return all non-revoked PATs for the authenticated user, up to a maximum of 100 tokens, with each entry containing: token name, token prefix (first 12 characters), creation date, last-used timestamp (or null if never used), and expiry date, sorted by creation date descending
2. THE Web_UI SHALL display the token list sorted by creation date (newest first) showing: name, prefix displayed as the first 4 visible characters followed by "••••••••", creation date, last-used date (or "Never" if null), and expiry date
3. WHEN a PAT is used to authenticate an API request, THE Server SHALL update the last_used_at timestamp for that token within 60 seconds of the request
4. IF the authenticated user has no non-revoked PATs, THEN THE Server SHALL return an empty list and THE Web_UI SHALL display an empty state message indicating no tokens exist
5. IF a token's expiry date is in the past, THEN THE Web_UI SHALL visually distinguish that token entry from non-expired tokens in the list

### Requirement 3: Personal Access Token Revocation

**User Story:** As a user, I want to revoke tokens that are no longer needed or may be compromised, so that I can maintain security of my account.

#### Acceptance Criteria

1. WHEN the user requests revocation of a PAT by its ID, THE Server SHALL delete the token record from the database and invalidate any cached authentication state for that token within the same request-response cycle, before returning the response to the caller
2. WHEN the user initiates token revocation in the Web_UI, THE Web_UI SHALL display a confirmation dialog that identifies the token name being revoked and requires explicit user confirmation before sending the revocation request to the Server
3. IF the user attempts to revoke a token that does not exist or does not belong to the authenticated user, THEN THE Server SHALL return a successful response (idempotent behavior) indistinguishable from a successful revocation of an existing token
4. IF a revoked PAT is used in an authentication request, THEN THE Server SHALL reject the request with an unauthorized error within 500 milliseconds of the revocation having completed

### Requirement 4: Token Display After Creation

**User Story:** As a user, I want to copy my newly created token immediately after creation, so that I can use it in the CLI before it becomes inaccessible.

#### Acceptance Criteria

1. WHEN a PAT is successfully created, THE Web_UI SHALL display the full unmasked token value in a modal dialog with a copy-to-clipboard button, and the dialog SHALL remain open until the user explicitly closes it via a close or done button
2. WHEN a PAT is successfully created, THE Web_UI SHALL display a warning message within the dialog indicating the token will not be shown again after the dialog is closed
3. WHEN the user clicks the copy button, THE Web_UI SHALL copy the token value to the system clipboard and display a visual confirmation for at least 2 seconds
4. IF the clipboard write operation fails, THEN THE Web_UI SHALL display an error message indicating the copy failed and keep the token value visible so the user can manually select and copy it

### Requirement 5: User Registration

**User Story:** As a new user, I want to create an account from the login page, so that I can start using AgenticFlow without requiring an OAuth provider.

#### Acceptance Criteria

1. THE Web_UI login page SHALL display a toggle or link to switch between the sign-in form and the registration form
2. WHEN the user submits the registration form, THE Server SHALL validate that: the email matches a valid email format (contains exactly one `@` followed by a domain with at least one dot), the email does not exceed 254 characters, the password is between 8 and 128 characters, and the name is between 1 and 128 characters after trimming leading and trailing whitespace
3. WHEN registration validation passes, THE Server SHALL create the user account with the provided email, hashed password, and trimmed name, then return a PAT with a 90-day expiry
4. IF the user submits a registration request with an email that already exists, THEN THE Server SHALL reject the request with an error indicating the email is already registered
5. IF the user submits a registration request with a password shorter than 8 characters, THEN THE Server SHALL reject the request with an error indicating the minimum password length requirement
6. IF the user submits a registration request with an email that does not match the valid email format or exceeds 254 characters, THEN THE Server SHALL reject the request with an error indicating the email is invalid
7. IF the user submits a registration request with a name that is empty or contains only whitespace, THEN THE Server SHALL reject the request with an error indicating the name is required

### Requirement 6: Agent Data Model

**User Story:** As a developer, I want agents to have a rich configuration including instructions, runtime binding, and execution parameters, so that I can define specialized AI agent personas for different tasks.

#### Acceptance Criteria

1. THE Server SHALL store agents with the following fields: id (UUID), name (1-64 characters), description (max 255 characters), instructions (text, max 50,000 characters), runtime_id (reference to a detected runtime), model (optional string, max 100 characters), custom_env (up to 20 key-value pairs where each key is 1-64 characters and each value is 1-1024 characters), custom_args (array of up to 20 strings, each max 256 characters), max_concurrent_tasks (integer 1-20, default 1), visibility (private or shared), avatar_url (optional URL, max 2048 characters), owner user_id, created_at, and updated_at
2. THE Server SHALL validate agent names match the pattern of 1-64 characters containing only alphanumeric characters, hyphens, and underscores, where the name must start with an alphanumeric character
3. IF the user submits an agent with a name that fails validation, THEN THE Server SHALL reject the request with a validation error indicating the name constraint that was violated
4. IF the user submits an agent with a description exceeding 255 characters, THEN THE Server SHALL reject the request with a validation error indicating the description length limit was exceeded
5. IF the user submits an agent with more than 20 custom environment variable pairs or any key or value exceeding its length limit, THEN THE Server SHALL reject the request with a validation error indicating which constraint was violated
6. IF the user submits an agent with max_concurrent_tasks outside the range 1-20, THEN THE Server SHALL reject the request with a validation error indicating the allowed range
7. IF the user submits an agent with a runtime_id that does not reference an existing registered runtime, THEN THE Server SHALL reject the request with a validation error indicating the runtime was not found
8. IF the user submits an agent with a name that already exists for the same owner, THEN THE Server SHALL reject the request with a validation error indicating the name is already in use

### Requirement 7: Default Agent Creation

**User Story:** As a new user, I want a default agent to be created automatically when I first set up AgenticFlow, so that I can start delegating tasks immediately without manual configuration.

#### Acceptance Criteria

1. WHEN a Daemon registers its first Agent_Runtime for a user who does not yet have any agents, THE Server SHALL create a default agent named "Nexus" with description "Your local AI coding agent", bound to the first Agent_Runtime in the registration payload, with max_concurrent_tasks set to 1, visibility set to "private", and instructions set to an empty string
2. IF a default agent named "Nexus" already exists for the user, THEN THE Server SHALL skip creation of the default agent and return success without modification
3. THE default agent "Nexus" SHALL be editable by the user for the following mutable fields: name, description, instructions, model, custom_env, custom_args, max_concurrent_tasks, and visibility
4. IF creation of the default agent fails due to a database error, THEN THE Server SHALL log the error and continue processing the daemon registration without blocking it, leaving the user without a default agent until the next runtime registration event triggers a retry

### Requirement 8: Agent CRUD Operations

**User Story:** As a user, I want to create, read, update, and delete agents through the API and Web UI, so that I can manage my agent configurations.

#### Acceptance Criteria

1. WHEN the user creates an agent via the API, THE Server SHALL validate all fields, store the agent, and return the created agent with its generated ID
2. WHEN the user requests the agent list, THE Server SHALL return all agents visible to the user (owned agents plus shared agents) with their current derived status
3. WHEN the user updates an agent, THE Server SHALL validate the updated fields and persist the changes
4. WHEN the user deletes an agent, THE Server SHALL prevent new task assignments to that agent, allow in-flight tasks to complete, and remove the agent record
5. IF the user attempts to create an agent with a name that already exists within their account, THEN THE Server SHALL reject the request with a conflict error
6. IF the user attempts to bind an agent to a runtime_id that does not exist or is not accessible to the user, THEN THE Server SHALL reject the request with a validation error

### Requirement 9: Agent Status Derivation

**User Story:** As a user, I want to see the current status of each agent at a glance, so that I can know which agents are available for task delegation.

#### Acceptance Criteria

1. IF the agent's bound runtime's daemon is offline, THEN THE Server SHALL report the agent status as "offline", regardless of the agent's task state
2. IF the agent's bound runtime's daemon is online and the agent has at least one task in "running" status, THEN THE Server SHALL report the agent status as "working"
3. IF the agent's bound runtime's daemon is online and the agent has no tasks in "running" status, THEN THE Server SHALL report the agent status as "idle"
4. WHEN a daemon connects or disconnects, THE Server SHALL recompute and broadcast updated agent statuses for all agents bound to that daemon's runtimes via WebSocket within 2 seconds
5. WHEN a task transitions to "running", "completed", "failed", or "cancelled" status, THE Server SHALL recompute and broadcast the owning agent's status via WebSocket within 2 seconds
6. THE Server SHALL evaluate agent status conditions in priority order: "offline" takes precedence over "working", which takes precedence over "idle"

### Requirement 10: Agent List Page

**User Story:** As a user, I want to view all my agents in a list with their status, so that I can quickly assess which agents are available and manage them.

#### Acceptance Criteria

1. THE Web_UI SHALL display the agent list page at route `/agents` showing all agents accessible to the user
2. THE Web_UI SHALL display each agent card with: name, description (truncated), bound runtime name, current status (idle/working/offline) with a color-coded badge, and model (if set)
3. THE Web_UI SHALL provide a button to navigate to the agent creation page
4. WHEN an agent's status changes, THE Web_UI SHALL update the status badge within 2 seconds via WebSocket

### Requirement 11: Agent Creation Page

**User Story:** As a user, I want a form to create new agents with all configuration options, so that I can define specialized agents for different tasks.

#### Acceptance Criteria

1. THE Web_UI SHALL display the agent creation form at route `/agents/new` with fields for: name, description, instructions (large text area), runtime (dropdown of available runtimes), model, custom environment variables (key-value editor), custom arguments (array editor), max concurrent tasks (number input), and visibility (private/shared toggle)
2. WHEN the user submits the creation form with valid data, THE Web_UI SHALL send the creation request to the Server and navigate to the agent list page on success
3. IF the creation form has validation errors, THEN THE Web_UI SHALL display inline error messages for each invalid field and prevent submission
4. THE runtime dropdown SHALL display only runtimes from online daemons belonging to the user

### Requirement 12: Agent Detail and Edit Page

**User Story:** As a user, I want to view and edit an agent's configuration inline, so that I can adjust agent behavior without recreating it.

#### Acceptance Criteria

1. THE Web_UI SHALL display the agent detail page at route `/agents/:id` showing all agent fields with inline editing capability
2. WHEN the user modifies a field and saves, THE Web_UI SHALL send the update request to the Server and display a success confirmation
3. THE agent detail page SHALL display the agent's recent task history (last 10 tasks) with: status, prompt preview, duration, and completion time
4. IF the user does not own the agent and the agent is shared, THEN THE Web_UI SHALL display the agent in read-only mode without edit controls

### Requirement 13: Prompt Routing Through Agent Instructions

**User Story:** As a user, I want my agent's instructions to be automatically injected as a system prompt when tasks are executed, so that the AI CLI behaves according to the agent's defined persona.

#### Acceptance Criteria

1. WHEN the Daemon claims a task, THE Server SHALL include the full agent configuration in the task claim response: agent ID, name, instructions, custom_env, custom_args, and model
2. WHEN the Daemon executes a task for an agent with non-empty instructions, THE Daemon SHALL construct a Runtime_Brief by combining the agent's name, instructions, and available workspace context into a single markdown document, and pass it to the CLI backend
3. WHEN the agent's bound runtime is of type "claude" or "pi", THE Daemon SHALL pass the Runtime_Brief via the `--append-system-prompt` CLI flag as a single string argument
4. WHEN the agent's bound runtime is of type "opencode", THE Daemon SHALL pass the Runtime_Brief via the `--prompt` CLI flag as a single string argument
5. WHEN the agent's bound runtime is of type "openclaw", "kiro", or "kimi", THE Daemon SHALL prepend the Runtime_Brief to the task prompt text separated by a "\n\n---\n\n" delimiter before passing the combined text as the user message
6. WHEN the agent's bound runtime is of type "codex", THE Daemon SHALL pass the Runtime_Brief as the `developerInstructions` field in the Codex session configuration
7. WHEN the agent's bound runtime is of type "hermes", THE Daemon SHALL write the Runtime_Brief to an AGENTS.md file in the task workspace directory and SHALL NOT pass it inline as a system prompt or user message
8. IF the agent's bound runtime type does not match any of "claude", "pi", "opencode", "openclaw", "kiro", "kimi", "codex", or "hermes", THEN THE Daemon SHALL write the Runtime_Brief to an AGENTS.md file in the task workspace directory and skip inline prompt injection
9. IF the agent's instructions field is empty, THEN THE Daemon SHALL skip Runtime_Brief construction and execute the task with only the user prompt

### Requirement 14: Agent Custom Environment and Arguments Injection

**User Story:** As a user, I want my agent's custom environment variables and arguments to be applied when tasks execute, so that I can configure API keys, feature flags, and CLI options per agent.

#### Acceptance Criteria

1. WHEN the Daemon executes a task for an agent with custom_env entries, THE Daemon SHALL set each key-value pair as an environment variable in the spawned CLI process, supporting up to 20 key-value pairs
2. IF a custom_env key matches a blocked key (HOME, PATH, USER, SHELL, TERM, or any daemon-internal prefix such as AF_), THEN THE Daemon SHALL skip that key without setting it in the process environment and log a warning identifying the blocked key
3. WHEN the Daemon executes a task for an agent with custom_args entries, THE Daemon SHALL append the custom arguments to the CLI invocation after any daemon-wide default arguments
4. WHEN the Daemon executes a task for an agent with a model override, THE Daemon SHALL pass the model value to the CLI backend using the provider-appropriate mechanism (--model flag or equivalent), falling back to the daemon-wide provider model if the agent-level model is empty
5. WHEN the Daemon executes a task for an agent that has both custom_env entries and task-level environment variables with overlapping keys, THE Daemon SHALL merge the two sets with task-level values taking precedence over agent-level values for duplicate keys
6. IF the agent has no custom_env entries and no custom_args entries, THEN THE Daemon SHALL execute the CLI process with only the daemon-managed environment variables and daemon-wide default arguments

### Requirement 15: Agent Concurrency Enforcement

**User Story:** As a user, I want to limit how many tasks an agent can run simultaneously, so that I can prevent resource exhaustion on my local machine.

#### Acceptance Criteria

1. WHILE an agent has active tasks equal to its max_concurrent_tasks setting, THE Server SHALL skip that agent when assigning new tasks from the queue, leaving tasks in "pending" status
2. WHEN an agent's active task count drops below max_concurrent_tasks, THE Server SHALL resume assigning pending tasks to that agent on the next poll cycle
3. THE Server SHALL enforce max_concurrent_tasks independently per agent, allowing multiple agents on the same daemon to each run up to their individual limits (subject to the daemon's global max_concurrent_tasks cap)

### Requirement 16: Task-Agent Association

**User Story:** As a user, I want tasks to be associated with the agent that executed them, so that I can track which agent performed which work.

#### Acceptance Criteria

1. WHEN a task is created targeting a specific agent, THE Server SHALL store the agent_id reference on the task record
2. WHEN the user views an agent's detail page, THE Web_UI SHALL display the agent's task history showing tasks executed by that agent
3. WHEN the user views a task detail, THE Web_UI SHALL display the name of the agent that executed the task

### Requirement 17: Agent Table Migration

**User Story:** As a developer, I want the database schema to evolve from the basic custom_agent table to a full agent table, so that the new agent management features are properly persisted.

#### Acceptance Criteria

1. THE Server SHALL provide a database migration that creates the new `agent` table with all fields defined in Requirement 6
2. THE migration SHALL preserve existing custom_agent data by migrating records to the new agent table with: name, command mapped to a custom runtime entry, args_template mapped to custom_args, model_override mapped to model, and env_vars mapped to custom_env
3. THE migration SHALL be reversible, allowing rollback to the previous schema
4. WHEN the Server starts, THE Server SHALL automatically run pending migrations before accepting requests

### Requirement 18: Settings Page with Token Management Tab

**User Story:** As a user, I want a Settings page in the Web UI where I can manage my Personal Access Tokens, so that I have a centralized place for account configuration.

#### Acceptance Criteria

1. THE Web_UI SHALL provide a Settings page accessible from the main navigation
2. THE Settings page SHALL include a "Tokens" tab displaying the PAT management interface (create, list, revoke)
3. THE token creation form SHALL include fields for: token name (text input) and expiry (dropdown with options: 30 days, 90 days, 365 days, Never)
4. THE token list SHALL display each token with: name, prefix (first 12 chars), creation date, last used date, expiry date, and a revoke button

### Requirement 19: Task Delegation via Agent Selection

**User Story:** As a user, I want to select a specific agent when delegating a task, so that the task executes with that agent's configuration and persona.

#### Acceptance Criteria

1. WHEN the user creates a task via the Web_UI, THE Web_UI SHALL present a dropdown of available agents (those with status "idle" or "working" below their concurrency limit) for the user to select
2. WHEN the user submits a task targeting a specific agent, THE Server SHALL resolve the agent's bound runtime and enqueue the task for that runtime
3. IF the selected agent's bound runtime is offline, THEN THE Server SHALL accept the task with "pending" status and assign it when the runtime comes back online
4. THE task creation form SHALL display the selected agent's name, bound runtime type, and model (if set) as confirmation before submission

