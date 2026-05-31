package provider

import (
	"testing"
)

func TestNewRegistry_RegistersAllAdapters(t *testing.T) {
	r := NewRegistry()

	expectedTypes := []string{"openai", "azure_openai", "aws_bedrock", "anthropic", "litellm"}
	for _, pt := range expectedTypes {
		adapter, ok := r.Get(pt)
		if !ok {
			t.Errorf("expected adapter for %q to be registered", pt)
			continue
		}
		if adapter.ProviderType() != pt {
			t.Errorf("adapter.ProviderType() = %q, want %q", adapter.ProviderType(), pt)
		}
	}
}

func TestRegistry_Get_UnknownType(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("unknown_provider")
	if ok {
		t.Error("expected Get for unknown provider type to return false")
	}
}

func TestRegistry_Register_OverwritesExisting(t *testing.T) {
	r := NewRegistry()

	// Register a new adapter with the same type as an existing one
	r.Register(&OpenAIAdapter{})

	adapter, ok := r.Get("openai")
	if !ok {
		t.Fatal("expected openai adapter to exist after re-registration")
	}
	if adapter.ProviderType() != "openai" {
		t.Errorf("adapter.ProviderType() = %q, want %q", adapter.ProviderType(), "openai")
	}
}
