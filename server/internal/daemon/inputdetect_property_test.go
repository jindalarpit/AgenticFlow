package daemon

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Feature: interactive-task-sessions, Property 4: Prompt Pattern Detection
// For any output string that ends with one of the recognized prompt patterns
// ("?", "? ", ": ", "> ", "$ "), the InputDetector SHALL signal an input_requested
// state. For any output string that does not end with a recognized pattern, the
// InputDetector SHALL NOT signal based on pattern matching alone.
//
// **Validates: Requirements 5.1**

// TestProperty_PromptPatternDetection_MatchingOutput verifies that for any output
// string ending with a recognized prompt pattern (after trimming \r\n), the
// InputDetector signals waiting.
func TestProperty_PromptPatternDetection_MatchingOutput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random prefix string (can include any characters).
		prefix := rapid.String().Draw(t, "prefix")

		// Pick a random pattern from DefaultPromptPatterns.
		patternIdx := rapid.IntRange(0, len(DefaultPromptPatterns)-1).Draw(t, "patternIdx")
		pattern := DefaultPromptPatterns[patternIdx]

		// Optionally append trailing \r\n (the detector trims these before matching).
		trailingSuffix := rapid.SampledFrom([]string{"", "\n", "\r\n", "\n\n", "\r\n\r\n"}).Draw(t, "trailing")

		// Construct output: prefix + pattern + optional trailing \r\n.
		output := prefix + pattern + trailingSuffix

		// Create an InputDetector with a long inactivity timeout so the timer
		// doesn't interfere with pattern-only testing.
		var waitingCalled atomic.Int32
		detector := NewInputDetector(
			InputDetectorConfig{
				InactivityTimeout: 60 * time.Second,
			},
			func() { waitingCalled.Add(1) }, // onWaiting
			nil,                             // onCleared
		)
		defer detector.Stop()

		// Feed the output to the detector.
		detector.OnOutput(output)

		// The detector should be in waiting state (pattern matched).
		if !detector.IsWaiting() {
			t.Fatalf("expected detector to signal waiting for output ending with pattern %q, "+
				"but IsWaiting() returned false.\nFull output: %q", pattern, output)
		}
	})
}

// TestProperty_PromptPatternDetection_NonMatchingOutput verifies that for any output
// string that does NOT end with any recognized prompt pattern (after trimming \r\n),
// the InputDetector does NOT signal waiting from pattern matching alone.
func TestProperty_PromptPatternDetection_NonMatchingOutput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random base string.
		base := rapid.StringMatching(`[a-zA-Z0-9 ]{0,100}`).Draw(t, "base")

		// Append a suffix that does NOT match any pattern.
		// We use characters that cannot form any of the default patterns as a suffix.
		// DefaultPromptPatterns: "? ", ": ", "> ", "$ ", "?"
		// A safe non-matching suffix is one that doesn't end with any of these.
		nonMatchingSuffix := rapid.SampledFrom([]string{
			"!",    // exclamation
			".",    // period
			"#",    // hash
			"a",    // letter
			"1",    // digit
			")",    // paren
			"]",    // bracket
			"abc",  // word
			" ",    // just a space (no preceding ? : > $)
			"\t",   // tab
			"done", // word
		}).Draw(t, "nonMatchingSuffix")

		output := base + nonMatchingSuffix

		// Verify our generated output truly doesn't match any pattern.
		// (This is a test invariant check — if it fails, our generator is wrong.)
		trimmed := strings.TrimRight(output, "\r\n")
		for _, pattern := range DefaultPromptPatterns {
			if strings.HasSuffix(trimmed, pattern) {
				// Skip this case — our generator accidentally produced a match.
				// This can happen if base ends with "?" or ": " etc.
				t.Skip("generated output accidentally matches a pattern, skipping")
			}
		}

		// Create an InputDetector with a long inactivity timeout so the timer
		// doesn't interfere with pattern-only testing.
		var waitingCalled atomic.Int32
		detector := NewInputDetector(
			InputDetectorConfig{
				InactivityTimeout: 60 * time.Second,
			},
			func() { waitingCalled.Add(1) }, // onWaiting
			nil,                             // onCleared
		)
		defer detector.Stop()

		// Feed the output to the detector.
		detector.OnOutput(output)

		// The detector should NOT be in waiting state (no pattern matched).
		if detector.IsWaiting() {
			t.Fatalf("expected detector NOT to signal waiting for non-matching output, "+
				"but IsWaiting() returned true.\nFull output: %q\nTrimmed: %q",
				output, trimmed)
		}

		// Also verify the onWaiting callback was never called.
		if waitingCalled.Load() > 0 {
			t.Fatalf("onWaiting callback was called for non-matching output %q", output)
		}
	})
}
