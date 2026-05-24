package service

import (
	"sync"

	"github.com/agenticflow/agenticflow/internal/realtime"
)

// SessionState represents the interaction state of a running task.
type SessionState string

const (
	// SessionStateIdle indicates the task has started but no output yet.
	SessionStateIdle SessionState = "idle"
	// SessionStateProducingOutput indicates the task is actively producing output.
	SessionStateProducingOutput SessionState = "producing_output"
	// SessionStateWaitingForInput indicates the task is waiting for user input.
	SessionStateWaitingForInput SessionState = "waiting_for_input"
)

// SessionStateManager tracks the interaction state of running tasks.
// State is ephemeral (in-memory only) and cleared on task completion.
type SessionStateManager struct {
	mu     sync.RWMutex
	states map[string]SessionState // task_id -> state
	hub    *realtime.Hub
}

// NewSessionStateManager creates a new SessionStateManager.
func NewSessionStateManager(hub *realtime.Hub) *SessionStateManager {
	return &SessionStateManager{
		states: make(map[string]SessionState),
		hub:    hub,
	}
}

// SetState updates the session state for a task and broadcasts the change
// via WebSocket if the state actually changed.
func (m *SessionStateManager) SetState(taskID string, state SessionState) {
	m.mu.Lock()
	prev := m.states[taskID]
	m.states[taskID] = state
	m.mu.Unlock()

	if prev != state {
		m.hub.Broadcast(realtime.Event{
			Type: "session_state_changed",
			Payload: map[string]interface{}{
				"task_id": taskID,
				"state":   string(state),
			},
		})
	}
}

// GetState returns the current session state for a task.
// Returns SessionStateIdle if no state has been set for the task.
func (m *SessionStateManager) GetState(taskID string) SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.states[taskID]
	if !ok {
		return SessionStateIdle
	}
	return state
}

// ClearState removes the session state for a task.
// This should be called when a task transitions to a terminal status
// (completed, failed, cancelled, or timeout).
func (m *SessionStateManager) ClearState(taskID string) {
	m.mu.Lock()
	delete(m.states, taskID)
	m.mu.Unlock()
}
