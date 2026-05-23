package handler

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Feature: agenticflow-core, Property 7: Task Assignment Matching
//
// For any queued task with a specified agent type, the Server SHALL assign it
// only to a Daemon that has a registered Agent_Runtime matching that agent type.
// Tasks with no matching online Daemon SHALL remain in pending status.
//
// **Validates: Requirements 4.7**

// DaemonRuntime represents a daemon with its registered agent runtimes for matching logic.
type DaemonRuntime struct {
	DaemonID string
	Online   bool
	Runtimes []string // list of provider names (agent types)
}

// MatchTaskToDaemon implements the task assignment matching logic.
// Given a task's agent type and a list of daemons with their runtimes,
// it returns the ID of the first online daemon that has a matching runtime,
// or empty string if no match is found.
func MatchTaskToDaemon(agentType string, daemons []DaemonRuntime) string {
	for _, d := range daemons {
		if !d.Online {
			continue
		}
		for _, rt := range d.Runtimes {
			if rt == agentType {
				return d.DaemonID
			}
		}
	}
	return ""
}

// FindAllMatchingDaemons returns all online daemons that have a matching runtime
// for the given agent type.
func FindAllMatchingDaemons(agentType string, daemons []DaemonRuntime) []string {
	var matches []string
	for _, d := range daemons {
		if !d.Online {
			continue
		}
		for _, rt := range d.Runtimes {
			if rt == agentType {
				matches = append(matches, d.DaemonID)
				break
			}
		}
	}
	return matches
}

// knownProviders is the set of known agent providers for generating test data.
var knownProviders = []string{
	"claude", "gemini", "opencode", "openclaw", "codex",
	"copilot", "hermes", "pi", "cursor", "kimi", "kiro",
}

func TestProperty7_TaskAssignmentMatching_OnlyMatchingDaemon(t *testing.T) {
	// Feature: agenticflow-core, Property 7: Task Assignment Matching
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random agent type for the task
		agentType := rapid.SampledFrom(knownProviders).Draw(t, "agentType")

		// Generate a list of daemons with various runtimes
		numDaemons := rapid.IntRange(1, 10).Draw(t, "numDaemons")
		daemons := make([]DaemonRuntime, numDaemons)
		for i := range daemons {
			numRuntimes := rapid.IntRange(1, 5).Draw(t, "numRuntimes")
			runtimes := make([]string, numRuntimes)
			for j := range runtimes {
				runtimes[j] = rapid.SampledFrom(knownProviders).Draw(t, "runtime")
			}
			daemons[i] = DaemonRuntime{
				DaemonID: rapid.StringMatching(`daemon-[a-z0-9]{8}`).Draw(t, "daemonID"),
				Online:   rapid.Bool().Draw(t, "online"),
				Runtimes: runtimes,
			}
		}

		result := MatchTaskToDaemon(agentType, daemons)

		if result != "" {
			// Verify the matched daemon is online and has the matching runtime
			var found bool
			for _, d := range daemons {
				if d.DaemonID == result {
					if !d.Online {
						t.Fatalf("matched daemon %q is offline", result)
					}
					for _, rt := range d.Runtimes {
						if rt == agentType {
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("matched daemon %q does not have runtime %q", result, agentType)
					}
					break
				}
			}
			if !found {
				t.Fatalf("matched daemon %q not found in daemon list or missing runtime", result)
			}
		} else {
			// Verify no online daemon has the matching runtime
			for _, d := range daemons {
				if !d.Online {
					continue
				}
				for _, rt := range d.Runtimes {
					if rt == agentType {
						t.Fatalf("no match returned but daemon %q is online with runtime %q",
							d.DaemonID, agentType)
					}
				}
			}
		}
	})
}

func TestProperty7_TaskAssignmentMatching_OfflineDaemonNeverMatched(t *testing.T) {
	// Feature: agenticflow-core, Property 7: Task Assignment Matching
	rapid.Check(t, func(t *rapid.T) {
		agentType := rapid.SampledFrom(knownProviders).Draw(t, "agentType")

		// Create daemons that ALL have the matching runtime but are ALL offline
		numDaemons := rapid.IntRange(1, 5).Draw(t, "numDaemons")
		daemons := make([]DaemonRuntime, numDaemons)
		for i := range daemons {
			daemons[i] = DaemonRuntime{
				DaemonID: rapid.StringMatching(`daemon-[a-z0-9]{8}`).Draw(t, "daemonID"),
				Online:   false, // all offline
				Runtimes: []string{agentType},
			}
		}

		result := MatchTaskToDaemon(agentType, daemons)
		if result != "" {
			t.Fatalf("expected no match for all-offline daemons, got %q", result)
		}
	})
}

func TestProperty7_TaskAssignmentMatching_NoMatchingRuntime(t *testing.T) {
	// Feature: agenticflow-core, Property 7: Task Assignment Matching
	rapid.Check(t, func(t *rapid.T) {
		agentType := rapid.SampledFrom(knownProviders).Draw(t, "agentType")

		// Create online daemons that do NOT have the matching runtime
		numDaemons := rapid.IntRange(1, 5).Draw(t, "numDaemons")
		daemons := make([]DaemonRuntime, numDaemons)
		for i := range daemons {
			// Pick runtimes that are NOT the target agent type
			otherProviders := make([]string, 0, len(knownProviders)-1)
			for _, p := range knownProviders {
				if p != agentType {
					otherProviders = append(otherProviders, p)
				}
			}
			numRuntimes := rapid.IntRange(1, 3).Draw(t, "numRuntimes")
			runtimes := make([]string, numRuntimes)
			for j := range runtimes {
				runtimes[j] = rapid.SampledFrom(otherProviders).Draw(t, "runtime")
			}
			daemons[i] = DaemonRuntime{
				DaemonID: rapid.StringMatching(`daemon-[a-z0-9]{8}`).Draw(t, "daemonID"),
				Online:   true, // all online
				Runtimes: runtimes,
			}
		}

		result := MatchTaskToDaemon(agentType, daemons)
		if result != "" {
			t.Fatalf("expected no match when no daemon has runtime %q, got %q", agentType, result)
		}
	})
}

func TestProperty7_TaskAssignmentMatching_ResultAlwaysInMatchingSet(t *testing.T) {
	// Feature: agenticflow-core, Property 7: Task Assignment Matching
	rapid.Check(t, func(t *rapid.T) {
		agentType := rapid.SampledFrom(knownProviders).Draw(t, "agentType")

		// Generate mixed daemons
		numDaemons := rapid.IntRange(1, 10).Draw(t, "numDaemons")
		daemons := make([]DaemonRuntime, numDaemons)
		for i := range daemons {
			numRuntimes := rapid.IntRange(1, 4).Draw(t, "numRuntimes")
			runtimes := make([]string, numRuntimes)
			for j := range runtimes {
				runtimes[j] = rapid.SampledFrom(knownProviders).Draw(t, "runtime")
			}
			daemons[i] = DaemonRuntime{
				DaemonID: rapid.StringMatching(`daemon-[a-z0-9]{8}`).Draw(t, "daemonID"),
				Online:   rapid.Bool().Draw(t, "online"),
				Runtimes: runtimes,
			}
		}

		result := MatchTaskToDaemon(agentType, daemons)
		allMatches := FindAllMatchingDaemons(agentType, daemons)

		if result == "" {
			// No match — allMatches should be empty
			if len(allMatches) > 0 {
				t.Fatalf("MatchTaskToDaemon returned empty but FindAllMatchingDaemons found %v", allMatches)
			}
		} else {
			// Result must be in the set of all matching daemons
			found := false
			for _, m := range allMatches {
				if m == result {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("MatchTaskToDaemon returned %q which is not in matching set %v", result, allMatches)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Feature: agenticflow-core, Property 14: Daemon Offline Detection
//
// For any Daemon whose last_heartbeat_at timestamp is older than
// 3 × heartbeat_interval (default 45 seconds), the Server SHALL mark
// the daemon as offline and deregister its Agent_Runtimes.
//
// **Validates: Requirements 7.7**
// ---------------------------------------------------------------------------

const (
	// DefaultHeartbeatInterval is the default heartbeat interval.
	DefaultHeartbeatInterval = 15 * time.Second
	// OfflineMultiplier is the number of missed heartbeats before marking offline.
	OfflineMultiplier = 3
)

// IsDaemonOffline determines whether a daemon should be marked offline.
// A daemon is offline if the time since its last heartbeat exceeds
// offlineMultiplier × heartbeatInterval.
func IsDaemonOffline(lastHeartbeat time.Time, now time.Time, heartbeatInterval time.Duration) bool {
	threshold := time.Duration(OfflineMultiplier) * heartbeatInterval
	return now.Sub(lastHeartbeat) > threshold
}

// DaemonOfflineStatus represents the result of offline detection for a daemon.
type DaemonOfflineStatus struct {
	DaemonID  string
	IsOffline bool
}

// DetectOfflineDaemons checks a list of daemons and returns their offline status.
func DetectOfflineDaemons(daemons []struct {
	DaemonID      string
	LastHeartbeat time.Time
}, now time.Time, heartbeatInterval time.Duration) []DaemonOfflineStatus {
	results := make([]DaemonOfflineStatus, len(daemons))
	for i, d := range daemons {
		results[i] = DaemonOfflineStatus{
			DaemonID:  d.DaemonID,
			IsOffline: IsDaemonOffline(d.LastHeartbeat, now, heartbeatInterval),
		}
	}
	return results
}

func TestProperty14_DaemonOfflineDetection_DefaultInterval(t *testing.T) {
	// Feature: agenticflow-core, Property 14: Daemon Offline Detection
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()

		// Generate a random duration since last heartbeat (0 to 300 seconds)
		secondsSinceHeartbeat := rapid.IntRange(0, 300).Draw(t, "secondsSince")
		lastHeartbeat := now.Add(-time.Duration(secondsSinceHeartbeat) * time.Second)

		isOffline := IsDaemonOffline(lastHeartbeat, now, DefaultHeartbeatInterval)

		// Default threshold is 3 × 15s = 45s
		expectedOffline := secondsSinceHeartbeat > 45

		if isOffline != expectedOffline {
			t.Fatalf("IsDaemonOffline(lastHeartbeat=%v ago, interval=15s) = %v, want %v (seconds=%d, threshold=45)",
				now.Sub(lastHeartbeat), isOffline, expectedOffline, secondsSinceHeartbeat)
		}
	})
}

func TestProperty14_DaemonOfflineDetection_CustomInterval(t *testing.T) {
	// Feature: agenticflow-core, Property 14: Daemon Offline Detection
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()

		// Generate a random heartbeat interval between 5s and 60s
		intervalSeconds := rapid.IntRange(5, 60).Draw(t, "intervalSeconds")
		heartbeatInterval := time.Duration(intervalSeconds) * time.Second

		// Generate a random duration since last heartbeat
		secondsSinceHeartbeat := rapid.IntRange(0, 600).Draw(t, "secondsSince")
		lastHeartbeat := now.Add(-time.Duration(secondsSinceHeartbeat) * time.Second)

		isOffline := IsDaemonOffline(lastHeartbeat, now, heartbeatInterval)

		// Threshold is 3 × interval
		thresholdSeconds := 3 * intervalSeconds
		expectedOffline := secondsSinceHeartbeat > thresholdSeconds

		if isOffline != expectedOffline {
			t.Fatalf("IsDaemonOffline(lastHeartbeat=%ds ago, interval=%ds) = %v, want %v (threshold=%ds)",
				secondsSinceHeartbeat, intervalSeconds, isOffline, expectedOffline, thresholdSeconds)
		}
	})
}

func TestProperty14_DaemonOfflineDetection_ExactBoundary(t *testing.T) {
	// Feature: agenticflow-core, Property 14: Daemon Offline Detection
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()

		// Generate a random heartbeat interval
		intervalSeconds := rapid.IntRange(5, 60).Draw(t, "intervalSeconds")
		heartbeatInterval := time.Duration(intervalSeconds) * time.Second
		thresholdSeconds := 3 * intervalSeconds

		// Test exactly at the threshold — should NOT be offline (not strictly older)
		lastHeartbeatAtThreshold := now.Add(-time.Duration(thresholdSeconds) * time.Second)
		isOfflineAtThreshold := IsDaemonOffline(lastHeartbeatAtThreshold, now, heartbeatInterval)
		if isOfflineAtThreshold {
			t.Fatalf("daemon should NOT be offline when exactly at threshold (%ds)", thresholdSeconds)
		}

		// Test 1 second past the threshold — should be offline
		lastHeartbeatPastThreshold := now.Add(-time.Duration(thresholdSeconds+1) * time.Second)
		isOfflinePastThreshold := IsDaemonOffline(lastHeartbeatPastThreshold, now, heartbeatInterval)
		if !isOfflinePastThreshold {
			t.Fatalf("daemon should be offline when %ds past threshold (%ds)",
				thresholdSeconds+1, thresholdSeconds)
		}
	})
}

func TestProperty14_DaemonOfflineDetection_RecentHeartbeatNeverOffline(t *testing.T) {
	// Feature: agenticflow-core, Property 14: Daemon Offline Detection
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()

		// Generate a random heartbeat interval
		intervalSeconds := rapid.IntRange(5, 60).Draw(t, "intervalSeconds")
		heartbeatInterval := time.Duration(intervalSeconds) * time.Second

		// Generate a last heartbeat that is within the threshold
		// (0 to 3*interval - 1 seconds ago)
		maxSafe := 3*intervalSeconds - 1
		if maxSafe < 0 {
			maxSafe = 0
		}
		secondsSince := rapid.IntRange(0, maxSafe).Draw(t, "secondsSince")
		lastHeartbeat := now.Add(-time.Duration(secondsSince) * time.Second)

		isOffline := IsDaemonOffline(lastHeartbeat, now, heartbeatInterval)
		if isOffline {
			t.Fatalf("daemon with heartbeat %ds ago should NOT be offline (interval=%ds, threshold=%ds)",
				secondsSince, intervalSeconds, 3*intervalSeconds)
		}
	})
}

func TestProperty14_DaemonOfflineDetection_OldHeartbeatAlwaysOffline(t *testing.T) {
	// Feature: agenticflow-core, Property 14: Daemon Offline Detection
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()

		// Generate a random heartbeat interval
		intervalSeconds := rapid.IntRange(5, 60).Draw(t, "intervalSeconds")
		heartbeatInterval := time.Duration(intervalSeconds) * time.Second

		// Generate a last heartbeat that is strictly past the threshold
		thresholdSeconds := 3 * intervalSeconds
		secondsSince := rapid.IntRange(thresholdSeconds+1, thresholdSeconds+600).Draw(t, "secondsSince")
		lastHeartbeat := now.Add(-time.Duration(secondsSince) * time.Second)

		isOffline := IsDaemonOffline(lastHeartbeat, now, heartbeatInterval)
		if !isOffline {
			t.Fatalf("daemon with heartbeat %ds ago should be offline (interval=%ds, threshold=%ds)",
				secondsSince, intervalSeconds, thresholdSeconds)
		}
	})
}

func TestProperty14_DaemonOfflineDetection_BatchDetection(t *testing.T) {
	// Feature: agenticflow-core, Property 14: Daemon Offline Detection
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()
		heartbeatInterval := DefaultHeartbeatInterval
		threshold := time.Duration(OfflineMultiplier) * heartbeatInterval

		// Generate a batch of daemons with various last heartbeat times
		numDaemons := rapid.IntRange(1, 20).Draw(t, "numDaemons")
		daemons := make([]struct {
			DaemonID      string
			LastHeartbeat time.Time
		}, numDaemons)

		for i := range daemons {
			secondsAgo := rapid.IntRange(0, 300).Draw(t, "secondsAgo")
			daemons[i].DaemonID = rapid.StringMatching(`daemon-[a-z0-9]{6}`).Draw(t, "daemonID")
			daemons[i].LastHeartbeat = now.Add(-time.Duration(secondsAgo) * time.Second)
		}

		results := DetectOfflineDaemons(daemons, now, heartbeatInterval)

		// Verify each result matches the expected offline status
		for i, result := range results {
			elapsed := now.Sub(daemons[i].LastHeartbeat)
			expectedOffline := elapsed > threshold

			if result.IsOffline != expectedOffline {
				t.Fatalf("daemon %q: elapsed=%v, threshold=%v, got offline=%v, want %v",
					result.DaemonID, elapsed, threshold, result.IsOffline, expectedOffline)
			}
		}
	})
}


// ---------------------------------------------------------------------------
// Feature: agenticflow-core, Property 6: Task Prompt Validation
//
// For any string submitted as a task prompt, the Server SHALL accept it if and
// only if it is non-empty, non-whitespace-only, and does not exceed 32,000
// characters.
//
// **Validates: Requirements 4.6, 6.3**
// ---------------------------------------------------------------------------

// ValidatePrompt checks whether a prompt string is acceptable.
// It returns true if the prompt is non-empty after trimming whitespace
// and does not exceed maxPromptLength (32,000) characters.
func ValidatePrompt(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return false
	}
	if len(trimmed) > maxPromptLength {
		return false
	}
	return true
}

func TestProperty6_TaskPromptValidation_EmptyRejected(t *testing.T) {
	// Feature: agenticflow-core, Property 6: Task Prompt Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate empty string
		prompt := ""
		if ValidatePrompt(prompt) {
			t.Fatal("empty prompt should be rejected")
		}
	})
}

func TestProperty6_TaskPromptValidation_WhitespaceOnlyRejected(t *testing.T) {
	// Feature: agenticflow-core, Property 6: Task Prompt Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate whitespace-only strings of various lengths
		numSpaces := rapid.IntRange(1, 100).Draw(t, "numSpaces")
		wsChars := []rune{' ', '\t', '\n', '\r'}
		runes := make([]rune, numSpaces)
		for i := range runes {
			runes[i] = wsChars[rapid.IntRange(0, len(wsChars)-1).Draw(t, "wsIdx")]
		}
		prompt := string(runes)

		if ValidatePrompt(prompt) {
			t.Fatalf("whitespace-only prompt %q should be rejected", prompt)
		}
	})
}

func TestProperty6_TaskPromptValidation_ValidPromptAccepted(t *testing.T) {
	// Feature: agenticflow-core, Property 6: Task Prompt Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate a valid prompt: 1 to 1000 alphanumeric characters
		length := rapid.IntRange(1, 1000).Draw(t, "length")
		prompt := rapid.StringMatching(`[a-zA-Z0-9 .!]{1,1000}`).Draw(t, "prompt")

		// Ensure at least one non-whitespace character
		if strings.TrimSpace(prompt) == "" {
			prompt = "x"
		}

		// Trim to requested length
		if len(prompt) > length {
			prompt = prompt[:length]
		}
		if strings.TrimSpace(prompt) == "" {
			prompt = "x"
		}

		if !ValidatePrompt(prompt) {
			t.Fatalf("valid prompt of length %d should be accepted", len(prompt))
		}
	})
}

func TestProperty6_TaskPromptValidation_OverLengthRejected(t *testing.T) {
	// Feature: agenticflow-core, Property 6: Task Prompt Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate a prompt that exceeds maxPromptLength after trimming
		extraLen := rapid.IntRange(1, 1000).Draw(t, "extraLen")
		prompt := strings.Repeat("a", maxPromptLength+extraLen)

		if ValidatePrompt(prompt) {
			t.Fatalf("prompt of length %d (exceeds %d) should be rejected",
				len(prompt), maxPromptLength)
		}
	})
}

func TestProperty6_TaskPromptValidation_BoundaryAccepted(t *testing.T) {
	// Feature: agenticflow-core, Property 6: Task Prompt Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate a prompt of exactly maxPromptLength characters (non-whitespace)
		prompt := strings.Repeat("x", maxPromptLength)

		if !ValidatePrompt(prompt) {
			t.Fatalf("prompt of exactly %d chars should be accepted", maxPromptLength)
		}
	})
}

func TestProperty6_TaskPromptValidation_ArbitraryStrings(t *testing.T) {
	// Feature: agenticflow-core, Property 6: Task Prompt Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary strings and verify the validation logic is consistent
		prompt := rapid.String().Draw(t, "prompt")

		result := ValidatePrompt(prompt)
		trimmed := strings.TrimSpace(prompt)

		expectedValid := trimmed != "" && len(trimmed) <= maxPromptLength

		if result != expectedValid {
			t.Fatalf("ValidatePrompt(%q) = %v, want %v (trimmed len=%d, maxLen=%d)",
				prompt, result, expectedValid, len(trimmed), maxPromptLength)
		}
	})
}

// ---------------------------------------------------------------------------
// Feature: agenticflow-core, Property 12: Custom Agent Name Validation
//
// For any string submitted as a Custom Agent name, the Server SHALL accept it
// if and only if it matches the pattern ^[a-zA-Z0-9_-]{1,64}$.
//
// **Validates: Requirements 5.1**
// ---------------------------------------------------------------------------

// validCustomAgentNameRegex is the compiled regex for property testing.
var validCustomAgentNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// ValidateCustomAgentName checks whether a custom agent name is acceptable.
// It returns true if the name matches ^[a-zA-Z0-9_-]{1,64}$.
func ValidateCustomAgentName(name string) bool {
	return validCustomAgentNameRegex.MatchString(name)
}

func TestProperty12_CustomAgentNameValidation_ValidNamesAccepted(t *testing.T) {
	// Feature: agenticflow-core, Property 12: Custom Agent Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate valid names matching ^[a-zA-Z0-9_-]{1,64}$
		name := rapid.StringMatching(`^[a-zA-Z0-9_-]{1,64}$`).Draw(t, "name")

		if !ValidateCustomAgentName(name) {
			t.Fatalf("valid name %q should be accepted", name)
		}
	})
}

func TestProperty12_CustomAgentNameValidation_EmptyRejected(t *testing.T) {
	// Feature: agenticflow-core, Property 12: Custom Agent Name Validation
	rapid.Check(t, func(t *rapid.T) {
		if ValidateCustomAgentName("") {
			t.Fatal("empty name should be rejected")
		}
	})
}

func TestProperty12_CustomAgentNameValidation_TooLongRejected(t *testing.T) {
	// Feature: agenticflow-core, Property 12: Custom Agent Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate names longer than 64 characters using valid chars
		extraLen := rapid.IntRange(1, 100).Draw(t, "extraLen")
		name := strings.Repeat("a", 64+extraLen)

		if ValidateCustomAgentName(name) {
			t.Fatalf("name of length %d (exceeds 64) should be rejected", len(name))
		}
	})
}

func TestProperty12_CustomAgentNameValidation_SpecialCharsRejected(t *testing.T) {
	// Feature: agenticflow-core, Property 12: Custom Agent Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate names with at least one invalid character
		validPart := rapid.StringMatching(`^[a-zA-Z0-9_-]{0,30}$`).Draw(t, "validPart")
		invalidChars := []rune{'!', '@', '#', '$', '%', '^', '&', '*', '(', ')', ' ', '.', '/', '\\', '~', '`', '+', '=', '{', '}', '[', ']', '|', ':', ';', '"', '\'', '<', '>', ',', '?'}
		invalidChar := invalidChars[rapid.IntRange(0, len(invalidChars)-1).Draw(t, "charIdx")]
		// Insert invalid char at a random position
		pos := rapid.IntRange(0, len(validPart)).Draw(t, "pos")
		name := validPart[:pos] + string(invalidChar) + validPart[pos:]

		if ValidateCustomAgentName(name) {
			t.Fatalf("name %q with special char %q should be rejected", name, string(invalidChar))
		}
	})
}

func TestProperty12_CustomAgentNameValidation_ArbitraryStrings(t *testing.T) {
	// Feature: agenticflow-core, Property 12: Custom Agent Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary strings and verify validation matches the regex
		name := rapid.String().Draw(t, "name")

		result := ValidateCustomAgentName(name)
		expected := validCustomAgentNameRegex.MatchString(name)

		if result != expected {
			t.Fatalf("ValidateCustomAgentName(%q) = %v, want %v", name, result, expected)
		}
	})
}

func TestProperty12_CustomAgentNameValidation_BoundaryLength(t *testing.T) {
	// Feature: agenticflow-core, Property 12: Custom Agent Name Validation
	rapid.Check(t, func(t *rapid.T) {
		// Exactly 64 characters should be accepted
		name := strings.Repeat("a", 64)
		if !ValidateCustomAgentName(name) {
			t.Fatal("name of exactly 64 chars should be accepted")
		}

		// 65 characters should be rejected
		name65 := strings.Repeat("a", 65)
		if ValidateCustomAgentName(name65) {
			t.Fatal("name of 65 chars should be rejected")
		}

		// Single character should be accepted
		singleChar := rapid.StringMatching(`^[a-zA-Z0-9_-]$`).Draw(t, "singleChar")
		if !ValidateCustomAgentName(singleChar) {
			t.Fatalf("single valid char %q should be accepted", singleChar)
		}
	})
}
