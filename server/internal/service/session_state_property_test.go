package service

import (
	"testing"

	"github.com/agenticflow/agenticflow/internal/realtime"
	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: interactive-task-sessions, Property 6: Session State Machine Validity
//
// For any sequence of events (output received, input_requested, input_cleared,
// task terminal) applied to a task's session state, the resulting state SHALL
// always be one of: "idle", "producing_output", or "waiting_for_input".
// Specifically:
//   (a) input_requested → "waiting_for_input"
//   (b) output received or input_cleared → "producing_output"
//   (c) task terminal → state is removed (GetState returns "idle")
//
// **Validates: Requirements 7.1, 7.2, 7.3, 7.4**
// ---------------------------------------------------------------------------

// validSessionStates is the set of all valid session states.
var validSessionStates = map[SessionState]bool{
	SessionStateIdle:            true,
	SessionStateProducingOutput: true,
	SessionStateWaitingForInput: true,
}

func newTestSessionStateManager() *SessionStateManager {
	hub := realtime.NewHub()
	return NewSessionStateManager(hub)
}

func TestProperty6_SessionStateMachine_AlwaysValidState(t *testing.T) {
	// Feature: interactive-task-sessions, Property 6: Session State Machine Validity
	// For any random sequence of operations (SetState with random valid states,
	// ClearState), GetState always returns one of the three valid states.
	rapid.Check(t, func(t *rapid.T) {
		mgr := newTestSessionStateManager()
		taskID := rapid.StringMatching(`[a-f0-9-]{36}`).Draw(t, "taskID")

		// Generate a random sequence of events
		numEvents := rapid.IntRange(1, 50).Draw(t, "numEvents")

		for i := 0; i < numEvents; i++ {
			// Choose a random event type
			eventType := rapid.IntRange(0, 3).Draw(t, "eventType")

			switch eventType {
			case 0: // input_requested → waiting_for_input
				mgr.SetState(taskID, SessionStateWaitingForInput)
			case 1: // output received → producing_output
				mgr.SetState(taskID, SessionStateProducingOutput)
			case 2: // input_cleared → producing_output
				mgr.SetState(taskID, SessionStateProducingOutput)
			case 3: // task terminal → clear state
				mgr.ClearState(taskID)
			}

			// After every event, verify the state is valid
			state := mgr.GetState(taskID)
			if !validSessionStates[state] {
				t.Fatalf("after event %d (type=%d): GetState(%q) = %q, not a valid session state",
					i, eventType, taskID, state)
			}
		}
	})
}

func TestProperty6_SessionStateMachine_InputRequestedTransition(t *testing.T) {
	// Feature: interactive-task-sessions, Property 6: Session State Machine Validity
	// For any task, SetState(waiting_for_input) always results in "waiting_for_input"
	rapid.Check(t, func(t *rapid.T) {
		mgr := newTestSessionStateManager()
		taskID := rapid.StringMatching(`[a-f0-9-]{36}`).Draw(t, "taskID")

		// Apply some random prior state
		priorEvent := rapid.IntRange(0, 3).Draw(t, "priorEvent")
		switch priorEvent {
		case 0:
			mgr.SetState(taskID, SessionStateIdle)
		case 1:
			mgr.SetState(taskID, SessionStateProducingOutput)
		case 2:
			mgr.SetState(taskID, SessionStateWaitingForInput)
		case 3:
			mgr.ClearState(taskID)
		}

		// Apply input_requested
		mgr.SetState(taskID, SessionStateWaitingForInput)

		state := mgr.GetState(taskID)
		if state != SessionStateWaitingForInput {
			t.Fatalf("after SetState(waiting_for_input): GetState(%q) = %q, want %q",
				taskID, state, SessionStateWaitingForInput)
		}
	})
}

func TestProperty6_SessionStateMachine_OutputReceivedTransition(t *testing.T) {
	// Feature: interactive-task-sessions, Property 6: Session State Machine Validity
	// For any task, SetState(producing_output) always results in "producing_output"
	rapid.Check(t, func(t *rapid.T) {
		mgr := newTestSessionStateManager()
		taskID := rapid.StringMatching(`[a-f0-9-]{36}`).Draw(t, "taskID")

		// Apply some random prior state
		priorEvent := rapid.IntRange(0, 3).Draw(t, "priorEvent")
		switch priorEvent {
		case 0:
			mgr.SetState(taskID, SessionStateIdle)
		case 1:
			mgr.SetState(taskID, SessionStateProducingOutput)
		case 2:
			mgr.SetState(taskID, SessionStateWaitingForInput)
		case 3:
			mgr.ClearState(taskID)
		}

		// Apply output received / input_cleared
		mgr.SetState(taskID, SessionStateProducingOutput)

		state := mgr.GetState(taskID)
		if state != SessionStateProducingOutput {
			t.Fatalf("after SetState(producing_output): GetState(%q) = %q, want %q",
				taskID, state, SessionStateProducingOutput)
		}
	})
}

func TestProperty6_SessionStateMachine_TerminalClearsState(t *testing.T) {
	// Feature: interactive-task-sessions, Property 6: Session State Machine Validity
	// For any task in any state, ClearState results in GetState returning "idle"
	rapid.Check(t, func(t *rapid.T) {
		mgr := newTestSessionStateManager()
		taskID := rapid.StringMatching(`[a-f0-9-]{36}`).Draw(t, "taskID")

		// Set a random state first
		states := []SessionState{SessionStateIdle, SessionStateProducingOutput, SessionStateWaitingForInput}
		stateIdx := rapid.IntRange(0, len(states)-1).Draw(t, "stateIdx")
		mgr.SetState(taskID, states[stateIdx])

		// Clear state (task terminal)
		mgr.ClearState(taskID)

		state := mgr.GetState(taskID)
		if state != SessionStateIdle {
			t.Fatalf("after ClearState: GetState(%q) = %q, want %q (idle is default for cleared state)",
				taskID, state, SessionStateIdle)
		}
	})
}

func TestProperty6_SessionStateMachine_MultipleTasksIndependent(t *testing.T) {
	// Feature: interactive-task-sessions, Property 6: Session State Machine Validity
	// Operations on one task do not affect another task's state
	rapid.Check(t, func(t *rapid.T) {
		mgr := newTestSessionStateManager()
		taskA := rapid.StringMatching(`task-a-[a-f0-9]{8}`).Draw(t, "taskA")
		taskB := rapid.StringMatching(`task-b-[a-f0-9]{8}`).Draw(t, "taskB")

		// Set task A to a specific state
		statesA := []SessionState{SessionStateIdle, SessionStateProducingOutput, SessionStateWaitingForInput}
		stateIdxA := rapid.IntRange(0, len(statesA)-1).Draw(t, "stateIdxA")
		mgr.SetState(taskA, statesA[stateIdxA])

		// Set task B to a different state
		statesB := []SessionState{SessionStateIdle, SessionStateProducingOutput, SessionStateWaitingForInput}
		stateIdxB := rapid.IntRange(0, len(statesB)-1).Draw(t, "stateIdxB")
		mgr.SetState(taskB, statesB[stateIdxB])

		// Modify task A
		mgr.ClearState(taskA)

		// Task B should be unaffected
		stateB := mgr.GetState(taskB)
		if stateB != statesB[stateIdxB] {
			t.Fatalf("modifying task A affected task B: GetState(%q) = %q, want %q",
				taskB, stateB, statesB[stateIdxB])
		}

		// Task A should be idle (cleared)
		stateA := mgr.GetState(taskA)
		if stateA != SessionStateIdle {
			t.Fatalf("after ClearState: GetState(%q) = %q, want %q",
				taskA, stateA, SessionStateIdle)
		}
	})
}

func TestProperty6_SessionStateMachine_DefaultStateIsIdle(t *testing.T) {
	// Feature: interactive-task-sessions, Property 6: Session State Machine Validity
	// For any task that has never been set, GetState returns "idle"
	rapid.Check(t, func(t *rapid.T) {
		mgr := newTestSessionStateManager()
		taskID := rapid.StringMatching(`[a-f0-9-]{36}`).Draw(t, "taskID")

		state := mgr.GetState(taskID)
		if state != SessionStateIdle {
			t.Fatalf("GetState for unknown task %q = %q, want %q",
				taskID, state, SessionStateIdle)
		}
	})
}

func TestProperty6_SessionStateMachine_SetStateOnlyAcceptsValidStates(t *testing.T) {
	// Feature: interactive-task-sessions, Property 6: Session State Machine Validity
	// Even if SetState is called with one of the three valid states, GetState
	// always returns a valid state. This verifies the type system enforcement.
	rapid.Check(t, func(t *rapid.T) {
		mgr := newTestSessionStateManager()
		taskID := rapid.StringMatching(`[a-f0-9-]{36}`).Draw(t, "taskID")

		// Only set valid states (as the API is typed)
		validStates := []SessionState{SessionStateIdle, SessionStateProducingOutput, SessionStateWaitingForInput}
		stateIdx := rapid.IntRange(0, len(validStates)-1).Draw(t, "stateIdx")
		mgr.SetState(taskID, validStates[stateIdx])

		got := mgr.GetState(taskID)
		if !validSessionStates[got] {
			t.Fatalf("SetState(%q, %q) then GetState returned invalid state %q",
				taskID, validStates[stateIdx], got)
		}
		if got != validStates[stateIdx] {
			t.Fatalf("SetState(%q, %q) then GetState = %q, want %q",
				taskID, validStates[stateIdx], got, validStates[stateIdx])
		}
	})
}
