package agent

import (
	"regexp"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: agentic-output-architecture, Property 4: ACP Tool Name Normalization
//
// **Validates: Requirements 5.6, 11.4**

// validSnakeCase matches a valid snake_case identifier: lowercase letters,
// digits, and underscores only, with no leading/trailing underscores and
// no consecutive underscores.
var validSnakeCase = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// isValidSnakeCase checks that a non-empty string is valid snake_case:
// lowercase, underscores allowed (not leading/trailing), no spaces.
func isValidSnakeCase(s string) bool {
	if s == "" {
		return false
	}
	// Must not have leading or trailing underscores
	if strings.HasPrefix(s, "_") || strings.HasSuffix(s, "_") {
		return false
	}
	// Must not contain spaces
	if strings.Contains(s, " ") {
		return false
	}
	// Must be all lowercase (no uppercase letters)
	if s != strings.ToLower(s) {
		return false
	}
	// Must match valid snake_case pattern
	return validSnakeCase.MatchString(s)
}

// TestNormalizeACPToolName_SnakeCase tests that for any non-empty ACP tool
// title, NormalizeACPToolName produces a valid snake_case identifier.
func TestNormalizeACPToolName_SnakeCase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate non-empty strings that simulate ACP tool titles.
		// ACP titles are typically human-readable names like "Read file",
		// "Run command", "List directory", etc.
		title := rapid.OneOf(
			// Common ACP-style titles with spaces
			rapid.SampledFrom([]string{
				"Read file", "Write file", "Run command", "List directory",
				"Search files", "Web search", "Web fetch", "Execute code",
				"Delegate task", "Edit", "Patch", "Shell", "Bash",
				"Terminal", "Grep", "Search", "Find", "Glob", "Code",
				"Todo", "Todo write", "Todo list",
				"Read file: /path/to/file.go",
				"Run command: ls -la",
				"List dir",
			}),
			// Random word-based titles (1-3 words)
			rapid.Custom(func(t *rapid.T) string {
				numWords := rapid.IntRange(1, 3).Draw(t, "numWords")
				words := make([]string, numWords)
				for i := range words {
					words[i] = rapid.StringMatching(`[A-Za-z][a-z]{1,8}`).Draw(t, "word")
				}
				return strings.Join(words, " ")
			}),
		).Draw(t, "title")

		result := NormalizeACPToolName(title)

		// The result must be non-empty for non-empty input
		if result == "" {
			t.Fatalf("NormalizeACPToolName(%q) returned empty string", title)
		}

		// The result must be valid snake_case
		if !isValidSnakeCase(result) {
			t.Fatalf("NormalizeACPToolName(%q) = %q is not valid snake_case", title, result)
		}
	})
}

// TestNormalizeACPToolName_Idempotence tests that normalizing twice produces
// the same result as normalizing once.
func TestNormalizeACPToolName_Idempotence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a variety of inputs including already-normalized names
		title := rapid.OneOf(
			// ACP-style titles
			rapid.SampledFrom([]string{
				"Read file", "Write file", "Run command", "List directory",
				"Search files", "Web search", "Web fetch", "Execute code",
				"Delegate task", "Edit", "Patch", "Shell", "Bash",
				"Terminal", "Grep", "Search", "Find", "Glob", "Code",
				"Todo write", "List dir",
				"Read file: /some/path",
			}),
			// Already-normalized snake_case names
			rapid.SampledFrom([]string{
				"read_file", "write_file", "terminal", "search_files",
				"web_search", "web_fetch", "execute_code", "delegate_task",
				"list_directory", "todo_write", "glob", "code",
			}),
			// Random word-based titles
			rapid.Custom(func(t *rapid.T) string {
				numWords := rapid.IntRange(1, 3).Draw(t, "numWords")
				words := make([]string, numWords)
				for i := range words {
					words[i] = rapid.StringMatching(`[A-Za-z][a-z]{1,8}`).Draw(t, "word")
				}
				return strings.Join(words, " ")
			}),
		).Draw(t, "title")

		once := NormalizeACPToolName(title)
		twice := NormalizeACPToolName(once)

		if once != twice {
			t.Fatalf("idempotence violated: NormalizeACPToolName(%q) = %q, but NormalizeACPToolName(%q) = %q",
				title, once, once, twice)
		}
	})
}

// TestNormalizeACPToolName_EmptyInput verifies that empty/whitespace input
// returns an empty string.
func TestNormalizeACPToolName_EmptyInput(t *testing.T) {
	cases := []string{"", "   ", "\t", "\n"}
	for _, input := range cases {
		result := NormalizeACPToolName(input)
		if result != "" {
			t.Errorf("NormalizeACPToolName(%q) = %q, want empty string", input, result)
		}
	}
}
