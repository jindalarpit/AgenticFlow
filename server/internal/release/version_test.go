package release

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestFormatVersionContainsAllMetadata is a property-based test verifying that
// FormatVersion always includes all three injected metadata values in its output.
//
// **Validates: Requirements 1.3, 5.2, 6.1, 6.2, 6.3, 6.4**
func TestFormatVersionContainsAllMetadata(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random valid version string X.Y.Z with non-negative integers
		major := rapid.IntRange(0, 999).Draw(t, "major")
		minor := rapid.IntRange(0, 999).Draw(t, "minor")
		patch := rapid.IntRange(0, 999).Draw(t, "patch")
		version := fmt.Sprintf("%d.%d.%d", major, minor, patch)

		// Generate random 7-character hex commit hash
		hexChars := "0123456789abcdef"
		commitBytes := make([]byte, 7)
		for i := range commitBytes {
			commitBytes[i] = hexChars[rapid.IntRange(0, 15).Draw(t, fmt.Sprintf("hex_%d", i))]
		}
		commit := string(commitBytes)

		// Generate random RFC 3339 date string
		year := rapid.IntRange(2020, 2099).Draw(t, "year")
		month := rapid.IntRange(1, 12).Draw(t, "month")
		day := rapid.IntRange(1, 28).Draw(t, "day")
		hour := rapid.IntRange(0, 23).Draw(t, "hour")
		minute := rapid.IntRange(0, 59).Draw(t, "minute")
		second := rapid.IntRange(0, 59).Draw(t, "second")
		date := fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02dZ", year, month, day, hour, minute, second)

		// Call FormatVersion
		output := FormatVersion(version, commit, date)

		// Property: output must contain all three injected values
		if !strings.Contains(output, version) {
			t.Fatalf("output %q does not contain version %q", output, version)
		}
		if !strings.Contains(output, commit) {
			t.Fatalf("output %q does not contain commit %q", output, commit)
		}
		if !strings.Contains(output, date) {
			t.Fatalf("output %q does not contain date %q", output, date)
		}
	})
}

// Unit tests for version formatting edge cases
// Validates: Requirement 6.4

func TestFormatVersion_DevBuild(t *testing.T) {
	// Default values when not built with GoReleaser
	got := FormatVersion("dev", "unknown", "unknown")

	if !strings.Contains(got, "dev") {
		t.Errorf("expected output to contain 'dev', got: %s", got)
	}
	if !strings.Contains(got, "unknown") {
		t.Errorf("expected output to contain 'unknown', got: %s", got)
	}
	if got != "af dev (commit: unknown, built: unknown)" {
		t.Errorf("unexpected format for dev build, got: %s", got)
	}
}

func TestFormatVersion_NormalRelease(t *testing.T) {
	got := FormatVersion("0.1.0", "abc1234", "2025-01-01T00:00:00Z")

	if !strings.Contains(got, "0.1.0") {
		t.Errorf("expected output to contain version '0.1.0', got: %s", got)
	}
	if !strings.Contains(got, "abc1234") {
		t.Errorf("expected output to contain commit 'abc1234', got: %s", got)
	}
	if !strings.Contains(got, "2025-01-01T00:00:00Z") {
		t.Errorf("expected output to contain date '2025-01-01T00:00:00Z', got: %s", got)
	}
	if got != "af 0.1.0 (commit: abc1234, built: 2025-01-01T00:00:00Z)" {
		t.Errorf("unexpected format for normal release, got: %s", got)
	}
}

func TestFormatVersion_PreRelease(t *testing.T) {
	got := FormatVersion("1.0.0-rc.1", "def5678", "2025-06-15T12:30:00Z")

	if !strings.Contains(got, "1.0.0-rc.1") {
		t.Errorf("expected output to contain version '1.0.0-rc.1', got: %s", got)
	}
	if !strings.Contains(got, "def5678") {
		t.Errorf("expected output to contain commit 'def5678', got: %s", got)
	}
	if !strings.Contains(got, "2025-06-15T12:30:00Z") {
		t.Errorf("expected output to contain date '2025-06-15T12:30:00Z', got: %s", got)
	}
	if got != "af 1.0.0-rc.1 (commit: def5678, built: 2025-06-15T12:30:00Z)" {
		t.Errorf("unexpected format for pre-release, got: %s", got)
	}
}
