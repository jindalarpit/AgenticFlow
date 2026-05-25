/**
 * Session storage persistence for the Task Result Panel.
 *
 * Stores the panel's state (which task is displayed, whether it was dismissed)
 * in sessionStorage so it survives in-tab navigation but resets on tab close.
 *
 * All functions degrade gracefully when sessionStorage is unavailable
 * (e.g., sandboxed iframes, private browsing restrictions).
 */

/* ─── Types ─── */

export interface TaskResultPanelSessionData {
  taskId: string;
  dismissed: boolean;
}

/* ─── Constants ─── */

const STORAGE_KEY = "af_task_result_panel";

/* ─── Helpers ─── */

function getSessionStorage(): Storage | null {
  if (typeof window === "undefined") return null;
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

/* ─── Public API ─── */

/**
 * Persist the current Task Result Panel state to sessionStorage.
 * Silently no-ops if storage is unavailable or quota is exceeded.
 */
export function saveTaskResultPanelState(state: TaskResultPanelSessionData): void {
  const storage = getSessionStorage();
  if (!storage) return;
  try {
    storage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // Quota exceeded or storage disabled — silently skip.
  }
}

/**
 * Load the Task Result Panel state from sessionStorage.
 * Returns null if the key is missing, the data is malformed, or storage is unavailable.
 */
export function loadTaskResultPanelState(): TaskResultPanelSessionData | null {
  const storage = getSessionStorage();
  if (!storage) return null;
  try {
    const raw = storage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (
      parsed &&
      typeof parsed === "object" &&
      typeof parsed.taskId === "string" &&
      typeof parsed.dismissed === "boolean"
    ) {
      return parsed as TaskResultPanelSessionData;
    }
    return null;
  } catch {
    // Malformed JSON or storage error — treat as empty.
    return null;
  }
}

/**
 * Remove the Task Result Panel state from sessionStorage.
 * Silently no-ops if storage is unavailable.
 */
export function clearTaskResultPanelState(): void {
  const storage = getSessionStorage();
  if (!storage) return;
  try {
    storage.removeItem(STORAGE_KEY);
  } catch {
    // Storage error — silently skip.
  }
}
