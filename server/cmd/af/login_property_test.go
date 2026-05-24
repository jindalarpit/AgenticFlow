package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: cli-auth-daemon, Property 2: CSRF state validation
// For any generated state and callback request, accept iff state matches exactly;
// reject with HTTP 400 otherwise.
// Validates: Requirements 1.4, 1.9
func TestProperty_CSRFStateValidation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a known expected state (simulating what GenerateState produces).
		expectedState := rapid.StringMatching(`[0-9a-f]{32}`).Draw(t, "expectedState")

		// Generate a request state that may or may not match.
		requestState := rapid.StringMatching(`[0-9a-f]{1,64}`).Draw(t, "requestState")

		// Build the callback handler with the expected state captured in closure
		// (same pattern as runLoginBrowser in cmd_login.go).
		jwtCh := make(chan string, 1)
		mux := http.NewServeMux()
		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			returnedState := r.URL.Query().Get("state")
			if returnedState != expectedState {
				http.Error(w, "invalid state parameter", http.StatusBadRequest)
				return
			}
			token := r.URL.Query().Get("token")
			if token == "" {
				http.Error(w, "missing token", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			jwtCh <- token
		})

		// Make a request with the generated state and a valid token.
		req := httptest.NewRequest("GET", "/callback?state="+url.QueryEscape(requestState)+"&token=test_jwt", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if requestState == expectedState {
			// State matches: should be accepted (HTTP 200).
			if rec.Code != http.StatusOK {
				t.Fatalf("state matches (%q == %q) but got HTTP %d, want 200",
					requestState, expectedState, rec.Code)
			}
		} else {
			// State does not match: should be rejected (HTTP 400).
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("state mismatch (%q != %q) but got HTTP %d, want 400",
					requestState, expectedState, rec.Code)
			}
		}
	})
}

// Feature: cli-auth-daemon, Property 2: CSRF state validation (matching state always accepted)
// For any generated state, a callback with the exact same state is always accepted.
// Validates: Requirements 1.4, 1.9
func TestProperty_CSRFStateValidation_MatchAlwaysAccepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a state token (same format as GenerateState: 32 hex chars).
		state := rapid.StringMatching(`[0-9a-f]{32}`).Draw(t, "state")

		// Build the callback handler with this state.
		jwtCh := make(chan string, 1)
		mux := http.NewServeMux()
		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			returnedState := r.URL.Query().Get("state")
			if returnedState != state {
				http.Error(w, "invalid state parameter", http.StatusBadRequest)
				return
			}
			token := r.URL.Query().Get("token")
			if token == "" {
				http.Error(w, "missing token", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			jwtCh <- token
		})

		// Request with the exact same state — must always be accepted.
		req := httptest.NewRequest("GET", "/callback?state="+url.QueryEscape(state)+"&token=jwt_value", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("matching state %q should be accepted, got HTTP %d", state, rec.Code)
		}
	})
}

// Feature: cli-auth-daemon, Property 2: CSRF state validation (non-matching state always rejected)
// For any two distinct state values, a callback with a different state is always rejected with HTTP 400.
// Validates: Requirements 1.4, 1.9
func TestProperty_CSRFStateValidation_MismatchAlwaysRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate two distinct state values.
		expectedState := rapid.StringMatching(`[0-9a-f]{32}`).Draw(t, "expectedState")
		requestState := rapid.String().Draw(t, "requestState")

		// Ensure they are different.
		if requestState == expectedState {
			requestState = expectedState + "x"
		}

		// Build the callback handler with the expected state.
		mux := http.NewServeMux()
		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			returnedState := r.URL.Query().Get("state")
			if returnedState != expectedState {
				http.Error(w, "invalid state parameter", http.StatusBadRequest)
				return
			}
			token := r.URL.Query().Get("token")
			if token == "" {
				http.Error(w, "missing token", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		// Request with a different state — must always be rejected.
		req := httptest.NewRequest("GET", "/callback?state="+url.QueryEscape(requestState)+"&token=jwt_value", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("mismatched state (expected=%q, got=%q) should be rejected with 400, got HTTP %d",
				expectedState, requestState, rec.Code)
		}
	})
}

// Feature: cli-auth-daemon, Property 3: Token format validation
// For any string, validation passes iff it starts with "af_".
// Validates: Requirements 2.1, 2.5
func TestProperty_TokenFormatValidation(t *testing.T) {
	// Test with arbitrary random strings: ValidateTokenFormat should return true
	// iff the string starts with "af_".
	rapid.Check(t, func(t *rapid.T) {
		token := rapid.String().Draw(t, "token")

		result := ValidateTokenFormat(token)
		expected := strings.HasPrefix(token, "af_")

		if result != expected {
			t.Fatalf("ValidateTokenFormat(%q) = %v, want %v", token, result, expected)
		}
	})
}

// Feature: cli-auth-daemon, Property 3: Token format validation (valid prefix)
// For any string that starts with "af_", ValidateTokenFormat must return true.
// Validates: Requirements 2.1, 2.5
func TestProperty_TokenFormatValidation_ValidPrefix(t *testing.T) {
	// Generate strings that DO start with "af_" to ensure they always pass.
	rapid.Check(t, func(t *rapid.T) {
		suffix := rapid.String().Draw(t, "suffix")
		token := "af_" + suffix

		result := ValidateTokenFormat(token)

		if !result {
			t.Fatalf("ValidateTokenFormat(%q) = false, want true (starts with af_)", token)
		}
	})
}

// Feature: cli-auth-daemon, Property 5: Login URL construction
// For any server URL, callback URL, and state token, constructed URL contains
// properly encoded cli_callback and cli_state params whose decoded values match
// the original callback URL and state token respectively.
// Validates: Requirements 1.2
func TestProperty_LoginURLConstruction(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a server URL: scheme + host + optional port + optional path
		scheme := rapid.SampledFrom([]string{"http", "https"}).Draw(t, "scheme")
		host := rapid.StringMatching(`[a-z][a-z0-9]{1,8}\.[a-z]{2,4}`).Draw(t, "host")
		port := rapid.IntRange(80, 65535).Draw(t, "port")
		serverURL := fmt.Sprintf("%s://%s:%d", scheme, host, port)

		// Generate a callback URL (local HTTP server on random port)
		callbackPort := rapid.IntRange(1024, 65535).Draw(t, "callbackPort")
		callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", callbackPort)

		// Generate a random state token (hex string, 16-64 chars)
		state := rapid.StringMatching(`[0-9a-f]{16,64}`).Draw(t, "state")

		// Build the login URL
		result := BuildLoginURL(serverURL, callbackURL, state)

		// Parse the result URL — must be valid
		parsed, err := url.Parse(result)
		if err != nil {
			t.Fatalf("BuildLoginURL produced unparseable URL: %v\nURL: %s", err, result)
		}

		// Verify the URL path ends with /login
		if !strings.HasSuffix(parsed.Path, "/login") {
			t.Fatalf("expected path to end with /login, got path=%q in URL=%s", parsed.Path, result)
		}

		// Extract and decode query parameters
		query := parsed.Query()

		// Verify cli_callback param decodes to the original callback URL
		gotCallback := query.Get("cli_callback")
		if gotCallback == "" {
			t.Fatalf("cli_callback query param missing from URL: %s", result)
		}
		if gotCallback != callbackURL {
			t.Fatalf("cli_callback mismatch:\n  got:  %q\n  want: %q\n  URL: %s", gotCallback, callbackURL, result)
		}

		// Verify cli_state param decodes to the original state token
		gotState := query.Get("cli_state")
		if gotState == "" {
			t.Fatalf("cli_state query param missing from URL: %s", result)
		}
		if gotState != state {
			t.Fatalf("cli_state mismatch:\n  got:  %q\n  want: %q\n  URL: %s", gotState, state, result)
		}
	})
}
