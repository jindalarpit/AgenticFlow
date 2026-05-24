/**
 * Type definitions for the Agent Detail UI.
 *
 * These types support the two-column agent detail page with sidebar inspector
 * and tabbed content area (Activity, Tasks, Instructions, Skills, Environment, Custom Args).
 */

/* ─── Status Literals ─── */

export type AgentStatus = "idle" | "working" | "offline";

export type AgentVisibility = "private" | "shared";

export type TaskStatus =
  | "pending"
  | "dispatched"
  | "running"
  | "completed"
  | "failed"
  | "cancelled"
  | "timeout";

/* ─── Core Interfaces ─── */

/** Skill attached to an agent. */
export interface AgentSkill {
  id: string;
  name: string;
}

/**
 * Extended Agent interface for the detail page.
 * Includes owner_name and skills beyond the base ManagedAgent type.
 */
export interface Agent {
  id: string;
  name: string;
  description: string;
  instructions: string;
  avatar_url: string | null;
  runtime_id: string;
  runtime_name?: string;
  custom_env: Record<string, string>;
  custom_args: string[];
  model: string;
  visibility: AgentVisibility;
  status: AgentStatus;
  max_concurrent_tasks: number;
  owner_id: string;
  owner_name?: string;
  skills: AgentSkill[];
  created_at: string;
  updated_at: string;
}

/** Pre-computed 30-day statistics for an agent. */
export interface AgentStats {
  total_runs: number;
  success_rate: number;
  avg_duration_ms: number;
  total_terminal: number;
}

/** A task assigned to an agent. */
export interface AgentTask {
  id: string;
  status: TaskStatus;
  prompt: string;
  failure_reason?: string;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
  duration_ms?: number;
}

/** Paginated response for agent tasks. */
export interface PaginatedTasks {
  tasks: AgentTask[];
  total: number;
}

/* ─── WebSocket Event Payloads ─── */

/** Payload for task-related WebSocket events (task_created, task_completed, task_failed). */
export interface TaskEvent {
  task_id: string;
  agent_id: string;
  status: string;
  prompt?: string;
  failure_reason?: string;
}

/** Payload for daemon-related WebSocket events (daemon_connected, daemon_disconnected). */
export interface DaemonEvent {
  daemon_id: string;
  runtime_ids: string[];
}

/* ─── Tab State ─── */

/** Identifiers for the six tabs in the Overview Pane. */
export type TabId = "activity" | "tasks" | "instructions" | "skills" | "env" | "custom_args";

/** State for tab navigation and dirty-guard logic. */
export interface TabState {
  activeTab: TabId;
  activeDirty: boolean;
  pendingTab: TabId | null;
}

/* ─── Component Props ─── */

/**
 * Props shared by editable tabs (Instructions, Environment, Custom Args).
 * Each tab reports dirty state upward and receives a save handler.
 */
export interface EditableTabProps {
  agent: Agent;
  isOwner: boolean;
  onDirtyChange: (dirty: boolean) => void;
  onSave: (data: Partial<Agent>) => Promise<void>;
}
