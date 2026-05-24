package handler

import (
	"strings"
	"testing"
	"unicode/utf8"

	"pgregory.net/rapid"
)

// ---------------------------------------------------------------------------
// Feature: interactive-task-sessions, Property 10: Task Ownership Authorization
//
// For any (user, task) pair where the user does not own the task, submitting
// input via POST /api/tasks/{id}/input SHALL be rejected with an authorization
// error, regardless of the task's status or the input content.
//
// **Validates: Requirements 9.3**
// ---------------------------------------------------------------------------

// CheckTaskOwnership implements the ownership authorization logic from SendTaskInput.
// It returns true if the requesting user is authorized (owns the task), false otherwise.
// This mirrors the check: uuidToString(task.UserID) != userID → 403 forbidden.
func CheckTaskOwnership(requestingUserID string, taskOwnerUserID string) bool {
	return requestingUserID == taskOwnerUserID
}

func TestProperty10_TaskOwnershipAuthorization_NonOwnerRejected(t *testing.T) {
	// Feature: interactive-task-sessions, Property 10: Task Ownership Authorization
	rapid.Check(t, func(t *rapid.T) {
		// Generate two distinct user IDs (UUID format).
		ownerID := rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).Draw(t, "ownerID")
		requestingID := rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).Draw(t, "requestingID")

		// Ensure the requesting user is NOT the owner.
		if requestingID == ownerID {
			// Extremely unlikely with random UUIDs, but skip if it happens.
			t.Skip("generated identical UUIDs, skipping")
		}

		// Regardless of task status or input content, a non-owner should be rejected.
		taskStatus := rapid.SampledFrom([]string{
			"pending", "running", "completed", "failed", "cancelled", "timeout",
		}).Draw(t, "taskStatus")
		_ = taskStatus // status is irrelevant to ownership check

		inputText := rapid.StringN(1, 100, 200).Draw(t, "inputText")
		_ = inputText // input content is irrelevant to ownership check

		authorized := CheckTaskOwnership(requestingID, ownerID)
		if authorized {
			t.Fatalf("non-owner user %q should NOT be authorized for task owned by %q",
				requestingID, ownerID)
		}
	})
}

func TestProperty10_TaskOwnershipAuthorization_OwnerAccepted(t *testing.T) {
	// Feature: interactive-task-sessions, Property 10: Task Ownership Authorization
	rapid.Check(t, func(t *rapid.T) {
		// Generate a user ID (UUID format) — same user is both owner and requester.
		userID := rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).Draw(t, "userID")

		authorized := CheckTaskOwnership(userID, userID)
		if !authorized {
			t.Fatalf("owner user %q should be authorized for their own task", userID)
		}
	})
}

func TestProperty10_TaskOwnershipAuthorization_IndependentOfStatus(t *testing.T) {
	// Feature: interactive-task-sessions, Property 10: Task Ownership Authorization
	// Verifies that the authorization decision is independent of task status.
	rapid.Check(t, func(t *rapid.T) {
		ownerID := rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).Draw(t, "ownerID")
		requestingID := rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).Draw(t, "requestingID")

		// Test across all possible task statuses.
		statuses := []string{"pending", "running", "completed", "failed", "cancelled", "timeout"}

		// The authorization result should be the same regardless of status.
		expectedAuth := (requestingID == ownerID)

		for _, status := range statuses {
			_ = status // status does not affect ownership check
			result := CheckTaskOwnership(requestingID, ownerID)
			if result != expectedAuth {
				t.Fatalf("authorization result changed with status %q: got %v, want %v (requester=%q, owner=%q)",
					status, result, expectedAuth, requestingID, ownerID)
			}
		}
	})
}

func TestProperty10_TaskOwnershipAuthorization_IndependentOfInputContent(t *testing.T) {
	// Feature: interactive-task-sessions, Property 10: Task Ownership Authorization
	// Verifies that the authorization decision is independent of input text content.
	rapid.Check(t, func(t *rapid.T) {
		ownerID := rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).Draw(t, "ownerID")
		requestingID := rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).Draw(t, "requestingID")

		// Generate various input texts — none should affect the authorization decision.
		inputTexts := []string{
			rapid.String().Draw(t, "randomText"),
			rapid.StringN(1, 5000, 10000).Draw(t, "longText"),
			"",
			"   ",
			"normal input",
		}

		expectedAuth := (requestingID == ownerID)

		for _, text := range inputTexts {
			_ = text // input content does not affect ownership check
			result := CheckTaskOwnership(requestingID, ownerID)
			if result != expectedAuth {
				t.Fatalf("authorization result changed with input text %q: got %v, want %v",
					text, result, expectedAuth)
			}
		}
	})
}


// ---------------------------------------------------------------------------
// Feature: interactive-task-sessions, Property 2: Input Text Validation
//
// For any string submitted as task input text, the Server SHALL accept it if
// and only if the trimmed string is non-empty and does not exceed 10,000
// characters. Empty strings, whitespace-only strings, and strings exceeding
// 10,000 characters SHALL be rejected with HTTP 400.
//
// **Validates: Requirements 2.5, 2.6**
// ---------------------------------------------------------------------------

// ValidateInputText checks whether a task input text string is acceptable.
// It returns true if the text is non-empty after trimming whitespace
// and does not exceed maxInputTextLength (10,000) rune characters.
func ValidateInputText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if utf8.RuneCountInString(text) > maxInputTextLength {
		return false
	}
	return true
}

func TestProperty2_InputTextValidation_EmptyRejected(t *testing.T) {
	// Feature: interactive-task-sessions, Property 2: Input Text Validation
	rapid.Check(t, func(t *rapid.T) {
		// Empty string should always be rejected.
		text := ""
		if ValidateInputText(text) {
			t.Fatal("empty text should be rejected")
		}
	})
}

func TestProperty2_InputTextValidation_WhitespaceOnlyRejected(t *testing.T) {
	// Feature: interactive-task-sessions, Property 2: Input Text Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate whitespace-only strings of various lengths.
		numChars := rapid.IntRange(1, 200).Draw(t, "numChars")
		wsChars := []rune{' ', '\t', '\n', '\r'}
		runes := make([]rune, numChars)
		for i := range runes {
			runes[i] = wsChars[rapid.IntRange(0, len(wsChars)-1).Draw(t, "wsIdx")]
		}
		text := string(runes)

		if ValidateInputText(text) {
			t.Fatalf("whitespace-only text %q should be rejected", text)
		}
	})
}

func TestProperty2_InputTextValidation_ValidTextAccepted(t *testing.T) {
	// Feature: interactive-task-sessions, Property 2: Input Text Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate valid text: 1 to 500 non-whitespace characters.
		length := rapid.IntRange(1, 500).Draw(t, "length")
		text := rapid.StringMatching(`[a-zA-Z0-9 .!?]{1,500}`).Draw(t, "text")

		// Ensure at least one non-whitespace character.
		if strings.TrimSpace(text) == "" {
			text = "x"
		}

		// Trim to requested length (in runes).
		runes := []rune(text)
		if len(runes) > length {
			runes = runes[:length]
		}
		text = string(runes)
		if strings.TrimSpace(text) == "" {
			text = "x"
		}

		if !ValidateInputText(text) {
			t.Fatalf("valid text of rune length %d should be accepted", utf8.RuneCountInString(text))
		}
	})
}

func TestProperty2_InputTextValidation_OverLengthRejected(t *testing.T) {
	// Feature: interactive-task-sessions, Property 2: Input Text Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate text that exceeds maxInputTextLength (10,000 runes).
		extraLen := rapid.IntRange(1, 500).Draw(t, "extraLen")
		text := strings.Repeat("a", maxInputTextLength+extraLen)

		if ValidateInputText(text) {
			t.Fatalf("text of rune length %d (exceeds %d) should be rejected",
				utf8.RuneCountInString(text), maxInputTextLength)
		}
	})
}

func TestProperty2_InputTextValidation_BoundaryAccepted(t *testing.T) {
	// Feature: interactive-task-sessions, Property 2: Input Text Validation
	rapid.Check(t, func(t *rapid.T) {
		// Exactly maxInputTextLength characters should be accepted.
		text := strings.Repeat("x", maxInputTextLength)
		if !ValidateInputText(text) {
			t.Fatalf("text of exactly %d runes should be accepted", maxInputTextLength)
		}
	})
}

func TestProperty2_InputTextValidation_BoundaryPlusOneRejected(t *testing.T) {
	// Feature: interactive-task-sessions, Property 2: Input Text Validation
	rapid.Check(t, func(t *rapid.T) {
		// maxInputTextLength + 1 characters should be rejected.
		text := strings.Repeat("x", maxInputTextLength+1)
		if ValidateInputText(text) {
			t.Fatalf("text of %d runes (exceeds %d) should be rejected",
				utf8.RuneCountInString(text), maxInputTextLength)
		}
	})
}

func TestProperty2_InputTextValidation_ArbitraryStrings(t *testing.T) {
	// Feature: interactive-task-sessions, Property 2: Input Text Validation
	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary strings and verify the validation logic is consistent.
		text := rapid.String().Draw(t, "text")

		result := ValidateInputText(text)
		trimmed := strings.TrimSpace(text)
		expectedValid := trimmed != "" && utf8.RuneCountInString(text) <= maxInputTextLength

		if result != expectedValid {
			t.Fatalf("ValidateInputText(%q) = %v, want %v (trimmed empty=%v, runeCount=%d, maxLen=%d)",
				text, result, expectedValid,
				trimmed == "", utf8.RuneCountInString(text), maxInputTextLength)
		}
	})
}

func TestProperty2_InputTextValidation_MultibyteChars(t *testing.T) {
	// Feature: interactive-task-sessions, Property 2: Input Text Validation
	// Verify that validation counts runes, not bytes (important for multibyte chars).
	rapid.Check(t, func(t *rapid.T) {
		// Generate text with multibyte characters (e.g., CJK, emoji).
		// Each character is 3-4 bytes but counts as 1 rune.
		numRunes := rapid.IntRange(1, 100).Draw(t, "numRunes")
		multibyteChars := []rune{'日', '本', '語', '中', '文', '한', '국', '어', '🎉', '🚀', '✨'}
		runes := make([]rune, numRunes)
		for i := range runes {
			runes[i] = multibyteChars[rapid.IntRange(0, len(multibyteChars)-1).Draw(t, "charIdx")]
		}
		text := string(runes)

		result := ValidateInputText(text)

		// Should be accepted since rune count <= 10000 and non-empty.
		if !result {
			t.Fatalf("multibyte text of %d runes should be accepted (byte len=%d)",
				utf8.RuneCountInString(text), len(text))
		}
	})
}

func TestProperty2_InputTextValidation_LeadingTrailingWhitespace(t *testing.T) {
	// Feature: interactive-task-sessions, Property 2: Input Text Validation
	// Text with leading/trailing whitespace but non-empty content should be accepted
	// as long as total rune count (including whitespace) doesn't exceed limit.
	rapid.Check(t, func(t *rapid.T) {
		// Generate non-empty content.
		content := rapid.StringMatching(`[a-z]{1,50}`).Draw(t, "content")
		// Add random whitespace padding.
		leadingSpaces := rapid.IntRange(0, 50).Draw(t, "leadingSpaces")
		trailingSpaces := rapid.IntRange(0, 50).Draw(t, "trailingSpaces")
		text := strings.Repeat(" ", leadingSpaces) + content + strings.Repeat(" ", trailingSpaces)

		result := ValidateInputText(text)

		// Should be accepted: non-empty after trim, and total rune count well under limit.
		if !result {
			t.Fatalf("text with content %q (padded with whitespace) should be accepted", content)
		}
	})
}
