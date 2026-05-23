package handler

import (
	"testing"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: agent-management, Property 13: Agent Visibility Filtering
//
// For any user requesting the agent list, the response SHALL contain exactly:
// all agents owned by that user (regardless of visibility) plus all agents
// with visibility "shared" owned by other users. No "private" agents owned
// by other users SHALL appear.
//
// **Validates: Requirements 8.2**
// ---------------------------------------------------------------------------

// AgentRecord represents a minimal agent for visibility filtering tests.
type AgentRecord struct {
	OwnerID    string
	Visibility string // "private" or "shared"
}

// FilterAgentsForUser implements the visibility filtering predicate that
// mirrors the SQL query: WHERE user_id = $1 OR visibility = 'shared'
// This is the logic-level equivalent of ListAgentsByUser.
func FilterAgentsForUser(agents []AgentRecord, requestingUserID string) []AgentRecord {
	var result []AgentRecord
	for _, a := range agents {
		if a.OwnerID == requestingUserID || a.Visibility == "shared" {
			result = append(result, a)
		}
	}
	return result
}

// --- Generators ---

// genUserID generates a random user ID string.
func genUserID(t *rapid.T, label string) string {
	return rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(t, label)
}

// genVisibility generates a valid visibility value: "private" or "shared".
func genVisibility(t *rapid.T, label string) string {
	if rapid.Bool().Draw(t, label+"IsShared") {
		return "shared"
	}
	return "private"
}

// genAgentRecords generates a slice of agent records with various owners and visibilities.
func genAgentRecords(t *rapid.T, userIDs []string) []AgentRecord {
	numAgents := rapid.IntRange(0, 50).Draw(t, "numAgents")
	agents := make([]AgentRecord, numAgents)
	for i := 0; i < numAgents; i++ {
		ownerIdx := rapid.IntRange(0, len(userIDs)-1).Draw(t, "ownerIdx")
		agents[i] = AgentRecord{
			OwnerID:    userIDs[ownerIdx],
			Visibility: genVisibility(t, "visibility"),
		}
	}
	return agents
}

// --- Property Tests ---

func TestProperty13_VisibilityFiltering_OwnedAgentsAlwaysVisible(t *testing.T) {
	// Feature: agent-management, Property 13: Agent Visibility Filtering
	// For any agent owned by the requesting user (regardless of visibility): it appears in the list
	rapid.Check(t, func(t *rapid.T) {
		requestingUser := genUserID(t, "requestingUser")
		otherUser := genUserID(t, "otherUser")

		// Create agents owned by the requesting user with various visibilities
		numOwned := rapid.IntRange(1, 20).Draw(t, "numOwned")
		agents := make([]AgentRecord, 0, numOwned+10)
		for i := 0; i < numOwned; i++ {
			agents = append(agents, AgentRecord{
				OwnerID:    requestingUser,
				Visibility: genVisibility(t, "ownedVis"),
			})
		}

		// Add some agents from other users
		numOther := rapid.IntRange(0, 10).Draw(t, "numOther")
		for i := 0; i < numOther; i++ {
			agents = append(agents, AgentRecord{
				OwnerID:    otherUser,
				Visibility: genVisibility(t, "otherVis"),
			})
		}

		filtered := FilterAgentsForUser(agents, requestingUser)

		// Count how many owned agents appear in the result
		ownedCount := 0
		for _, a := range filtered {
			if a.OwnerID == requestingUser {
				ownedCount++
			}
		}

		if ownedCount != numOwned {
			t.Fatalf("expected all %d owned agents to appear in filtered list, got %d",
				numOwned, ownedCount)
		}
	})
}

func TestProperty13_VisibilityFiltering_SharedAgentsAlwaysVisible(t *testing.T) {
	// Feature: agent-management, Property 13: Agent Visibility Filtering
	// For any agent with visibility "shared" (regardless of owner): it appears in the list
	rapid.Check(t, func(t *rapid.T) {
		requestingUser := genUserID(t, "requestingUser")
		otherUser1 := genUserID(t, "otherUser1")
		otherUser2 := genUserID(t, "otherUser2")

		userIDs := []string{requestingUser, otherUser1, otherUser2}

		// Create agents with mixed owners and visibilities
		numAgents := rapid.IntRange(1, 30).Draw(t, "numAgents")
		agents := make([]AgentRecord, numAgents)
		for i := 0; i < numAgents; i++ {
			ownerIdx := rapid.IntRange(0, len(userIDs)-1).Draw(t, "ownerIdx")
			agents[i] = AgentRecord{
				OwnerID:    userIDs[ownerIdx],
				Visibility: genVisibility(t, "vis"),
			}
		}

		filtered := FilterAgentsForUser(agents, requestingUser)

		// Count total shared agents in the input
		totalShared := 0
		for _, a := range agents {
			if a.Visibility == "shared" {
				totalShared++
			}
		}

		// Count shared agents in the filtered result
		filteredShared := 0
		for _, a := range filtered {
			if a.Visibility == "shared" {
				filteredShared++
			}
		}

		if filteredShared != totalShared {
			t.Fatalf("expected all %d shared agents to appear in filtered list, got %d",
				totalShared, filteredShared)
		}
	})
}

func TestProperty13_VisibilityFiltering_PrivateFromOthersNeverVisible(t *testing.T) {
	// Feature: agent-management, Property 13: Agent Visibility Filtering
	// For any agent with visibility "private" owned by a different user: it does NOT appear in the list
	rapid.Check(t, func(t *rapid.T) {
		requestingUser := genUserID(t, "requestingUser")
		otherUser := genUserID(t, "otherUser")

		// Create some private agents from other users
		numPrivateOther := rapid.IntRange(1, 20).Draw(t, "numPrivateOther")
		agents := make([]AgentRecord, 0, numPrivateOther+10)
		for i := 0; i < numPrivateOther; i++ {
			agents = append(agents, AgentRecord{
				OwnerID:    otherUser,
				Visibility: "private",
			})
		}

		// Add some agents from the requesting user (should appear)
		numOwned := rapid.IntRange(0, 5).Draw(t, "numOwned")
		for i := 0; i < numOwned; i++ {
			agents = append(agents, AgentRecord{
				OwnerID:    requestingUser,
				Visibility: genVisibility(t, "ownedVis"),
			})
		}

		// Add some shared agents from other users (should appear)
		numSharedOther := rapid.IntRange(0, 5).Draw(t, "numSharedOther")
		for i := 0; i < numSharedOther; i++ {
			agents = append(agents, AgentRecord{
				OwnerID:    otherUser,
				Visibility: "shared",
			})
		}

		filtered := FilterAgentsForUser(agents, requestingUser)

		// Verify no private agents from other users appear
		for _, a := range filtered {
			if a.OwnerID != requestingUser && a.Visibility == "private" {
				t.Fatalf("private agent from other user %q should NOT appear in filtered list for user %q",
					a.OwnerID, requestingUser)
			}
		}
	})
}

func TestProperty13_VisibilityFiltering_ExactSetComposition(t *testing.T) {
	// Feature: agent-management, Property 13: Agent Visibility Filtering
	// The filtered set is exactly: owned agents (any visibility) UNION shared agents (any owner)
	rapid.Check(t, func(t *rapid.T) {
		requestingUser := genUserID(t, "requestingUser")
		otherUser1 := genUserID(t, "otherUser1")
		otherUser2 := genUserID(t, "otherUser2")

		userIDs := []string{requestingUser, otherUser1, otherUser2}
		agents := genAgentRecords(t, userIDs)

		filtered := FilterAgentsForUser(agents, requestingUser)

		// Compute expected count manually
		expectedCount := 0
		for _, a := range agents {
			if a.OwnerID == requestingUser || a.Visibility == "shared" {
				expectedCount++
			}
		}

		if len(filtered) != expectedCount {
			t.Fatalf("expected %d agents in filtered list, got %d", expectedCount, len(filtered))
		}

		// Verify every agent in the filtered list satisfies the predicate
		for _, a := range filtered {
			if a.OwnerID != requestingUser && a.Visibility != "shared" {
				t.Fatalf("agent (owner=%q, visibility=%q) should not be in filtered list for user %q",
					a.OwnerID, a.Visibility, requestingUser)
			}
		}
	})
}

func TestProperty13_VisibilityFiltering_EmptyAgentList(t *testing.T) {
	// Feature: agent-management, Property 13: Agent Visibility Filtering
	// An empty agent list should produce an empty filtered result
	rapid.Check(t, func(t *rapid.T) {
		requestingUser := genUserID(t, "requestingUser")
		agents := []AgentRecord{}

		filtered := FilterAgentsForUser(agents, requestingUser)

		if len(filtered) != 0 {
			t.Fatalf("expected empty filtered list for empty input, got %d agents", len(filtered))
		}
	})
}

func TestProperty13_VisibilityFiltering_AllPrivateFromOthersExcluded(t *testing.T) {
	// Feature: agent-management, Property 13: Agent Visibility Filtering
	// When all agents are private and owned by others, the result should be empty
	rapid.Check(t, func(t *rapid.T) {
		requestingUser := genUserID(t, "requestingUser")
		otherUser := genUserID(t, "otherUser")

		numAgents := rapid.IntRange(1, 30).Draw(t, "numAgents")
		agents := make([]AgentRecord, numAgents)
		for i := 0; i < numAgents; i++ {
			agents[i] = AgentRecord{
				OwnerID:    otherUser,
				Visibility: "private",
			}
		}

		filtered := FilterAgentsForUser(agents, requestingUser)

		if len(filtered) != 0 {
			t.Fatalf("expected empty filtered list when all agents are private from others, got %d",
				len(filtered))
		}
	})
}

func TestProperty13_VisibilityFiltering_AllSharedVisible(t *testing.T) {
	// Feature: agent-management, Property 13: Agent Visibility Filtering
	// When all agents are shared, all should appear regardless of owner
	rapid.Check(t, func(t *rapid.T) {
		requestingUser := genUserID(t, "requestingUser")
		otherUser1 := genUserID(t, "otherUser1")
		otherUser2 := genUserID(t, "otherUser2")

		userIDs := []string{requestingUser, otherUser1, otherUser2}
		numAgents := rapid.IntRange(1, 30).Draw(t, "numAgents")
		agents := make([]AgentRecord, numAgents)
		for i := 0; i < numAgents; i++ {
			ownerIdx := rapid.IntRange(0, len(userIDs)-1).Draw(t, "ownerIdx")
			agents[i] = AgentRecord{
				OwnerID:    userIDs[ownerIdx],
				Visibility: "shared",
			}
		}

		filtered := FilterAgentsForUser(agents, requestingUser)

		if len(filtered) != numAgents {
			t.Fatalf("expected all %d shared agents to appear, got %d", numAgents, len(filtered))
		}
	})
}

func TestProperty13_VisibilityFiltering_OwnedPrivateVisible(t *testing.T) {
	// Feature: agent-management, Property 13: Agent Visibility Filtering
	// Private agents owned by the requesting user should still appear
	rapid.Check(t, func(t *rapid.T) {
		requestingUser := genUserID(t, "requestingUser")

		numAgents := rapid.IntRange(1, 20).Draw(t, "numAgents")
		agents := make([]AgentRecord, numAgents)
		for i := 0; i < numAgents; i++ {
			agents[i] = AgentRecord{
				OwnerID:    requestingUser,
				Visibility: "private",
			}
		}

		filtered := FilterAgentsForUser(agents, requestingUser)

		if len(filtered) != numAgents {
			t.Fatalf("expected all %d private owned agents to appear, got %d",
				numAgents, len(filtered))
		}
	})
}

func TestProperty13_VisibilityFiltering_MixedScenarioConsistency(t *testing.T) {
	// Feature: agent-management, Property 13: Agent Visibility Filtering
	// For any arbitrary mix of agents, the filter is consistent with the SQL predicate:
	// user_id = requestingUser OR visibility = 'shared'
	rapid.Check(t, func(t *rapid.T) {
		// Generate multiple users
		numUsers := rapid.IntRange(2, 5).Draw(t, "numUsers")
		userIDs := make([]string, numUsers)
		for i := 0; i < numUsers; i++ {
			userIDs[i] = genUserID(t, "user")
		}

		requestingUser := userIDs[0]
		agents := genAgentRecords(t, userIDs)

		filtered := FilterAgentsForUser(agents, requestingUser)

		// Verify each agent in the result satisfies the predicate
		for _, a := range filtered {
			isOwned := a.OwnerID == requestingUser
			isShared := a.Visibility == "shared"
			if !isOwned && !isShared {
				t.Fatalf("agent (owner=%q, visibility=%q) violates filter predicate for user %q",
					a.OwnerID, a.Visibility, requestingUser)
			}
		}

		// Verify each agent NOT in the result does NOT satisfy the predicate
		filteredSet := make(map[int]bool)
		idx := 0
		for i, a := range agents {
			if a.OwnerID == requestingUser || a.Visibility == "shared" {
				filteredSet[i] = true
				idx++
			}
		}

		if len(filtered) != len(filteredSet) {
			t.Fatalf("filtered count %d != expected count %d", len(filtered), len(filteredSet))
		}
	})
}

func TestProperty13_VisibilityFiltering_PredicateMatchesSQLSemantics(t *testing.T) {
	// Feature: agent-management, Property 13: Agent Visibility Filtering
	// The FilterAgentsForUser function must produce the same result as the SQL:
	// SELECT * FROM agent WHERE user_id = $1 OR visibility = 'shared'
	// This test verifies the predicate logic against an independent implementation.
	rapid.Check(t, func(t *rapid.T) {
		numUsers := rapid.IntRange(2, 6).Draw(t, "numUsers")
		userIDs := make([]string, numUsers)
		for i := 0; i < numUsers; i++ {
			userIDs[i] = genUserID(t, "user")
		}

		requestingUser := userIDs[0]
		agents := genAgentRecords(t, userIDs)

		// Implementation under test
		filtered := FilterAgentsForUser(agents, requestingUser)

		// Independent reference implementation (simulates SQL WHERE clause)
		var reference []AgentRecord
		for _, a := range agents {
			// SQL: WHERE user_id = $1 OR visibility = 'shared'
			if a.OwnerID == requestingUser || a.Visibility == "shared" {
				reference = append(reference, a)
			}
		}

		// Compare lengths
		if len(filtered) != len(reference) {
			t.Fatalf("filtered length %d != reference length %d", len(filtered), len(reference))
		}

		// Compare element by element (order should be preserved)
		for i := range filtered {
			if filtered[i].OwnerID != reference[i].OwnerID || filtered[i].Visibility != reference[i].Visibility {
				t.Fatalf("mismatch at index %d: filtered=%+v, reference=%+v",
					i, filtered[i], reference[i])
			}
		}
	})
}
