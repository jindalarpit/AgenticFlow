package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/realtime"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// notifyMockQuerier implements only the methods needed for notifyDaemon testing.
type notifyMockQuerier struct {
	db.Querier
	agents   map[pgtype.UUID]db.Agent
	runtimes map[pgtype.UUID]db.AgentRuntime
	daemons  map[pgtype.UUID]db.Daemon
}

func (m *notifyMockQuerier) GetAgent(ctx context.Context, id pgtype.UUID) (db.Agent, error) {
	agent, ok := m.agents[id]
	if !ok {
		return db.Agent{}, fmt.Errorf("agent not found")
	}
	return agent, nil
}

func (m *notifyMockQuerier) GetRuntimeByID(ctx context.Context, id pgtype.UUID) (db.AgentRuntime, error) {
	runtime, ok := m.runtimes[id]
	if !ok {
		return db.AgentRuntime{}, fmt.Errorf("runtime not found")
	}
	return runtime, nil
}

func (m *notifyMockQuerier) GetDaemonByID(ctx context.Context, id pgtype.UUID) (db.Daemon, error) {
	daemon, ok := m.daemons[id]
	if !ok {
		return db.Daemon{}, fmt.Errorf("daemon not found")
	}
	return daemon, nil
}

func TestNotifyDaemon_SendsEventWhenConnected(t *testing.T) {
	// Setup: agent -> runtime -> daemon, daemon is connected.
	agentID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	runtimeID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	daemonDBID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	taskID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	daemonIDStr := "daemon-abc-123"

	mock := &notifyMockQuerier{
		agents: map[pgtype.UUID]db.Agent{
			agentID: {ID: agentID, RuntimeID: runtimeID},
		},
		runtimes: map[pgtype.UUID]db.AgentRuntime{
			runtimeID: {ID: runtimeID, DaemonID: daemonDBID},
		},
		daemons: map[pgtype.UUID]db.Daemon{
			daemonDBID: {ID: daemonDBID, DaemonID: daemonIDStr},
		},
	}

	// Create a real Hub and register a daemon client.
	hub := realtime.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Register a daemon client using exported methods.
	daemonClient := &realtime.Client{
		ID:       daemonIDStr,
		IsDaemon: true,
	}
	daemonClient.SetSendChan(make(chan []byte, 10))
	hub.Register(daemonClient)

	// Give the hub time to process the registration.
	time.Sleep(50 * time.Millisecond)

	// Create TaskService and call notifyDaemon.
	svc := NewTaskService(mock, hub)
	task := db.Task{
		ID:      taskID,
		AgentID: agentID,
	}

	svc.notifyDaemon(context.Background(), task)

	// Give the hub time to process the send.
	time.Sleep(50 * time.Millisecond)

	// Read from the daemon client's send channel.
	select {
	case msg := <-daemonClient.SendChan():
		var event realtime.Event
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if event.Type != "task_available" {
			t.Errorf("expected event type 'task_available', got %q", event.Type)
		}
		// Check payload contains task_id.
		payload, ok := event.Payload.(map[string]interface{})
		if !ok {
			t.Fatalf("expected payload to be map[string]interface{}, got %T", event.Payload)
		}
		if payload["task_id"] != uuidToString(taskID) {
			t.Errorf("expected task_id %q, got %q", uuidToString(taskID), payload["task_id"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for daemon message")
	}
}

func TestNotifyDaemon_NoEventWhenDisconnected(t *testing.T) {
	// Setup: agent -> runtime -> daemon, but daemon is NOT connected.
	agentID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	runtimeID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	daemonDBID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	taskID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	daemonIDStr := "daemon-xyz-456"

	mock := &notifyMockQuerier{
		agents: map[pgtype.UUID]db.Agent{
			agentID: {ID: agentID, RuntimeID: runtimeID},
		},
		runtimes: map[pgtype.UUID]db.AgentRuntime{
			runtimeID: {ID: runtimeID, DaemonID: daemonDBID},
		},
		daemons: map[pgtype.UUID]db.Daemon{
			daemonDBID: {ID: daemonDBID, DaemonID: daemonIDStr},
		},
	}

	// Create a real Hub but do NOT register the daemon.
	hub := realtime.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	svc := NewTaskService(mock, hub)
	task := db.Task{
		ID:      taskID,
		AgentID: agentID,
	}

	// This should not panic or send anything.
	svc.notifyDaemon(context.Background(), task)
}

func TestNotifyDaemon_NoEventWhenAgentIDInvalid(t *testing.T) {
	mock := &notifyMockQuerier{
		agents:   make(map[pgtype.UUID]db.Agent),
		runtimes: make(map[pgtype.UUID]db.AgentRuntime),
		daemons:  make(map[pgtype.UUID]db.Daemon),
	}

	hub := realtime.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	svc := NewTaskService(mock, hub)
	task := db.Task{
		ID:      pgtype.UUID{Bytes: [16]byte{4}, Valid: true},
		AgentID: pgtype.UUID{Valid: false}, // invalid agent ID
	}

	// Should return early without error.
	svc.notifyDaemon(context.Background(), task)
}

func TestNotifyDaemon_HandlesAgentNotFound(t *testing.T) {
	// Agent ID is valid but not in the database.
	agentID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	taskID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}

	mock := &notifyMockQuerier{
		agents:   make(map[pgtype.UUID]db.Agent), // empty - agent not found
		runtimes: make(map[pgtype.UUID]db.AgentRuntime),
		daemons:  make(map[pgtype.UUID]db.Daemon),
	}

	hub := realtime.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	svc := NewTaskService(mock, hub)
	task := db.Task{
		ID:      taskID,
		AgentID: agentID,
	}

	// Should log warning but not panic.
	svc.notifyDaemon(context.Background(), task)
}

func TestNotifyDaemon_HandlesRuntimeNotFound(t *testing.T) {
	agentID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	runtimeID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	taskID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}

	mock := &notifyMockQuerier{
		agents: map[pgtype.UUID]db.Agent{
			agentID: {ID: agentID, RuntimeID: runtimeID},
		},
		runtimes: make(map[pgtype.UUID]db.AgentRuntime), // empty - runtime not found
		daemons:  make(map[pgtype.UUID]db.Daemon),
	}

	hub := realtime.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	svc := NewTaskService(mock, hub)
	task := db.Task{
		ID:      taskID,
		AgentID: agentID,
	}

	// Should log warning but not panic.
	svc.notifyDaemon(context.Background(), task)
}

func TestNotifyDaemon_HandlesDaemonNotFound(t *testing.T) {
	agentID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	runtimeID := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	daemonDBID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	taskID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}

	mock := &notifyMockQuerier{
		agents: map[pgtype.UUID]db.Agent{
			agentID: {ID: agentID, RuntimeID: runtimeID},
		},
		runtimes: map[pgtype.UUID]db.AgentRuntime{
			runtimeID: {ID: runtimeID, DaemonID: daemonDBID},
		},
		daemons: make(map[pgtype.UUID]db.Daemon), // empty - daemon not found
	}

	hub := realtime.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	svc := NewTaskService(mock, hub)
	task := db.Task{
		ID:      taskID,
		AgentID: agentID,
	}

	// Should log warning but not panic.
	svc.notifyDaemon(context.Background(), task)
}
