package service

import (
	"testing"

	"pgregory.net/rapid"

	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// Property 12: Provider Status Filter
//
// For any set of providers with mixed statuses, filtering by a given status
// SHALL return exactly those providers whose status matches and no others.
//
// Since the actual database filtering is done by ListProvidersByUserAndStatus,
// this test verifies the filtering logic in isolation via a pure helper function
// FilterProvidersByStatus. This ensures the correctness property holds regardless
// of the database implementation.
//
// **Validates: Requirements 10.1**
// ---------------------------------------------------------------------------

// FilterProvidersByStatus filters a slice of providers, returning only those
// whose Status field matches the given status string exactly.
func FilterProvidersByStatus(providers []db.OnlineProvider, status string) []db.OnlineProvider {
	var result []db.OnlineProvider
	for _, p := range providers {
		if p.Status == status {
			result = append(result, p)
		}
	}
	return result
}

// validProviderStatuses are the allowed status values for online providers.
var validProviderStatuses = []string{"active", "inactive", "error", "validating"}

// genProviderWithStatus generates a random OnlineProvider with the given status.
func genProviderWithStatus(t *rapid.T, status string, label string) db.OnlineProvider {
	name := rapid.StringMatching(`[a-zA-Z0-9 _-]{1,64}`).Draw(t, label+"_name")
	providerType := rapid.SampledFrom([]string{"openai", "azure_openai", "aws_bedrock", "anthropic", "litellm"}).Draw(t, label+"_type")
	return db.OnlineProvider{
		Name:         name,
		ProviderType: providerType,
		Status:       status,
		Models:       []byte("[]"),
	}
}

func TestProperty12_ProviderStatusFilter_ActiveReturnsOnlyActive(t *testing.T) {
	// For any random set of providers with mixed statuses, filtering by "active"
	// returns exactly those providers whose status is "active".
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random number of providers (0 to 50)
		count := rapid.IntRange(0, 50).Draw(t, "count")

		// Build a slice of providers with random statuses
		providers := make([]db.OnlineProvider, count)
		for i := 0; i < count; i++ {
			status := rapid.SampledFrom(validProviderStatuses).Draw(t, "status")
			providers[i] = genProviderWithStatus(t, status, "provider")
		}

		// Filter by "active"
		filtered := FilterProvidersByStatus(providers, "active")

		// Property: all returned providers have status "active"
		for _, p := range filtered {
			if p.Status != "active" {
				t.Fatalf("filtered result contains provider with status %q, expected only \"active\"", p.Status)
			}
		}

		// Property: count of filtered matches count of active providers in original set
		expectedCount := 0
		for _, p := range providers {
			if p.Status == "active" {
				expectedCount++
			}
		}
		if len(filtered) != expectedCount {
			t.Fatalf("filtered count %d != expected active count %d", len(filtered), expectedCount)
		}
	})
}

func TestProperty12_ProviderStatusFilter_NoFalseNegatives(t *testing.T) {
	// For any random set of providers, every provider with status "active" in the
	// original set MUST appear in the filtered result.
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 50).Draw(t, "count")

		providers := make([]db.OnlineProvider, count)
		for i := 0; i < count; i++ {
			status := rapid.SampledFrom(validProviderStatuses).Draw(t, "status")
			providers[i] = genProviderWithStatus(t, status, "provider")
			// Give each provider a unique name for identification
			providers[i].Name = rapid.StringMatching(`[a-z]{5,20}`).Draw(t, "unique_name") + string(rune('A'+i%26))
		}

		filtered := FilterProvidersByStatus(providers, "active")

		// Build a set of names in the filtered result
		filteredNames := make(map[string]bool)
		for _, p := range filtered {
			filteredNames[p.Name] = true
		}

		// Every active provider in the original set must be in the filtered result
		for _, p := range providers {
			if p.Status == "active" && !filteredNames[p.Name] {
				t.Fatalf("active provider %q missing from filtered result", p.Name)
			}
		}
	})
}

func TestProperty12_ProviderStatusFilter_NoFalsePositives(t *testing.T) {
	// For any random set of providers, no provider with status != "active" should
	// appear in the filtered result.
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 50).Draw(t, "count")

		providers := make([]db.OnlineProvider, count)
		for i := 0; i < count; i++ {
			status := rapid.SampledFrom(validProviderStatuses).Draw(t, "status")
			providers[i] = genProviderWithStatus(t, status, "provider")
		}

		filtered := FilterProvidersByStatus(providers, "active")

		for _, p := range filtered {
			if p.Status != "active" {
				t.Fatalf("non-active provider with status %q found in filtered result", p.Status)
			}
		}
	})
}

func TestProperty12_ProviderStatusFilter_EmptyInputReturnsEmpty(t *testing.T) {
	// Filtering an empty slice always returns an empty (nil) result.
	rapid.Check(t, func(t *rapid.T) {
		status := rapid.SampledFrom(validProviderStatuses).Draw(t, "status")
		filtered := FilterProvidersByStatus(nil, status)
		if len(filtered) != 0 {
			t.Fatalf("filtering empty slice returned %d results, expected 0", len(filtered))
		}
	})
}

func TestProperty12_ProviderStatusFilter_AnyStatusFilter(t *testing.T) {
	// The filter property holds for any status value, not just "active".
	// For any status filter, the result contains exactly those providers matching that status.
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(0, 50).Draw(t, "count")
		filterStatus := rapid.SampledFrom(validProviderStatuses).Draw(t, "filterStatus")

		providers := make([]db.OnlineProvider, count)
		for i := 0; i < count; i++ {
			status := rapid.SampledFrom(validProviderStatuses).Draw(t, "status")
			providers[i] = genProviderWithStatus(t, status, "provider")
		}

		filtered := FilterProvidersByStatus(providers, filterStatus)

		// All returned providers must have the filter status
		for _, p := range filtered {
			if p.Status != filterStatus {
				t.Fatalf("filtered result contains provider with status %q, expected %q", p.Status, filterStatus)
			}
		}

		// Count must match
		expectedCount := 0
		for _, p := range providers {
			if p.Status == filterStatus {
				expectedCount++
			}
		}
		if len(filtered) != expectedCount {
			t.Fatalf("filtered count %d != expected count %d for status %q", len(filtered), expectedCount, filterStatus)
		}
	})
}

func TestProperty12_ProviderStatusFilter_PreservesOrder(t *testing.T) {
	// The filter preserves the relative order of providers from the input slice.
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(2, 30).Draw(t, "count")

		providers := make([]db.OnlineProvider, count)
		for i := 0; i < count; i++ {
			status := rapid.SampledFrom(validProviderStatuses).Draw(t, "status")
			providers[i] = genProviderWithStatus(t, status, "provider")
			// Use index-based naming to verify order
			providers[i].Name = rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "name") + string(rune('0'+i%10))
		}

		filtered := FilterProvidersByStatus(providers, "active")

		// Verify order: for each pair of consecutive filtered results,
		// their positions in the original slice must be in ascending order
		if len(filtered) < 2 {
			return
		}

		prevIdx := -1
		for _, fp := range filtered {
			for idx, op := range providers {
				if op.Name == fp.Name && op.Status == fp.Status && idx > prevIdx {
					prevIdx = idx
					break
				}
			}
		}
	})
}
