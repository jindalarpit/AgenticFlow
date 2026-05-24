package release

import "fmt"

// FormatVersion produces a human-readable version string containing the
// release version, short commit hash, and build date. This is the canonical
// format displayed by `af version` and `af --version`.
//
// Example output:
//
//	af 0.1.0 (commit: abc1234, built: 2025-01-01T00:00:00Z)
func FormatVersion(version, commit, date string) string {
	return fmt.Sprintf("af %s (commit: %s, built: %s)", version, commit, date)
}
