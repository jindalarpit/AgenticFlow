package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateCredentials_OpenAI_Valid(t *testing.T) {
	creds := json.RawMessage(`{"api_key": "sk-test-key-123"}`)
	if err := ValidateCredentials("openai", creds); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateCredentials_OpenAI_WithOptionalFields(t *testing.T) {
	creds := json.RawMessage(`{"api_key": "sk-test", "base_url": "https://api.example.com/v1", "organization": "org-123"}`)
	if err := ValidateCredentials("openai", creds); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateCredentials_OpenAI_MissingAPIKey(t *testing.T) {
	creds := json.RawMessage(`{"base_url": "https://api.example.com"}`)
	err := ValidateCredentials("openai", creds)
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
	if !strings.Contains(err.Error(), "api_key is required for openai providers") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateCredentials_OpenAI_EmptyAPIKey(t *testing.T) {
	creds := json.RawMessage(`{"api_key": ""}`)
	err := ValidateCredentials("openai", creds)
	if err == nil {
		t.Fatal("expected error for empty api_key")
	}
	if !strings.Contains(err.Error(), "api_key is required for openai providers") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateCredentials_OpenAI_APIKeyTooLong(t *testing.T) {
	longKey := strings.Repeat("a", 257)
	creds := json.RawMessage(`{"api_key": "` + longKey + `"}`)
	err := ValidateCredentials("openai", creds)
	if err == nil {
		t.Fatal("expected error for api_key exceeding max length")
	}
	if !strings.Contains(err.Error(), "api_key must be at most 256 characters") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateCredentials_OpenAI_InvalidBaseURL(t *testing.T) {
	creds := json.RawMessage(`{"api_key": "sk-test", "base_url": "http://not-https.com"}`)
	err := ValidateCredentials("openai", creds)
	if err == nil {
		t.Fatal("expected error for non-HTTPS base_url")
	}
	if !strings.Contains(err.Error(), "base_url must be a valid HTTPS URL") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateCredentials_AzureOpenAI_Valid(t *testing.T) {
	creds := json.RawMessage(`{"api_key": "key123", "endpoint": "https://myresource.openai.azure.com", "api_version": "2024-02-01", "deployment_name": "gpt-4"}`)
	if err := ValidateCredentials("azure_openai", creds); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateCredentials_AzureOpenAI_MissingEndpoint(t *testing.T) {
	creds := json.RawMessage(`{"api_key": "key123", "api_version": "2024-02-01", "deployment_name": "gpt-4"}`)
	err := ValidateCredentials("azure_openai", creds)
	if err == nil {
		t.Fatal("expected error for missing endpoint")
	}
	if !strings.Contains(err.Error(), "endpoint is required for azure_openai providers") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateCredentials_AWSBedrock_Valid(t *testing.T) {
	creds := json.RawMessage(`{"access_key_id": "AKIA123", "secret_access_key": "secret", "region": "us-east-1"}`)
	if err := ValidateCredentials("aws_bedrock", creds); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateCredentials_AWSBedrock_WithSessionToken(t *testing.T) {
	creds := json.RawMessage(`{"access_key_id": "AKIA123", "secret_access_key": "secret", "region": "us-east-1", "session_token": "token123"}`)
	if err := ValidateCredentials("aws_bedrock", creds); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateCredentials_AWSBedrock_MissingRegion(t *testing.T) {
	creds := json.RawMessage(`{"access_key_id": "AKIA123", "secret_access_key": "secret"}`)
	err := ValidateCredentials("aws_bedrock", creds)
	if err == nil {
		t.Fatal("expected error for missing region")
	}
	if !strings.Contains(err.Error(), "region is required for aws_bedrock providers") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateCredentials_Anthropic_Valid(t *testing.T) {
	creds := json.RawMessage(`{"api_key": "sk-ant-test"}`)
	if err := ValidateCredentials("anthropic", creds); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateCredentials_LiteLLM_Valid(t *testing.T) {
	creds := json.RawMessage(`{"api_key": "key123", "api_base": "https://my-litellm.example.com"}`)
	if err := ValidateCredentials("litellm", creds); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateCredentials_LiteLLM_MissingAPIBase(t *testing.T) {
	creds := json.RawMessage(`{"api_key": "key123"}`)
	err := ValidateCredentials("litellm", creds)
	if err == nil {
		t.Fatal("expected error for missing api_base")
	}
	if !strings.Contains(err.Error(), "api_base is required for litellm providers") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateCredentials_UnsupportedType(t *testing.T) {
	creds := json.RawMessage(`{"api_key": "test"}`)
	err := ValidateCredentials("unknown_provider", creds)
	if err == nil {
		t.Fatal("expected error for unsupported provider type")
	}
}

func TestValidateCredentials_InvalidJSON(t *testing.T) {
	creds := json.RawMessage(`not valid json`)
	err := ValidateCredentials("openai", creds)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidateProviderName_Valid(t *testing.T) {
	if err := ValidateProviderName("My Provider"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateProviderName_Empty(t *testing.T) {
	err := ValidateProviderName("")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "name must be between 1 and 128 characters") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateProviderName_TooLong(t *testing.T) {
	longName := strings.Repeat("a", 129)
	err := ValidateProviderName(longName)
	if err == nil {
		t.Fatal("expected error for name exceeding 128 chars")
	}
}

func TestValidateProviderName_ExactlyMaxLength(t *testing.T) {
	name := strings.Repeat("a", 128)
	if err := ValidateProviderName(name); err != nil {
		t.Fatalf("expected no error for 128-char name, got: %v", err)
	}
}

func TestValidateProviderType_Valid(t *testing.T) {
	for _, pt := range AllowedProviderTypes {
		if err := ValidateProviderType(pt); err != nil {
			t.Fatalf("expected no error for %q, got: %v", pt, err)
		}
	}
}

func TestValidateProviderType_Invalid(t *testing.T) {
	err := ValidateProviderType("google_vertex")
	if err == nil {
		t.Fatal("expected error for invalid provider type")
	}
	if !strings.Contains(err.Error(), "provider_type must be one of") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateModelIdentifier_Valid(t *testing.T) {
	validModels := []string{
		"gpt-4",
		"claude-sonnet-4-20250514",
		"anthropic.claude-3-sonnet-20240229-v1:0",
		"meta.llama3-70b-instruct-v1:0",
		"gpt-4o/2024-05-13",
		"user@model",
		"a",
	}
	for _, m := range validModels {
		if err := ValidateModelIdentifier(m); err != nil {
			t.Fatalf("expected no error for %q, got: %v", m, err)
		}
	}
}

func TestValidateModelIdentifier_Invalid(t *testing.T) {
	invalidModels := []string{
		"",                         // too short
		strings.Repeat("a", 129),  // too long
		"-starts-with-hyphen",     // must start with alphanumeric
		".starts-with-dot",        // must start with alphanumeric
		"has spaces in it",        // spaces not allowed
		"has$pecial",              // $ not allowed
	}
	for _, m := range invalidModels {
		if err := ValidateModelIdentifier(m); err == nil {
			t.Fatalf("expected error for %q, got nil", m)
		}
	}
}

func TestValidateModelIdentifier_ExactlyMaxLength(t *testing.T) {
	model := "a" + strings.Repeat("b", 127) // 128 chars total
	if err := ValidateModelIdentifier(model); err != nil {
		t.Fatalf("expected no error for 128-char model, got: %v", err)
	}
}
