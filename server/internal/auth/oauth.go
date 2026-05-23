package auth

import "net/http"

// OAuthConfig holds the configuration for an OAuth provider.
type OAuthConfig struct {
	// Provider is the OAuth provider name (e.g., "github", "google").
	Provider string

	// ClientID is the OAuth application client ID.
	ClientID string

	// ClientSecret is the OAuth application client secret.
	ClientSecret string

	// RedirectURL is the callback URL registered with the provider.
	RedirectURL string
}

// OAuthHandler handles OAuth authentication flows.
type OAuthHandler struct {
	Config OAuthConfig
}

// NewOAuthHandler creates a new OAuthHandler with the given configuration.
// This is a placeholder — full OAuth flow will be implemented when provider
// integrations (GitHub, Google) are added.
func NewOAuthHandler(cfg OAuthConfig) *OAuthHandler {
	return &OAuthHandler{Config: cfg}
}

// HandleCallback processes the OAuth callback from the provider.
// This is a placeholder that returns 501 Not Implemented.
func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "OAuth callback not implemented for provider: "+h.Config.Provider, http.StatusNotImplemented)
}
