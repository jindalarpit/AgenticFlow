package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockRegisterHandler creates a test handler that only validates the request
// without hitting the database. This tests the validation logic in isolation.
func TestRegister_ValidationOnly(t *testing.T) {
	tests := []struct {
		name       string
		body       registerRequest
		wantStatus int
		wantError  string
	}{
		{
			name:       "empty name rejected",
			body:       registerRequest{Name: "", Email: "user@example.com", Password: "password123"},
			wantStatus: http.StatusBadRequest,
			wantError:  "name is required",
		},
		{
			name:       "whitespace-only name rejected",
			body:       registerRequest{Name: "   ", Email: "user@example.com", Password: "password123"},
			wantStatus: http.StatusBadRequest,
			wantError:  "name is required",
		},
		{
			name:       "name exceeding 128 chars rejected",
			body:       registerRequest{Name: strings.Repeat("a", 129), Email: "user@example.com", Password: "password123"},
			wantStatus: http.StatusBadRequest,
			wantError:  "name must not exceed 128 characters",
		},
		{
			name:       "invalid email no at sign",
			body:       registerRequest{Name: "Test", Email: "invalidemail", Password: "password123"},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid email format",
		},
		{
			name:       "invalid email no dot in domain",
			body:       registerRequest{Name: "Test", Email: "user@domain", Password: "password123"},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid email format",
		},
		{
			name:       "invalid email two at signs",
			body:       registerRequest{Name: "Test", Email: "user@foo@bar.com", Password: "password123"},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid email format",
		},
		{
			name:       "invalid email exceeds 254 chars",
			body:       registerRequest{Name: "Test", Email: "a@" + strings.Repeat("x", 251) + ".com", Password: "password123"},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid email format",
		},
		{
			name:       "password too short",
			body:       registerRequest{Name: "Test", Email: "user@example.com", Password: "short"},
			wantStatus: http.StatusBadRequest,
			wantError:  "password must be at least 8 characters",
		},
		{
			name:       "password too long",
			body:       registerRequest{Name: "Test", Email: "user@example.com", Password: strings.Repeat("a", 129)},
			wantStatus: http.StatusBadRequest,
			wantError:  "password must not exceed 128 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't test the full handler without a DB, but we can test
			// validation by calling the handler with a nil Queries (it will
			// fail at the DB step for valid inputs, but validation errors
			// should be caught before that).
			h := &AuthHandler{Queries: nil}

			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// This will panic on valid inputs (nil Queries), but validation
			// errors should be returned before hitting the DB.
			func() {
				defer func() {
					if r := recover(); r != nil {
						// If we panicked, it means validation passed and we hit the DB call.
						// This is expected for valid inputs — mark as unexpected for our test cases.
						t.Fatalf("handler panicked (validation passed unexpectedly): %v", r)
					}
				}()
				h.Register(w, req)
			}()

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if !strings.Contains(resp["error"], tt.wantError) {
				t.Errorf("error = %q, want to contain %q", resp["error"], tt.wantError)
			}
		})
	}
}

func TestRegister_ValidInputPassesValidation(t *testing.T) {
	// Valid inputs should pass validation and reach the DB call.
	// Since we have nil Queries, it will panic — that's our signal that
	// validation passed successfully.
	validCases := []registerRequest{
		{Name: "Test User", Email: "user@example.com", Password: "password123"},
		{Name: "A", Email: "a@b.co", Password: "12345678"},
		{Name: strings.Repeat("a", 128), Email: "test@domain.org", Password: strings.Repeat("x", 128)},
		{Name: "  trimmed  ", Email: "USER@EXAMPLE.COM", Password: "validpass"},
	}

	for _, req := range validCases {
		t.Run(req.Email, func(t *testing.T) {
			h := &AuthHandler{Queries: nil}

			body, _ := json.Marshal(req)
			r := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			panicked := false
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						panicked = true
					}
				}()
				h.Register(w, r)
			}()

			// If it didn't panic and returned a 4xx, validation failed unexpectedly
			if !panicked && w.Code >= 400 && w.Code < 500 {
				t.Errorf("valid input %+v was rejected with status %d: %s",
					req, w.Code, w.Body.String())
			}
		})
	}
}
