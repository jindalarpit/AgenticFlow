package provider

// Registry maps provider type strings to their adapter implementations.
type Registry struct {
	adapters map[string]ProviderAdapter
}

// NewRegistry creates a new Registry with all supported provider adapters registered.
func NewRegistry() *Registry {
	r := &Registry{adapters: make(map[string]ProviderAdapter)}
	r.Register(&OpenAIAdapter{})
	r.Register(&AzureOpenAIAdapter{})
	r.Register(&BedrockAdapter{})
	r.Register(&AnthropicAdapter{})
	r.Register(&LiteLLMAdapter{})
	return r
}

// Get returns the adapter for the given provider type, or false if not found.
func (r *Registry) Get(providerType string) (ProviderAdapter, bool) {
	a, ok := r.adapters[providerType]
	return a, ok
}

// Register adds an adapter to the registry, keyed by its ProviderType().
func (r *Registry) Register(a ProviderAdapter) {
	r.adapters[a.ProviderType()] = a
}
