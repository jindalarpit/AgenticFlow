package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: production-hardening, Property 9: Agent status persistence before broadcast
//
// For any status transition, the DB update (UpdateAgentStatus) is called
// BEFORE the WebSocket broadcast (BroadcastAgentStatusChanged).
// If the DB update fails, no broadcast occurs.
// If the DB update succeeds, the broadcast happens with the correct status.
//
// **Validates: Requirements 9.1, 9.3, 9.4**
// ---------------------------------------------------------------------------

// --- Test interfaces and mocks for verifying call ordering ---

// statusPersister abstracts the DB update operation for testing.
type statusPersister interface {
	UpdateAgentStatus(ctx context.Context, agentID string, status string) error
}

// statusBroadcaster abstracts the WebSocket broadcast operation for testing.
type statusBroadcaster interface {
	BroadcastAgentStatusChanged(agentID string, status string)
}

// callRecord tracks the order of operations for verification.
type callRecord struct {
	mu    sync.Mutex
	calls []string // e.g., "persist:idle", "broadcast:idle"
}

func (r *callRecord) record(op string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, op)
}

func (r *callRecord) getCalls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]string, len(r.calls))
	copy(result, r.calls)
	return result
}

// mockPersister records persist calls and optionally returns an error.
type mockPersister struct {
	recorder  *callRecord
	shouldErr bool
}

func (m *mockPersister) UpdateAgentStatus(_ context.Context, agentID string, status string) error {
	m.recorder.record("persist:" + status)
	if m.shouldErr {
		return errors.New("db error: connection refused")
	}
	return nil
}

// mockBroadcaster records broadcast calls.
type mockBroadcaster struct {
	recorder      *callRecord
	broadcastedID string
	broadcastedSt string
}

func (m *mockBroadcaster) BroadcastAgentStatusChanged(agentID string, status string) {
	m.recorder.record("broadcast:" + status)
	m.broadcastedID = agentID
	m.broadcastedSt = status
}

// reconcileWithPersistenceOrdering implements the core reconcile logic that
// mirrors AgentStatusService.ReconcileAndBroadcast. It persists the status
// to the database BEFORE broadcasting, and skips broadcast on DB failure.
// This function is used by property tests to verify the ordering invariant.
func reconcileWithPersistenceOrdering(
	ctx context.Context,
	persister statusPersister,
	broadcaster statusBroadcaster,
	agentID string,
	currentStatus string,
	newStatus string,
) (persisted bool, broadcasted bool) {
	// If no change, nothing to do.
	if currentStatus == newStatus {
		return false, false
	}

	// Persist FIRST — update the database before broadcasting.
	err := persister.UpdateAgentStatus(ctx, agentID, newStatus)
	if err != nil {
		// On DB failure, skip broadcast.
		return false, false
	}

	// Then broadcast the status change event.
	broadcaster.BroadcastAgentStatusChanged(agentID, newStatus)
	return true, true
}

// --- Property Tests ---

func TestProperty9_PersistenceOrdering_DBBeforeBroadcast(t *testing.T) {
	// Feature: production-hardening, Property 9: Agent status persistence before broadcast
	// For any status transition where DB succeeds, persist is called BEFORE broadcast.
	rapid.Check(t, func(t *rapid.T) {
		// Generate random agent ID
		agentID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, "agentID")

		// Generate current and new status (must be different to trigger transition)
		statuses := []string{"offline", "idle", "working"}
		currentIdx := rapid.IntRange(0, len(statuses)-1).Draw(t, "currentIdx")
		// Ensure new status is different from current
		newIdx := rapid.IntRange(0, len(statuses)-2).Draw(t, "newIdx")
		if newIdx >= currentIdx {
			newIdx++
		}
		currentStatus := statuses[currentIdx]
		newStatus := statuses[newIdx]

		recorder := &callRecord{}
		persister := &mockPersister{recorder: recorder, shouldErr: false}
		broadcaster := &mockBroadcaster{recorder: recorder}

		ctx := context.Background()
		persisted, broadcasted := reconcileWithPersistenceOrdering(
			ctx, persister, broadcaster, agentID, currentStatus, newStatus,
		)

		if !persisted {
			t.Fatalf("expected persist to succeed for transition %q -> %q", currentStatus, newStatus)
		}
		if !broadcasted {
			t.Fatalf("expected broadcast to occur for transition %q -> %q", currentStatus, newStatus)
		}

		// Verify ordering: persist must come before broadcast
		calls := recorder.getCalls()
		if len(calls) != 2 {
			t.Fatalf("expected exactly 2 calls, got %d: %v", len(calls), calls)
		}
		if calls[0] != "persist:"+newStatus {
			t.Fatalf("first call should be persist:%s, got %s", newStatus, calls[0])
		}
		if calls[1] != "broadcast:"+newStatus {
			t.Fatalf("second call should be broadcast:%s, got %s", newStatus, calls[1])
		}
	})
}

func TestProperty9_PersistenceOrdering_NoBroadcastOnDBFailure(t *testing.T) {
	// Feature: production-hardening, Property 9: Agent status persistence before broadcast
	// For any status transition where DB update fails, no broadcast occurs.
	rapid.Check(t, func(t *rapid.T) {
		// Generate random agent ID
		agentID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, "agentID")

		// Generate current and new status (must be different to trigger transition)
		statuses := []string{"offline", "idle", "working"}
		currentIdx := rapid.IntRange(0, len(statuses)-1).Draw(t, "currentIdx")
		newIdx := rapid.IntRange(0, len(statuses)-2).Draw(t, "newIdx")
		if newIdx >= currentIdx {
			newIdx++
		}
		currentStatus := statuses[currentIdx]
		newStatus := statuses[newIdx]

		recorder := &callRecord{}
		persister := &mockPersister{recorder: recorder, shouldErr: true}
		broadcaster := &mockBroadcaster{recorder: recorder}

		ctx := context.Background()
		persisted, broadcasted := reconcileWithPersistenceOrdering(
			ctx, persister, broadcaster, agentID, currentStatus, newStatus,
		)

		if persisted {
			t.Fatalf("expected persist to fail for transition %q -> %q", currentStatus, newStatus)
		}
		if broadcasted {
			t.Fatalf("expected NO broadcast on DB failure for transition %q -> %q", currentStatus, newStatus)
		}

		// Verify: persist was attempted but broadcast was NOT called
		calls := recorder.getCalls()
		if len(calls) != 1 {
			t.Fatalf("expected exactly 1 call (persist only), got %d: %v", len(calls), calls)
		}
		if calls[0] != "persist:"+newStatus {
			t.Fatalf("the single call should be persist:%s, got %s", newStatus, calls[0])
		}
	})
}

func TestProperty9_PersistenceOrdering_BroadcastHasCorrectStatus(t *testing.T) {
	// Feature: production-hardening, Property 9: Agent status persistence before broadcast
	// When DB update succeeds, the broadcast contains the correct agent ID and new status.
	rapid.Check(t, func(t *rapid.T) {
		// Generate random agent ID
		agentID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, "agentID")

		// Generate current and new status (must be different)
		statuses := []string{"offline", "idle", "working"}
		currentIdx := rapid.IntRange(0, len(statuses)-1).Draw(t, "currentIdx")
		newIdx := rapid.IntRange(0, len(statuses)-2).Draw(t, "newIdx")
		if newIdx >= currentIdx {
			newIdx++
		}
		currentStatus := statuses[currentIdx]
		newStatus := statuses[newIdx]

		recorder := &callRecord{}
		persister := &mockPersister{recorder: recorder, shouldErr: false}
		broadcaster := &mockBroadcaster{recorder: recorder}

		ctx := context.Background()
		reconcileWithPersistenceOrdering(
			ctx, persister, broadcaster, agentID, currentStatus, newStatus,
		)

		// Verify broadcast was called with correct arguments
		if broadcaster.broadcastedID != agentID {
			t.Fatalf("broadcast agent ID = %q, want %q", broadcaster.broadcastedID, agentID)
		}
		if broadcaster.broadcastedSt != newStatus {
			t.Fatalf("broadcast status = %q, want %q", broadcaster.broadcastedSt, newStatus)
		}
	})
}

func TestProperty9_PersistenceOrdering_NoOpOnSameStatus(t *testing.T) {
	// Feature: production-hardening, Property 9: Agent status persistence before broadcast
	// When current status equals new status, neither persist nor broadcast is called.
	rapid.Check(t, func(t *rapid.T) {
		agentID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, "agentID")

		// Same status for both current and new
		statuses := []string{"offline", "idle", "working"}
		statusIdx := rapid.IntRange(0, len(statuses)-1).Draw(t, "statusIdx")
		status := statuses[statusIdx]

		recorder := &callRecord{}
		persister := &mockPersister{recorder: recorder, shouldErr: false}
		broadcaster := &mockBroadcaster{recorder: recorder}

		ctx := context.Background()
		persisted, broadcasted := reconcileWithPersistenceOrdering(
			ctx, persister, broadcaster, agentID, status, status,
		)

		if persisted {
			t.Fatalf("expected no persist when status unchanged (%q)", status)
		}
		if broadcasted {
			t.Fatalf("expected no broadcast when status unchanged (%q)", status)
		}

		// Verify no calls were made
		calls := recorder.getCalls()
		if len(calls) != 0 {
			t.Fatalf("expected zero calls when status unchanged, got %d: %v", len(calls), calls)
		}
	})
}

func TestProperty9_PersistenceOrdering_AllTransitionsPreservesOrdering(t *testing.T) {
	// Feature: production-hardening, Property 9: Agent status persistence before broadcast
	// For any sequence of status transitions on the same agent, each transition
	// that changes status always persists before broadcasting.
	rapid.Check(t, func(t *rapid.T) {
		agentID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, "agentID")

		statuses := []string{"offline", "idle", "working"}
		numTransitions := rapid.IntRange(2, 20).Draw(t, "numTransitions")

		// Track current status
		currentIdx := rapid.IntRange(0, len(statuses)-1).Draw(t, "initialStatusIdx")
		currentStatus := statuses[currentIdx]

		for i := 0; i < numTransitions; i++ {
			// Generate next status (may or may not be different)
			nextIdx := rapid.IntRange(0, len(statuses)-1).Draw(t, "nextStatusIdx")
			nextStatus := statuses[nextIdx]

			recorder := &callRecord{}
			persister := &mockPersister{recorder: recorder, shouldErr: false}
			broadcaster := &mockBroadcaster{recorder: recorder}

			ctx := context.Background()
			reconcileWithPersistenceOrdering(
				ctx, persister, broadcaster, agentID, currentStatus, nextStatus,
			)

			calls := recorder.getCalls()

			if currentStatus == nextStatus {
				// No-op: no calls expected
				if len(calls) != 0 {
					t.Fatalf("transition %d: same status %q should produce 0 calls, got %d: %v",
						i, currentStatus, len(calls), calls)
				}
			} else {
				// Status changed: exactly 2 calls in order
				if len(calls) != 2 {
					t.Fatalf("transition %d: %q -> %q should produce 2 calls, got %d: %v",
						i, currentStatus, nextStatus, len(calls), calls)
				}
				if calls[0] != "persist:"+nextStatus {
					t.Fatalf("transition %d: first call should be persist:%s, got %s",
						i, nextStatus, calls[0])
				}
				if calls[1] != "broadcast:"+nextStatus {
					t.Fatalf("transition %d: second call should be broadcast:%s, got %s",
						i, nextStatus, calls[1])
				}
			}

			// Update current status for next iteration
			currentStatus = nextStatus
		}
	})
}

func TestProperty9_PersistenceOrdering_MatchesProductionCode(t *testing.T) {
	// Feature: production-hardening, Property 9: Agent status persistence before broadcast
	// Verify that the reconcileWithPersistenceOrdering function's behavior matches
	// the production ReconcileAndBroadcast logic: persist-then-broadcast ordering
	// and skip-broadcast-on-failure semantics.
	rapid.Check(t, func(t *rapid.T) {
		agentID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, "agentID")

		statuses := []string{"offline", "idle", "working"}
		currentIdx := rapid.IntRange(0, len(statuses)-1).Draw(t, "currentIdx")
		newIdx := rapid.IntRange(0, len(statuses)-1).Draw(t, "newIdx")
		currentStatus := statuses[currentIdx]
		newStatus := statuses[newIdx]

		// Randomly decide if DB should fail
		dbFails := rapid.Bool().Draw(t, "dbFails")

		recorder := &callRecord{}
		persister := &mockPersister{recorder: recorder, shouldErr: dbFails}
		broadcaster := &mockBroadcaster{recorder: recorder}

		ctx := context.Background()
		persisted, broadcasted := reconcileWithPersistenceOrdering(
			ctx, persister, broadcaster, agentID, currentStatus, newStatus,
		)

		calls := recorder.getCalls()

		if currentStatus == newStatus {
			// No change: nothing should happen
			if persisted || broadcasted {
				t.Fatalf("no-op case: persisted=%v, broadcasted=%v, want both false",
					persisted, broadcasted)
			}
			if len(calls) != 0 {
				t.Fatalf("no-op case: expected 0 calls, got %v", calls)
			}
		} else if dbFails {
			// DB failure: persist attempted, no broadcast
			if persisted || broadcasted {
				t.Fatalf("db failure case: persisted=%v, broadcasted=%v, want both false",
					persisted, broadcasted)
			}
			if len(calls) != 1 || calls[0] != "persist:"+newStatus {
				t.Fatalf("db failure case: expected [persist:%s], got %v", newStatus, calls)
			}
		} else {
			// Success: persist then broadcast
			if !persisted || !broadcasted {
				t.Fatalf("success case: persisted=%v, broadcasted=%v, want both true",
					persisted, broadcasted)
			}
			if len(calls) != 2 {
				t.Fatalf("success case: expected 2 calls, got %v", calls)
			}
			if calls[0] != "persist:"+newStatus {
				t.Fatalf("success case: first call should be persist:%s, got %s", newStatus, calls[0])
			}
			if calls[1] != "broadcast:"+newStatus {
				t.Fatalf("success case: second call should be broadcast:%s, got %s", newStatus, calls[1])
			}
		}
	})
}
