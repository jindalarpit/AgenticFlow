package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAPIClient(t *testing.T) {
	client := NewAPIClient("http://localhost:8080/", "af_testtoken123")

	if client.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL: got %q, want %q", client.BaseURL, "http://localhost:8080")
	}
	if client.Token != "af_testtoken123" {
		t.Errorf("Token: got %q, want %q", client.Token, "af_testtoken123")
	}
	if client.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
}

func TestGetJSON_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header is set.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer af_testtoken" {
			t.Errorf("Authorization header: got %q, want %q", auth, "Bearer af_testtoken")
		}
		if r.Method != http.MethodGet {
			t.Errorf("Method: got %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/me" {
			t.Errorf("Path: got %q, want /api/me", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"name":  "Test User",
			"email": "test@example.com",
		})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "af_testtoken")

	var me struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	err := client.GetJSON(context.Background(), "/api/me", &me)
	if err != nil {
		t.Fatalf("GetJSON: unexpected error: %v", err)
	}
	if me.Name != "Test User" {
		t.Errorf("Name: got %q, want %q", me.Name, "Test User")
	}
	if me.Email != "test@example.com" {
		t.Errorf("Email: got %q, want %q", me.Email, "test@example.com")
	}
}

func TestGetJSON_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "af_badtoken")

	var me struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	err := client.GetJSON(context.Background(), "/api/me", &me)
	if err == nil {
		t.Fatal("GetJSON: expected error for 401 response")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T: %v", err, err)
	}
	if !httpErr.IsUnauthorized() {
		t.Errorf("expected IsUnauthorized() = true, got false (status: %d)", httpErr.StatusCode)
	}
	if httpErr.StatusCode != 401 {
		t.Errorf("StatusCode: got %d, want 401", httpErr.StatusCode)
	}
}

func TestGetJSON_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "af_token")

	err := client.GetJSON(context.Background(), "/api/me", nil)
	if err == nil {
		t.Fatal("GetJSON: expected error for 500 response")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != 500 {
		t.Errorf("StatusCode: got %d, want 500", httpErr.StatusCode)
	}
}

func TestGetJSON_NoToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" {
			t.Errorf("Authorization header should be empty, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "")
	err := client.GetJSON(context.Background(), "/api/me", nil)
	if err != nil {
		t.Fatalf("GetJSON: unexpected error: %v", err)
	}
}

func TestPostJSON_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method: got %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type: got %q, want application/json", ct)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "test-token" {
			t.Errorf("body.name: got %q, want %q", body["name"], "test-token")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "tok_123"})
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "af_token")

	var resp struct {
		ID string `json:"id"`
	}
	err := client.PostJSON(context.Background(), "/api/tokens", map[string]string{"name": "test-token"}, &resp)
	if err != nil {
		t.Fatalf("PostJSON: unexpected error: %v", err)
	}
	if resp.ID != "tok_123" {
		t.Errorf("ID: got %q, want %q", resp.ID, "tok_123")
	}
}

func TestHTTPError_Error(t *testing.T) {
	err := &HTTPError{
		Method:     "GET",
		Path:       "/api/me",
		StatusCode: 401,
		Body:       "unauthorized",
	}

	expected := "GET /api/me returned 401: unauthorized"
	if err.Error() != expected {
		t.Errorf("Error(): got %q, want %q", err.Error(), expected)
	}
}
