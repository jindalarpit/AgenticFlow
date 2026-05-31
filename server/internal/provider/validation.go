package provider

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// AllowedProviderTypes is the set of valid provider type strings.
var AllowedProviderTypes = []string{
	"openai",
	"azure_openai",
	"aws_bedrock",
	"anthropic",
	"litellm",
}

// modelIdentifierPattern validates model identifier strings.
var modelIdentifierPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_./:@-]*$`)

// credentialField defines a single field in a provider's credential schema.
type credentialField struct {
	Name     string
	Required bool
	MaxLen   int
	IsURL    bool // if true, must be a valid HTTPS URL
}

// credentialSchemas maps provider types to their credential field definitions.
var credentialSchemas = map[string][]credentialField{
	"openai": {
		{Name: "api_key", Required: true, MaxLen: 256, IsURL: false},
		{Name: "base_url", Required: false, MaxLen: 2048, IsURL: true},
		{Name: "organization", Required: false, MaxLen: 128, IsURL: false},
	},
	"azure_openai": {
		{Name: "api_key", Required: true, MaxLen: 256, IsURL: false},
		{Name: "endpoint", Required: true, MaxLen: 2048, IsURL: true},
		{Name: "api_version", Required: true, MaxLen: 32, IsURL: false},
		{Name: "deployment_name", Required: true, MaxLen: 128, IsURL: false},
	},
	"aws_bedrock": {
		{Name: "access_key_id", Required: true, MaxLen: 128, IsURL: false},
		{Name: "secret_access_key", Required: true, MaxLen: 256, IsURL: false},
		{Name: "region", Required: true, MaxLen: 32, IsURL: false},
		{Name: "session_token", Required: false, MaxLen: 2048, IsURL: false},
	},
	"anthropic": {
		{Name: "api_key", Required: true, MaxLen: 256, IsURL: false},
		{Name: "base_url", Required: false, MaxLen: 2048, IsURL: true},
	},
	"litellm": {
		{Name: "api_key", Required: true, MaxLen: 256, IsURL: false},
		{Name: "api_base", Required: true, MaxLen: 2048, IsURL: true},
		{Name: "custom_llm_provider", Required: false, MaxLen: 64, IsURL: false},
	},
}

// ValidateCredentials validates the credential JSON for a given provider type.
// It checks required fields, non-empty constraints, length limits, and HTTPS URL format.
func ValidateCredentials(providerType string, creds json.RawMessage) error {
	schema, ok := credentialSchemas[providerType]
	if !ok {
		return fmt.Errorf("unsupported provider_type: %s", providerType)
	}

	var fields map[string]string
	if err := json.Unmarshal(creds, &fields); err != nil {
		return fmt.Errorf("credentials must be a JSON object with string values")
	}

	for _, field := range schema {
		value, exists := fields[field.Name]

		if field.Required {
			if !exists || value == "" {
				return fmt.Errorf("%s is required for %s providers", field.Name, providerType)
			}
		}

		// Only validate further if the field is present and non-empty
		if !exists || value == "" {
			continue
		}

		if len(value) > field.MaxLen {
			return fmt.Errorf("%s must be at most %d characters", field.Name, field.MaxLen)
		}

		if field.IsURL {
			if err := validateHTTPSURL(value, field.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateHTTPSURL checks that a string is a valid HTTPS URL.
func validateHTTPSURL(value, fieldName string) error {
	if !strings.HasPrefix(value, "https://") {
		return fmt.Errorf("%s must be a valid HTTPS URL", fieldName)
	}
	if _, err := url.Parse(value); err != nil {
		return fmt.Errorf("%s must be a valid HTTPS URL", fieldName)
	}
	return nil
}

// ValidateProviderName validates a provider name (1-128 characters).
func ValidateProviderName(name string) error {
	if len(name) < 1 || len(name) > 128 {
		return fmt.Errorf("name must be between 1 and 128 characters")
	}
	return nil
}

// ValidateProviderType validates that the provider type is one of the allowed values.
func ValidateProviderType(providerType string) error {
	for _, allowed := range AllowedProviderTypes {
		if providerType == allowed {
			return nil
		}
	}
	return fmt.Errorf("provider_type must be one of: openai, azure_openai, aws_bedrock, anthropic, litellm")
}

// ValidateModelIdentifier validates a model identifier string.
// It must match the pattern ^[a-zA-Z0-9][a-zA-Z0-9_./:@-]*$ and be 1-128 characters.
func ValidateModelIdentifier(model string) error {
	if len(model) < 1 || len(model) > 128 {
		return fmt.Errorf("model must be between 1 and 128 characters")
	}
	if !modelIdentifierPattern.MatchString(model) {
		return fmt.Errorf("model must match pattern: starts with alphanumeric, followed by alphanumeric, dots, underscores, slashes, colons, at-signs, or hyphens")
	}
	return nil
}
