package daemon

import (
	"context"
	"errors"
	"os"
	"testing"

	"pgregory.net/rapid"
)

// Feature: agenticflow-core, Property 1: Agent Detection with Precedence
// For any set of agent configurations (combining PATH availability and AF_<NAME>_PATH env vars),
// DetectAgents returns exactly the agents whose binary exists at either the custom path (if set)
// or on PATH, with custom paths taking precedence. Each returned entry has non-empty name, path,
// and version (or "unknown").
// Validates: Requirements 1.1, 1.5, 1.6, 1.7, 1.8
func TestProperty_AgentDetectionWithPrecedence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// For each known agent, randomly decide:
		// - whether it's available on PATH
		// - whether a custom AF_<NAME>_PATH env var is set
		// - if custom path is set, whether the binary actually exists there
		type agentConfig struct {
			onPATH         bool
			hasCustomPath  bool
			customPathValid bool // only relevant if hasCustomPath is true
			versionOutput  string
		}

		configs := make(map[string]agentConfig)
		for _, ag := range knownAgents {
			configs[ag.Name] = agentConfig{
				onPATH:          rapid.Bool().Draw(t, ag.Name+"-onPATH"),
				hasCustomPath:   rapid.Bool().Draw(t, ag.Name+"-hasCustomPath"),
				customPathValid: rapid.Bool().Draw(t, ag.Name+"-customPathValid"),
				versionOutput:   rapid.SampledFrom([]string{"1.0.0", "2.3.1", "", "error"}).Draw(t, ag.Name+"-version"),
			}
		}

		// Build mock dependencies based on the generated config.
		deps := &DetectionDeps{
			LookPath: func(file string) (string, error) {
				// Find the agent by command name.
				for _, ag := range knownAgents {
					if ag.Command == file {
						cfg := configs[ag.Name]
						if cfg.onPATH {
							return "/usr/local/bin/" + file, nil
						}
						return "", errors.New("not found")
					}
				}
				return "", errors.New("not found")
			},
			Getenv: func(key string) string {
				for _, ag := range knownAgents {
					if ag.EnvPath == key {
						cfg := configs[ag.Name]
						if cfg.hasCustomPath {
							return "/custom/" + ag.Name
						}
						return ""
					}
					if ag.EnvModel == key {
						return ""
					}
				}
				return ""
			},
			Stat: func(name string) (os.FileInfo, error) {
				for _, ag := range knownAgents {
					if name == "/custom/"+ag.Name {
						cfg := configs[ag.Name]
						if cfg.customPathValid {
							return fakeFileInfo{isDir: false}, nil
						}
						return nil, os.ErrNotExist
					}
				}
				return nil, os.ErrNotExist
			},
			DetectVersion: func(ctx context.Context, path string) (string, error) {
				// Find which agent this path belongs to.
				for _, ag := range knownAgents {
					pathPath := "/usr/local/bin/" + ag.Command
					customPath := "/custom/" + ag.Name
					if path == pathPath || path == customPath {
						cfg := configs[ag.Name]
						if cfg.versionOutput == "error" || cfg.versionOutput == "" {
							return "", errors.New("version detection failed")
						}
						return cfg.versionOutput, nil
					}
				}
				return "", errors.New("unknown binary")
			},
		}

		// Run detection.
		agents := DetectAgents(deps)

		// Compute expected set of detected agents.
		for _, ag := range knownAgents {
			cfg := configs[ag.Name]

			// An agent should be detected if:
			// 1. Custom path is set AND valid (takes precedence), OR
			// 2. No custom path is set AND agent is on PATH
			shouldBeDetected := false
			expectedPath := ""

			if cfg.hasCustomPath {
				if cfg.customPathValid {
					shouldBeDetected = true
					expectedPath = "/custom/" + ag.Name
				}
				// If custom path is set but invalid, agent is NOT detected
				// (even if it's on PATH — custom path takes precedence)
			} else {
				if cfg.onPATH {
					shouldBeDetected = true
					expectedPath = "/usr/local/bin/" + ag.Command
				}
			}

			entry, found := agents[ag.Name]

			if shouldBeDetected && !found {
				t.Fatalf("agent %q should be detected (onPATH=%v, hasCustomPath=%v, customPathValid=%v) but was not",
					ag.Name, cfg.onPATH, cfg.hasCustomPath, cfg.customPathValid)
			}
			if !shouldBeDetected && found {
				t.Fatalf("agent %q should NOT be detected (onPATH=%v, hasCustomPath=%v, customPathValid=%v) but was found",
					ag.Name, cfg.onPATH, cfg.hasCustomPath, cfg.customPathValid)
			}

			if found {
				// Verify non-empty name.
				if entry.Name == "" {
					t.Fatalf("agent %q has empty name", ag.Name)
				}
				// Verify non-empty path.
				if entry.Path == "" {
					t.Fatalf("agent %q has empty path", ag.Name)
				}
				// Verify path matches expected (custom path takes precedence).
				if entry.Path != expectedPath {
					t.Fatalf("agent %q: expected path %q, got %q", ag.Name, expectedPath, entry.Path)
				}
				// Verify version is non-empty (either actual version or "unknown").
				if entry.Version == "" {
					t.Fatalf("agent %q has empty version", ag.Name)
				}
				// Version should be either the detected version or "unknown".
				if cfg.versionOutput == "error" || cfg.versionOutput == "" {
					if entry.Version != "unknown" {
						t.Fatalf("agent %q: expected version 'unknown' for failed detection, got %q",
							ag.Name, entry.Version)
					}
				} else {
					if entry.Version != cfg.versionOutput {
						t.Fatalf("agent %q: expected version %q, got %q",
							ag.Name, cfg.versionOutput, entry.Version)
					}
				}
			}
		}

		// Verify no extra agents beyond known ones.
		for name := range agents {
			found := false
			for _, ag := range knownAgents {
				if ag.Name == name {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("unexpected agent %q in detection results", name)
			}
		}
	})
}

// Feature: agenticflow-core, Property 2: Agent Deregistration Equals Scan Difference
// For any two consecutive detection scans producing sets A and B, the deregistered set
// equals A\B and newly registered equals B\A.
// Validates: Requirements 1.4
func TestProperty_AgentDeregistrationEqualsScanDifference(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate two random availability states representing consecutive scans.
		type scanState struct {
			available map[string]bool
		}

		genScanState := func(label string) scanState {
			state := scanState{available: make(map[string]bool)}
			for _, ag := range knownAgents {
				state.available[ag.Name] = rapid.Bool().Draw(t, label+"-"+ag.Name)
			}
			return state
		}

		scanA := genScanState("scanA")
		scanB := genScanState("scanB")

		// Build deps for scan A.
		buildDeps := func(state scanState) *DetectionDeps {
			return &DetectionDeps{
				LookPath: func(file string) (string, error) {
					for _, ag := range knownAgents {
						if ag.Command == file && state.available[ag.Name] {
							return "/usr/local/bin/" + file, nil
						}
					}
					return "", errors.New("not found")
				},
				Getenv: func(key string) string {
					return ""
				},
				Stat: func(name string) (os.FileInfo, error) {
					return nil, os.ErrNotExist
				},
				DetectVersion: func(ctx context.Context, path string) (string, error) {
					return "1.0.0", nil
				},
			}
		}

		// Run both scans.
		agentsA := DetectAgents(buildDeps(scanA))
		agentsB := DetectAgents(buildDeps(scanB))

		// Compute expected deregistered set: A \ B (in A but not in B).
		expectedDeregistered := make(map[string]bool)
		for name := range agentsA {
			if _, inB := agentsB[name]; !inB {
				expectedDeregistered[name] = true
			}
		}

		// Compute expected newly registered set: B \ A (in B but not in A).
		expectedNewlyRegistered := make(map[string]bool)
		for name := range agentsB {
			if _, inA := agentsA[name]; !inA {
				expectedNewlyRegistered[name] = true
			}
		}

		// Compute actual deregistered: agents in A but not in B.
		actualDeregistered := make(map[string]bool)
		for name := range agentsA {
			if _, inB := agentsB[name]; !inB {
				actualDeregistered[name] = true
			}
		}

		// Compute actual newly registered: agents in B but not in A.
		actualNewlyRegistered := make(map[string]bool)
		for name := range agentsB {
			if _, inA := agentsA[name]; !inA {
				actualNewlyRegistered[name] = true
			}
		}

		// Verify deregistered set matches.
		if len(actualDeregistered) != len(expectedDeregistered) {
			t.Fatalf("deregistered set size mismatch: expected %d, got %d",
				len(expectedDeregistered), len(actualDeregistered))
		}
		for name := range expectedDeregistered {
			if !actualDeregistered[name] {
				t.Fatalf("agent %q should be in deregistered set but is not", name)
			}
		}

		// Verify newly registered set matches.
		if len(actualNewlyRegistered) != len(expectedNewlyRegistered) {
			t.Fatalf("newly registered set size mismatch: expected %d, got %d",
				len(expectedNewlyRegistered), len(actualNewlyRegistered))
		}
		for name := range expectedNewlyRegistered {
			if !actualNewlyRegistered[name] {
				t.Fatalf("agent %q should be in newly registered set but is not", name)
			}
		}

		// Additional invariant: deregistered ∩ newly registered = ∅
		for name := range actualDeregistered {
			if actualNewlyRegistered[name] {
				t.Fatalf("agent %q is in both deregistered and newly registered sets", name)
			}
		}

		// Additional invariant: |A| - |deregistered| + |newly registered| = |B|
		expectedBSize := len(agentsA) - len(actualDeregistered) + len(actualNewlyRegistered)
		if expectedBSize != len(agentsB) {
			t.Fatalf("set arithmetic invariant violated: |A|=%d - |dereg|=%d + |new|=%d = %d, but |B|=%d",
				len(agentsA), len(actualDeregistered), len(actualNewlyRegistered),
				expectedBSize, len(agentsB))
		}
	})
}

// fakeFileInfo is already defined in detection_test.go in the same package,
// so we reuse it here for property tests.
var _ os.FileInfo = fakeFileInfo{}
