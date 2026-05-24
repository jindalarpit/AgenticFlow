package main

import (
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// Feature: cli-auth-daemon, Property 15: Version info JSON round-trip
// For any version info struct, marshal to JSON and unmarshal back produces equivalent values.
// **Validates: Requirements 13.1, 13.2**
func TestProperty_VersionInfoJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random VersionInfo struct.
		info := VersionInfo{
			Version:   rapid.StringMatching(`[a-zA-Z0-9.\-]{1,30}`).Draw(t, "version"),
			Commit:    rapid.StringMatching(`[0-9a-f]{7,40}`).Draw(t, "commit"),
			BuildDate: rapid.StringMatching(`[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z`).Draw(t, "buildDate"),
			GoVersion: rapid.StringMatching(`go1\.[0-9]{1,2}\.[0-9]{1,2}`).Draw(t, "goVersion"),
			OS:        rapid.SampledFrom([]string{"linux", "darwin", "windows", "freebsd"}).Draw(t, "os"),
			Arch:      rapid.SampledFrom([]string{"amd64", "arm64", "386", "arm"}).Draw(t, "arch"),
		}

		// Marshal to JSON.
		data, err := json.Marshal(info)
		if err != nil {
			t.Fatalf("failed to marshal VersionInfo: %v", err)
		}

		// Unmarshal back into a new struct.
		var restored VersionInfo
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("failed to unmarshal VersionInfo: %v", err)
		}

		// Assert all fields are equivalent.
		if restored.Version != info.Version {
			t.Fatalf("Version mismatch: got %q, want %q", restored.Version, info.Version)
		}
		if restored.Commit != info.Commit {
			t.Fatalf("Commit mismatch: got %q, want %q", restored.Commit, info.Commit)
		}
		if restored.BuildDate != info.BuildDate {
			t.Fatalf("BuildDate mismatch: got %q, want %q", restored.BuildDate, info.BuildDate)
		}
		if restored.GoVersion != info.GoVersion {
			t.Fatalf("GoVersion mismatch: got %q, want %q", restored.GoVersion, info.GoVersion)
		}
		if restored.OS != info.OS {
			t.Fatalf("OS mismatch: got %q, want %q", restored.OS, info.OS)
		}
		if restored.Arch != info.Arch {
			t.Fatalf("Arch mismatch: got %q, want %q", restored.Arch, info.Arch)
		}
	})
}
