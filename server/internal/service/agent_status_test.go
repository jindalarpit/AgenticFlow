package service

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// pgTypeUUID is a test helper to create a pgtype.UUID.
func pgTypeUUID(bytes [16]byte, valid bool) pgtype.UUID {
	return pgtype.UUID{Bytes: bytes, Valid: valid}
}

func TestDeriveAgentStatus_OfflineRuntime(t *testing.T) {
	// When runtime is offline, status should always be offline regardless of task count.
	tests := []struct {
		name            string
		activeTaskCount int
	}{
		{"no tasks", 0},
		{"one task", 1},
		{"many tasks", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveAgentStatus("offline", tt.activeTaskCount)
			if got != AgentStatusOffline {
				t.Errorf("DeriveAgentStatus(\"offline\", %d) = %q, want %q", tt.activeTaskCount, got, AgentStatusOffline)
			}
		})
	}
}

func TestDeriveAgentStatus_OnlineWithActiveTasks(t *testing.T) {
	// When runtime is online and there are active tasks, status should be working.
	tests := []struct {
		name            string
		activeTaskCount int
	}{
		{"one task", 1},
		{"two tasks", 2},
		{"max tasks", 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveAgentStatus("online", tt.activeTaskCount)
			if got != AgentStatusWorking {
				t.Errorf("DeriveAgentStatus(\"online\", %d) = %q, want %q", tt.activeTaskCount, got, AgentStatusWorking)
			}
		})
	}
}

func TestDeriveAgentStatus_OnlineNoTasks(t *testing.T) {
	// When runtime is online and there are no active tasks, status should be idle.
	got := DeriveAgentStatus("online", 0)
	if got != AgentStatusIdle {
		t.Errorf("DeriveAgentStatus(\"online\", 0) = %q, want %q", got, AgentStatusIdle)
	}
}

func TestDeriveAgentStatus_PriorityOrder(t *testing.T) {
	// Verify priority: offline > working > idle.
	// Even with active tasks, offline runtime means offline status.
	got := DeriveAgentStatus("offline", 5)
	if got != AgentStatusOffline {
		t.Errorf("priority violation: offline runtime with active tasks should be offline, got %q", got)
	}

	// Online with tasks = working.
	got = DeriveAgentStatus("online", 1)
	if got != AgentStatusWorking {
		t.Errorf("priority violation: online runtime with active tasks should be working, got %q", got)
	}

	// Online with no tasks = idle.
	got = DeriveAgentStatus("online", 0)
	if got != AgentStatusIdle {
		t.Errorf("priority violation: online runtime with no tasks should be idle, got %q", got)
	}
}

func TestDeriveAgentStatus_UnknownRuntimeStatus(t *testing.T) {
	// Any non-"offline" runtime status should be treated as online.
	tests := []struct {
		name          string
		runtimeStatus string
		taskCount     int
		want          AgentStatus
	}{
		{"empty status no tasks", "", 0, AgentStatusIdle},
		{"empty status with tasks", "", 3, AgentStatusWorking},
		{"unknown status no tasks", "unknown", 0, AgentStatusIdle},
		{"unknown status with tasks", "unknown", 1, AgentStatusWorking},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveAgentStatus(tt.runtimeStatus, tt.taskCount)
			if got != tt.want {
				t.Errorf("DeriveAgentStatus(%q, %d) = %q, want %q", tt.runtimeStatus, tt.taskCount, got, tt.want)
			}
		})
	}
}

func TestUuidToString(t *testing.T) {
	tests := []struct {
		name string
		id   [16]byte
		want string
	}{
		{
			name: "zero uuid",
			id:   [16]byte{},
			want: "00000000-0000-0000-0000-000000000000",
		},
		{
			name: "sample uuid",
			id:   [16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0},
			want: "12345678-9abc-def0-1234-56789abcdef0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := pgTypeUUID(tt.id, true)
			got := uuidToString(id)
			if got != tt.want {
				t.Errorf("uuidToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUuidToString_Invalid(t *testing.T) {
	id := pgTypeUUID([16]byte{}, false)
	got := uuidToString(id)
	if got != "" {
		t.Errorf("uuidToString(invalid) = %q, want empty string", got)
	}
}
