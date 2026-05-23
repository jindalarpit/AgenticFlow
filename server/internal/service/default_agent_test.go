package service

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestDefaultAgentConstants(t *testing.T) {
	// Verify the default agent constants match the requirements.
	if DefaultAgentName != "Nexus" {
		t.Errorf("DefaultAgentName = %q, want %q", DefaultAgentName, "Nexus")
	}
	if DefaultAgentDescription != "Your local AI coding agent" {
		t.Errorf("DefaultAgentDescription = %q, want %q", DefaultAgentDescription, "Your local AI coding agent")
	}
}

func TestParseUUID_Valid(t *testing.T) {
	validUUID := "12345678-1234-1234-1234-123456789abc"
	parsed, err := parseUUID(validUUID)
	if err != nil {
		t.Fatalf("parseUUID(%q) failed: %v", validUUID, err)
	}
	if !parsed.Valid {
		t.Error("expected parsed UUID to be valid")
	}
}

func TestParseUUID_Empty(t *testing.T) {
	_, err := parseUUID("")
	if err == nil {
		t.Error("expected parseUUID to fail for empty string")
	}
}

func TestParseUUID_Invalid(t *testing.T) {
	_, err := parseUUID("not-a-uuid")
	if err == nil {
		t.Error("expected parseUUID to fail for invalid UUID")
	}
}

func TestEnsureDefaultAgent_RuntimeIDSelection(t *testing.T) {
	// Verify that parseUUID correctly identifies valid runtime IDs from a map.
	runtimeIDs := map[string]string{
		"claude": "12345678-1234-1234-1234-123456789abc",
		"codex":  "invalid-uuid",
	}

	var firstValid pgtype.UUID
	for _, rid := range runtimeIDs {
		parsed, err := parseUUID(rid)
		if err == nil {
			firstValid = parsed
			break
		}
	}

	if !firstValid.Valid {
		t.Error("expected to find at least one valid runtime ID")
	}
}

func TestEnsureDefaultAgent_NoValidRuntimeIDs(t *testing.T) {
	// When all runtime IDs are invalid, no UUID should be selected.
	runtimeIDs := map[string]string{
		"claude": "invalid-1",
		"codex":  "invalid-2",
	}

	var firstValid pgtype.UUID
	for _, rid := range runtimeIDs {
		parsed, err := parseUUID(rid)
		if err == nil {
			firstValid = parsed
			break
		}
	}

	if firstValid.Valid {
		t.Error("expected no valid runtime ID to be found")
	}
}

func TestEnsureDefaultAgent_EmptyRuntimeIDs(t *testing.T) {
	// When the runtime IDs map is empty, no UUID should be selected.
	runtimeIDs := map[string]string{}

	var firstValid pgtype.UUID
	for _, rid := range runtimeIDs {
		parsed, err := parseUUID(rid)
		if err == nil {
			firstValid = parsed
			break
		}
	}

	if firstValid.Valid {
		t.Error("expected no valid runtime ID from empty map")
	}
}
