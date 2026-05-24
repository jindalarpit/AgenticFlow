package release

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// **Validates: Requirements 4.2, 4.3**
//
// Property 2: Tag validation accepts valid semver and rejects invalid tags
//
// For any string, the tag validation logic SHALL accept it if and only if it
// matches the pattern vX.Y.Z or vX.Y.Z-suffix (where X, Y, Z are non-negative
// integers and suffix is a valid pre-release identifier) AND does not contain
// the substring "dirty".

// TestProperty2_ValidTagsAccepted verifies that any well-formed semver tag
// (vX.Y.Z or vX.Y.Z-suffix without "dirty") is accepted by ValidateTag.
func TestProperty2_ValidTagsAccepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		major := rapid.IntRange(0, 999).Draw(t, "major")
		minor := rapid.IntRange(0, 999).Draw(t, "minor")
		patch := rapid.IntRange(0, 999).Draw(t, "patch")

		tag := fmt.Sprintf("v%d.%d.%d", major, minor, patch)

		version, isStable, err := ValidateTag(tag)
		if err != nil {
			t.Fatalf("expected valid tag %q to be accepted, got error: %v", tag, err)
		}
		expectedVersion := fmt.Sprintf("%d.%d.%d", major, minor, patch)
		if version != expectedVersion {
			t.Fatalf("expected version %q, got %q", expectedVersion, version)
		}
		if !isStable {
			t.Fatalf("expected tag %q to be stable", tag)
		}
	})
}

// prereleaseSuffixGen generates valid pre-release suffixes (alphanumeric, dots, hyphens)
// that do NOT contain the substring "dirty".
func prereleaseSuffixGen() *rapid.Generator[string] {
	// Generate 1-3 dot-separated identifiers, each alphanumeric 1-8 chars
	return rapid.Custom(func(t *rapid.T) string {
		numParts := rapid.IntRange(1, 3).Draw(t, "numParts")
		parts := make([]string, numParts)
		for i := range parts {
			parts[i] = rapid.StringMatching(`[0-9A-Za-z]{1,8}`).Draw(t, fmt.Sprintf("part%d", i))
		}
		suffix := strings.Join(parts, ".")
		// Ensure no "dirty" substring by replacing if present
		suffix = strings.ReplaceAll(suffix, "dirty", "clean")
		suffix = strings.ReplaceAll(suffix, "Dirty", "Clean")
		return suffix
	})
}

// TestProperty2_ValidPreReleaseTagsAccepted verifies that valid tags with
// pre-release suffixes are accepted and marked as unstable.
func TestProperty2_ValidPreReleaseTagsAccepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		major := rapid.IntRange(0, 999).Draw(t, "major")
		minor := rapid.IntRange(0, 999).Draw(t, "minor")
		patch := rapid.IntRange(0, 999).Draw(t, "patch")
		suffix := prereleaseSuffixGen().Draw(t, "suffix")

		tag := fmt.Sprintf("v%d.%d.%d-%s", major, minor, patch, suffix)

		version, isStable, err := ValidateTag(tag)
		if err != nil {
			t.Fatalf("expected valid pre-release tag %q to be accepted, got error: %v", tag, err)
		}
		expectedVersion := fmt.Sprintf("%d.%d.%d-%s", major, minor, patch, suffix)
		if version != expectedVersion {
			t.Fatalf("expected version %q, got %q", expectedVersion, version)
		}
		if isStable {
			t.Fatalf("expected pre-release tag %q to be unstable", tag)
		}
	})
}

// TestProperty2_InvalidTagsMissingPrefix verifies that tags without the "v" prefix
// are rejected.
func TestProperty2_InvalidTagsMissingPrefix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		major := rapid.IntRange(0, 999).Draw(t, "major")
		minor := rapid.IntRange(0, 999).Draw(t, "minor")
		patch := rapid.IntRange(0, 999).Draw(t, "patch")

		// Valid version numbers but missing "v" prefix
		tag := fmt.Sprintf("%d.%d.%d", major, minor, patch)

		_, _, err := ValidateTag(tag)
		if err == nil {
			t.Fatalf("expected tag %q without 'v' prefix to be rejected", tag)
		}
	})
}

// TestProperty2_InvalidTagsNonNumericParts verifies that tags with non-numeric
// version components are rejected.
func TestProperty2_InvalidTagsNonNumericParts(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a string that contains at least one non-digit in a version position
		nonNumeric := rapid.StringMatching(`[a-zA-Z]{1,5}`).Draw(t, "nonNumeric")
		minor := rapid.IntRange(0, 999).Draw(t, "minor")
		patch := rapid.IntRange(0, 999).Draw(t, "patch")

		tag := fmt.Sprintf("v%s.%d.%d", nonNumeric, minor, patch)

		_, _, err := ValidateTag(tag)
		if err == nil {
			t.Fatalf("expected tag %q with non-numeric major to be rejected", tag)
		}
	})
}

// TestProperty2_DirtyTagsRejected verifies that any tag containing "dirty"
// is rejected, even if it otherwise matches valid semver format.
func TestProperty2_DirtyTagsRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		major := rapid.IntRange(0, 999).Draw(t, "major")
		minor := rapid.IntRange(0, 999).Draw(t, "minor")
		patch := rapid.IntRange(0, 999).Draw(t, "patch")

		// Generate a suffix that contains "dirty"
		prefix := rapid.StringMatching(`[0-9A-Za-z]{0,4}`).Draw(t, "prefix")
		postfix := rapid.StringMatching(`[0-9A-Za-z]{0,4}`).Draw(t, "postfix")
		suffix := prefix + "dirty" + postfix

		tag := fmt.Sprintf("v%d.%d.%d-%s", major, minor, patch, suffix)

		_, _, err := ValidateTag(tag)
		if err == nil {
			t.Fatalf("expected dirty tag %q to be rejected", tag)
		}
	})
}

// TestProperty2_RandomStringsRejected verifies that arbitrary random strings
// that don't match semver format are rejected.
func TestProperty2_RandomStringsRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random strings that are unlikely to match vX.Y.Z pattern
		s := rapid.OneOf(
			// Empty string
			rapid.Just(""),
			// Random alphanumeric without structure
			rapid.StringMatching(`[a-z]{3,20}`),
			// Missing dots
			rapid.StringMatching(`v[0-9]+`),
			// Only two version parts
			rapid.Custom(func(t *rapid.T) string {
				a := rapid.IntRange(0, 99).Draw(t, "a")
				b := rapid.IntRange(0, 99).Draw(t, "b")
				return fmt.Sprintf("v%d.%d", a, b)
			}),
			// Four version parts (too many)
			rapid.Custom(func(t *rapid.T) string {
				a := rapid.IntRange(0, 99).Draw(t, "a")
				b := rapid.IntRange(0, 99).Draw(t, "b")
				c := rapid.IntRange(0, 99).Draw(t, "c")
				d := rapid.IntRange(0, 99).Draw(t, "d")
				return fmt.Sprintf("v%d.%d.%d.%d", a, b, c, d)
			}),
		).Draw(t, "invalidTag")

		_, _, err := ValidateTag(s)
		if err == nil {
			t.Fatalf("expected random string %q to be rejected", s)
		}
	})
}
