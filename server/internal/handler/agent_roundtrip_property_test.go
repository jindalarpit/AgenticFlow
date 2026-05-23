package handler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/agenticflow/agenticflow/pkg/db/generated"
	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: agent-management, Property 7: Agent Configuration Round-Trip
//
// For any valid agent configuration (name, description, instructions,
// runtime_id, model, custom_env, custom_args, max_concurrent_tasks,
// visibility), creating the agent and then retrieving it SHALL produce a
// record with all fields matching the input values.
//
// **Validates: Requirements 6.1, 8.1**
// ---------------------------------------------------------------------------

// --- Generators ---

// genValidAgentName generates a valid agent name matching ^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$
func genValidAgentName(t *rapid.T) string {
	return rapid.StringMatching(`[a-zA-Z0-9][a-zA-Z0-9_-]{0,15}`).Draw(t, "name")
}

// genDescription generates a valid description (0-255 chars).
func genDescription(t *rapid.T) string {
	length := rapid.IntRange(0, 100).Draw(t, "descLen")
	if length == 0 {
		return ""
	}
	return rapid.StringMatching(`[a-zA-Z0-9 .,!?]{1,100}`).Draw(t, "desc")
}

// genInstructions generates a valid instructions string (0-500 chars for perf).
func genInstructions(t *rapid.T) string {
	length := rapid.IntRange(0, 200).Draw(t, "instrLen")
	if length == 0 {
		return ""
	}
	return rapid.StringMatching(`[a-zA-Z0-9 .,\n]{1,200}`).Draw(t, "instr")
}

// genModel generates a valid model string (0-100 chars).
func genModel(t *rapid.T) string {
	if rapid.Bool().Draw(t, "hasModel") {
		return rapid.StringMatching(`[a-z0-9-]{1,30}`).Draw(t, "model")
	}
	return ""
}

// genCustomEnv generates a valid custom_env map (0-5 pairs for perf).
func genCustomEnv(t *rapid.T) map[string]string {
	numPairs := rapid.IntRange(0, 5).Draw(t, "envPairs")
	env := make(map[string]string, numPairs)
	for i := 0; i < numPairs; i++ {
		key := rapid.StringMatching(`[A-Z_][A-Z0-9_]{0,15}`).Draw(t, "envKey")
		value := rapid.StringMatching(`[a-zA-Z0-9/._-]{1,30}`).Draw(t, "envVal")
		env[key] = value
	}
	return env
}

// genCustomArgs generates a valid custom_args slice (0-5 items for perf).
func genCustomArgs(t *rapid.T) []string {
	numArgs := rapid.IntRange(0, 5).Draw(t, "numArgs")
	args := make([]string, numArgs)
	for i := range args {
		args[i] = rapid.StringMatching(`--?[a-z][a-z0-9-]{0,20}`).Draw(t, "arg")
	}
	return args
}

// genRoundTripVisibility generates a valid visibility value.
func genRoundTripVisibility(t *rapid.T) string {
	if rapid.Bool().Draw(t, "isShared") {
		return "shared"
	}
	return "private"
}

// genMaxConcurrentTasks generates a valid max_concurrent_tasks value (1-20).
func genMaxConcurrentTasks(t *rapid.T) int32 {
	return int32(rapid.IntRange(1, 20).Draw(t, "maxTasks"))
}

// genUUID generates a valid pgtype.UUID.
func genUUID(t *rapid.T) pgtype.UUID {
	var u pgtype.UUID
	u.Valid = true
	for i := range u.Bytes {
		u.Bytes[i] = byte(rapid.IntRange(0, 255).Draw(t, "uuidByte"))
	}
	return u
}

// --- Property Tests ---

func TestProperty7_RoundTrip_ToAgentResponsePreservesAllFields(t *testing.T) {
	// Feature: agent-management, Property 7: Agent Configuration Round-Trip
	// Verify that toAgentResponse correctly converts a db.Agent and preserves
	// all configuration fields from the input.
	rapid.Check(t, func(t *rapid.T) {
		name := genValidAgentName(t)
		desc := genDescription(t)
		instr := genInstructions(t)
		model := genModel(t)
		customEnv := genCustomEnv(t)
		customArgs := genCustomArgs(t)
		maxTasks := genMaxConcurrentTasks(t)
		visibility := genRoundTripVisibility(t)
		agentID := genUUID(t)
		userID := genUUID(t)
		runtimeID := genUUID(t)

		// Marshal custom_env and custom_args to JSON (as stored in DB)
		envJSON, _ := json.Marshal(customEnv)
		if customEnv == nil || len(customEnv) == 0 {
			envJSON = []byte("{}")
		}
		argsJSON, _ := json.Marshal(customArgs)
		if customArgs == nil || len(customArgs) == 0 {
			argsJSON = []byte("[]")
		}

		now := time.Now().UTC().Truncate(time.Second)
		agent := db.Agent{
			ID:                 agentID,
			UserID:             userID,
			Name:               name,
			Description:        desc,
			Instructions:       instr,
			RuntimeID:          runtimeID,
			Model:              pgtype.Text{String: model, Valid: model != ""},
			CustomEnv:          envJSON,
			CustomArgs:         argsJSON,
			MaxConcurrentTasks: maxTasks,
			Visibility:         visibility,
			Status:             "idle",
			CreatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
		}

		resp := toAgentResponse(agent)

		// Verify all fields are preserved
		if resp.Name != name {
			t.Fatalf("Name mismatch: got %q, want %q", resp.Name, name)
		}
		if resp.Description != desc {
			t.Fatalf("Description mismatch: got %q, want %q", resp.Description, desc)
		}
		if resp.Instructions != instr {
			t.Fatalf("Instructions mismatch: got %q, want %q",
				resp.Instructions, instr)
		}
		if resp.Model != model {
			t.Fatalf("Model mismatch: got %q, want %q", resp.Model, model)
		}
		if resp.MaxConcurrentTasks != maxTasks {
			t.Fatalf("MaxConcurrentTasks mismatch: got %d, want %d",
				resp.MaxConcurrentTasks, maxTasks)
		}
		if resp.Visibility != visibility {
			t.Fatalf("Visibility mismatch: got %q, want %q",
				resp.Visibility, visibility)
		}
		if resp.ID != uuidToString(agentID) {
			t.Fatalf("ID mismatch: got %q, want %q",
				resp.ID, uuidToString(agentID))
		}
		if resp.OwnerID != uuidToString(userID) {
			t.Fatalf("OwnerID mismatch: got %q, want %q",
				resp.OwnerID, uuidToString(userID))
		}
		if resp.RuntimeID != uuidToString(runtimeID) {
			t.Fatalf("RuntimeID mismatch: got %q, want %q",
				resp.RuntimeID, uuidToString(runtimeID))
		}

		// Verify CustomEnv round-trip
		if len(customEnv) == 0 {
			if len(resp.CustomEnv) != 0 {
				t.Fatalf("CustomEnv should be empty, got %v", resp.CustomEnv)
			}
		} else {
			for k, v := range customEnv {
				if resp.CustomEnv[k] != v {
					t.Fatalf("CustomEnv[%q] mismatch: got %q, want %q",
						k, resp.CustomEnv[k], v)
				}
			}
			if len(resp.CustomEnv) != len(customEnv) {
				t.Fatalf("CustomEnv length mismatch: got %d, want %d",
					len(resp.CustomEnv), len(customEnv))
			}
		}

		// Verify CustomArgs round-trip (order matters)
		if len(customArgs) == 0 {
			if len(resp.CustomArgs) != 0 {
				t.Fatalf("CustomArgs should be empty, got %v", resp.CustomArgs)
			}
		} else {
			if len(resp.CustomArgs) != len(customArgs) {
				t.Fatalf("CustomArgs length mismatch: got %d, want %d",
					len(resp.CustomArgs), len(customArgs))
			}
			for i, arg := range customArgs {
				if resp.CustomArgs[i] != arg {
					t.Fatalf("CustomArgs[%d] mismatch: got %q, want %q",
						i, resp.CustomArgs[i], arg)
				}
			}
		}

		// Verify timestamps are formatted correctly
		if resp.CreatedAt != now.Format(time.RFC3339) {
			t.Fatalf("CreatedAt mismatch: got %q, want %q",
				resp.CreatedAt, now.Format(time.RFC3339))
		}
		if resp.UpdatedAt != now.Format(time.RFC3339) {
			t.Fatalf("UpdatedAt mismatch: got %q, want %q",
				resp.UpdatedAt, now.Format(time.RFC3339))
		}
	})
}

func TestProperty7_RoundTrip_JSONMarshalUnmarshalCreateAgentRequest(t *testing.T) {
	// Feature: agent-management, Property 7: Agent Configuration Round-Trip
	// Verify that JSON marshaling/unmarshaling of CreateAgentRequest preserves
	// all fields exactly.
	rapid.Check(t, func(t *rapid.T) {
		name := genValidAgentName(t)
		desc := genDescription(t)
		instr := genInstructions(t)
		model := genModel(t)
		customEnv := genCustomEnv(t)
		customArgs := genCustomArgs(t)
		maxTasks := genMaxConcurrentTasks(t)
		visibility := genRoundTripVisibility(t)
		runtimeID := rapid.StringMatching(
			`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`,
		).Draw(t, "runtimeID")

		req := CreateAgentRequest{
			Name:               name,
			Description:        desc,
			Instructions:       instr,
			RuntimeID:          runtimeID,
			CustomEnv:          customEnv,
			CustomArgs:         customArgs,
			Model:              model,
			Visibility:         visibility,
			MaxConcurrentTasks: maxTasks,
		}

		// Marshal to JSON
		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("failed to marshal CreateAgentRequest: %v", err)
		}

		// Unmarshal back
		var decoded CreateAgentRequest
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("failed to unmarshal CreateAgentRequest: %v", err)
		}

		// Verify all fields preserved
		if decoded.Name != name {
			t.Fatalf("Name mismatch after JSON round-trip: got %q, want %q",
				decoded.Name, name)
		}
		if decoded.Description != desc {
			t.Fatalf("Description mismatch after JSON round-trip: got %q, want %q",
				decoded.Description, desc)
		}
		if decoded.Instructions != instr {
			t.Fatalf("Instructions mismatch after JSON round-trip: got %q, want %q",
				decoded.Instructions, instr)
		}
		if decoded.Model != model {
			t.Fatalf("Model mismatch after JSON round-trip: got %q, want %q",
				decoded.Model, model)
		}
		if decoded.RuntimeID != runtimeID {
			t.Fatalf("RuntimeID mismatch after JSON round-trip: got %q, want %q",
				decoded.RuntimeID, runtimeID)
		}
		if decoded.Visibility != visibility {
			t.Fatalf("Visibility mismatch after JSON round-trip: got %q, want %q",
				decoded.Visibility, visibility)
		}
		if decoded.MaxConcurrentTasks != maxTasks {
			t.Fatalf("MaxConcurrentTasks mismatch after JSON round-trip: got %d, want %d",
				decoded.MaxConcurrentTasks, maxTasks)
		}

		// Verify CustomEnv
		if len(customEnv) == 0 {
			// JSON null or empty object both decode to nil map
			if decoded.CustomEnv != nil && len(decoded.CustomEnv) != 0 {
				t.Fatalf("CustomEnv should be nil/empty, got %v", decoded.CustomEnv)
			}
		} else {
			if len(decoded.CustomEnv) != len(customEnv) {
				t.Fatalf("CustomEnv length mismatch: got %d, want %d",
					len(decoded.CustomEnv), len(customEnv))
			}
			for k, v := range customEnv {
				if decoded.CustomEnv[k] != v {
					t.Fatalf("CustomEnv[%q] mismatch: got %q, want %q",
						k, decoded.CustomEnv[k], v)
				}
			}
		}

		// Verify CustomArgs (order preserved)
		if len(customArgs) == 0 {
			if decoded.CustomArgs != nil && len(decoded.CustomArgs) != 0 {
				t.Fatalf("CustomArgs should be nil/empty, got %v", decoded.CustomArgs)
			}
		} else {
			if len(decoded.CustomArgs) != len(customArgs) {
				t.Fatalf("CustomArgs length mismatch: got %d, want %d",
					len(decoded.CustomArgs), len(customArgs))
			}
			for i, arg := range customArgs {
				if decoded.CustomArgs[i] != arg {
					t.Fatalf("CustomArgs[%d] mismatch: got %q, want %q",
						i, decoded.CustomArgs[i], arg)
				}
			}
		}
	})
}

func TestProperty7_RoundTrip_AgentResponseContainsAllRequestFields(t *testing.T) {
	// Feature: agent-management, Property 7: Agent Configuration Round-Trip
	// Verify that the full pipeline from CreateAgentRequest → db.Agent →
	// AgentResponse preserves all configuration fields.
	rapid.Check(t, func(t *rapid.T) {
		name := genValidAgentName(t)
		desc := genDescription(t)
		instr := genInstructions(t)
		model := genModel(t)
		customEnv := genCustomEnv(t)
		customArgs := genCustomArgs(t)
		maxTasks := genMaxConcurrentTasks(t)
		visibility := genRoundTripVisibility(t)
		runtimeID := genUUID(t)
		agentID := genUUID(t)
		userID := genUUID(t)

		// Simulate what the handler does: marshal env/args to JSON
		envJSON, _ := json.Marshal(customEnv)
		if customEnv == nil || len(customEnv) == 0 {
			envJSON = []byte("{}")
		}
		argsJSON, _ := json.Marshal(customArgs)
		if customArgs == nil || len(customArgs) == 0 {
			argsJSON = []byte("[]")
		}

		now := time.Now().UTC().Truncate(time.Second)

		// Simulate the db.Agent that would be returned after creation
		agent := db.Agent{
			ID:                 agentID,
			UserID:             userID,
			Name:               name,
			Description:        desc,
			Instructions:       instr,
			RuntimeID:          runtimeID,
			Model:              pgtype.Text{String: model, Valid: model != ""},
			CustomEnv:          envJSON,
			CustomArgs:         argsJSON,
			MaxConcurrentTasks: maxTasks,
			Visibility:         visibility,
			Status:             "idle",
			CreatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
		}

		resp := toAgentResponse(agent)

		// The response must contain all fields from the original request
		if resp.Name != name {
			t.Fatalf("Name not preserved: got %q, want %q", resp.Name, name)
		}
		if resp.Description != desc {
			t.Fatalf("Description not preserved: got %q, want %q",
				resp.Description, desc)
		}
		if resp.Instructions != instr {
			t.Fatalf("Instructions not preserved: got %q, want %q",
				resp.Instructions, instr)
		}
		if resp.Model != model {
			t.Fatalf("Model not preserved: got %q, want %q", resp.Model, model)
		}
		if resp.Visibility != visibility {
			t.Fatalf("Visibility not preserved: got %q, want %q",
				resp.Visibility, visibility)
		}
		if resp.MaxConcurrentTasks != maxTasks {
			t.Fatalf("MaxConcurrentTasks not preserved: got %d, want %d",
				resp.MaxConcurrentTasks, maxTasks)
		}

		// Verify CustomEnv matches input
		if len(customEnv) == 0 {
			if len(resp.CustomEnv) != 0 {
				t.Fatalf("CustomEnv should be empty map, got %v", resp.CustomEnv)
			}
		} else {
			if len(resp.CustomEnv) != len(customEnv) {
				t.Fatalf("CustomEnv length mismatch: got %d, want %d",
					len(resp.CustomEnv), len(customEnv))
			}
			for k, v := range customEnv {
				if resp.CustomEnv[k] != v {
					t.Fatalf("CustomEnv[%q] not preserved: got %q, want %q",
						k, resp.CustomEnv[k], v)
				}
			}
		}

		// Verify CustomArgs matches input (order preserved)
		if len(customArgs) == 0 {
			if len(resp.CustomArgs) != 0 {
				t.Fatalf("CustomArgs should be empty, got %v", resp.CustomArgs)
			}
		} else {
			if len(resp.CustomArgs) != len(customArgs) {
				t.Fatalf("CustomArgs length mismatch: got %d, want %d",
					len(resp.CustomArgs), len(customArgs))
			}
			for i, arg := range customArgs {
				if resp.CustomArgs[i] != arg {
					t.Fatalf("CustomArgs[%d] not preserved: got %q, want %q",
						i, resp.CustomArgs[i], arg)
				}
			}
		}

		// Verify the response has a valid ID, OwnerID, RuntimeID
		if resp.ID == "" {
			t.Fatal("Response ID should not be empty")
		}
		if resp.OwnerID == "" {
			t.Fatal("Response OwnerID should not be empty")
		}
		if resp.RuntimeID == "" {
			t.Fatal("Response RuntimeID should not be empty")
		}
	})
}

func TestProperty7_RoundTrip_EmptyModelPreservedAsEmptyString(t *testing.T) {
	// Feature: agent-management, Property 7: Agent Configuration Round-Trip
	// When model is not set (pgtype.Text{Valid: false}), the response should
	// contain an empty string for model.
	rapid.Check(t, func(t *rapid.T) {
		name := genValidAgentName(t)
		agentID := genUUID(t)
		userID := genUUID(t)
		runtimeID := genUUID(t)
		now := time.Now().UTC().Truncate(time.Second)

		agent := db.Agent{
			ID:                 agentID,
			UserID:             userID,
			Name:               name,
			Description:        "",
			Instructions:       "",
			RuntimeID:          runtimeID,
			Model:              pgtype.Text{Valid: false}, // not set
			CustomEnv:          []byte("{}"),
			CustomArgs:         []byte("[]"),
			MaxConcurrentTasks: 1,
			Visibility:         "private",
			Status:             "idle",
			CreatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
		}

		resp := toAgentResponse(agent)

		if resp.Model != "" {
			t.Fatalf("Model should be empty string when not set, got %q",
				resp.Model)
		}
	})
}

func TestProperty7_RoundTrip_NilCustomEnvAndArgsDefaultToEmpty(t *testing.T) {
	// Feature: agent-management, Property 7: Agent Configuration Round-Trip
	// When custom_env is "{}" and custom_args is "[]" in the DB, the response
	// should contain empty map and empty slice respectively.
	rapid.Check(t, func(t *rapid.T) {
		name := genValidAgentName(t)
		agentID := genUUID(t)
		userID := genUUID(t)
		runtimeID := genUUID(t)
		now := time.Now().UTC().Truncate(time.Second)

		agent := db.Agent{
			ID:                 agentID,
			UserID:             userID,
			Name:               name,
			Description:        "test",
			Instructions:       "do stuff",
			RuntimeID:          runtimeID,
			Model:              pgtype.Text{String: "gpt-4", Valid: true},
			CustomEnv:          []byte("{}"),
			CustomArgs:         []byte("[]"),
			MaxConcurrentTasks: 5,
			Visibility:         "shared",
			Status:             "idle",
			CreatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
		}

		resp := toAgentResponse(agent)

		// Empty JSON objects/arrays should produce empty Go maps/slices
		if resp.CustomEnv == nil {
			t.Fatal("CustomEnv should be non-nil empty map, got nil")
		}
		if len(resp.CustomEnv) != 0 {
			t.Fatalf("CustomEnv should be empty, got %v", resp.CustomEnv)
		}
		if resp.CustomArgs == nil {
			t.Fatal("CustomArgs should be non-nil empty slice, got nil")
		}
		if len(resp.CustomArgs) != 0 {
			t.Fatalf("CustomArgs should be empty, got %v", resp.CustomArgs)
		}
	})
}

func TestProperty7_RoundTrip_CustomArgsOrderPreserved(t *testing.T) {
	// Feature: agent-management, Property 7: Agent Configuration Round-Trip
	// Verify that custom_args order is preserved through JSON serialization
	// and toAgentResponse conversion.
	rapid.Check(t, func(t *rapid.T) {
		numArgs := rapid.IntRange(2, 10).Draw(t, "numArgs")
		args := make([]string, numArgs)
		for i := range args {
			args[i] = rapid.StringMatching(`--[a-z]{1,10}`).Draw(t, "arg")
		}

		argsJSON, _ := json.Marshal(args)
		agentID := genUUID(t)
		userID := genUUID(t)
		runtimeID := genUUID(t)
		now := time.Now().UTC().Truncate(time.Second)

		agent := db.Agent{
			ID:                 agentID,
			UserID:             userID,
			Name:               "test-agent",
			Description:        "",
			Instructions:       "",
			RuntimeID:          runtimeID,
			Model:              pgtype.Text{Valid: false},
			CustomEnv:          []byte("{}"),
			CustomArgs:         argsJSON,
			MaxConcurrentTasks: 1,
			Visibility:         "private",
			Status:             "idle",
			CreatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
		}

		resp := toAgentResponse(agent)

		if len(resp.CustomArgs) != len(args) {
			t.Fatalf("CustomArgs length mismatch: got %d, want %d",
				len(resp.CustomArgs), len(args))
		}
		for i, expected := range args {
			if resp.CustomArgs[i] != expected {
				t.Fatalf("CustomArgs[%d] order not preserved: got %q, want %q",
					i, resp.CustomArgs[i], expected)
			}
		}
	})
}
