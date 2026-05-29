package middleware_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
	"pgregory.net/rapid"
)

// Property 1: Request body limit enforcement
//
// For any HTTP request body with size S and a configured limit L:
// - if S > L then the server SHALL respond with HTTP 413
// - if S <= L then the request SHALL be processed normally (HTTP 200)
// - The 413 response SHALL contain {"error":"request body too large"}
//
// **Validates: Requirements 5.1, 5.2, 5.3, 5.4**

// bodyLimitTestHandler reads the full request body and returns 200 on success,
// or uses IsMaxBytesError + WriteBodyTooLargeError for proper error handling.
func bodyLimitTestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			if middleware.IsMaxBytesError(err) {
				middleware.WriteBodyTooLargeError(w)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"size": len(body),
		})
	}
}

func TestProperty1_BodyLimitEnforcement_OversizedRejected(t *testing.T) {
	// Property 1: Request body limit enforcement
	// For any body size S > configured limit L, the server returns HTTP 413
	// with {"error":"request body too large"}.
	rapid.Check(t, func(t *rapid.T) {
		// Generate a limit from the configured values: 32KB, 64KB, 1MB
		limits := []int64{32 << 10, 64 << 10, 1 << 20}
		limitIdx := rapid.IntRange(0, len(limits)-1).Draw(t, "limitIdx")
		limit := limits[limitIdx]

		// Generate a body size that exceeds the limit (limit+1 to limit+4096)
		excess := int64(rapid.IntRange(1, 4096).Draw(t, "excess"))
		bodySize := limit + excess

		handler := middleware.BodyLimit(limit)(middleware.BodyLimitErrorHandler(bodyLimitTestHandler()))

		body := make([]byte, bodySize)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected HTTP 413 for body size %d with limit %d, got %d",
				bodySize, limit, rec.Code)
		}

		// Verify the error response body
		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode 413 response body: %v", err)
		}
		if resp["error"] != "request body too large" {
			t.Fatalf("expected error 'request body too large', got %q", resp["error"])
		}
	})
}

func TestProperty1_BodyLimitEnforcement_WithinLimitSucceeds(t *testing.T) {
	// Property 1: Request body limit enforcement
	// For any body size S <= configured limit L, the request succeeds with HTTP 200.
	rapid.Check(t, func(t *rapid.T) {
		// Generate a limit from the configured values: 32KB, 64KB, 1MB
		limits := []int64{32 << 10, 64 << 10, 1 << 20}
		limitIdx := rapid.IntRange(0, len(limits)-1).Draw(t, "limitIdx")
		limit := limits[limitIdx]

		// Generate a body size within the limit (0 to limit)
		bodySize := int64(rapid.IntRange(0, int(limit)).Draw(t, "bodySize"))

		handler := middleware.BodyLimit(limit)(middleware.BodyLimitErrorHandler(bodyLimitTestHandler()))

		body := make([]byte, bodySize)
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200 for body size %d with limit %d, got %d",
				bodySize, limit, rec.Code)
		}

		// Verify the response reports the correct body size
		var resp map[string]interface{}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode success response body: %v", err)
		}
		gotSize := int64(resp["size"].(float64))
		if gotSize != bodySize {
			t.Fatalf("expected response size %d, got %d", bodySize, gotSize)
		}
	})
}

func TestProperty1_BodyLimitEnforcement_BoundaryBehavior(t *testing.T) {
	// Property 1: Request body limit enforcement
	// For any configured limit L, a body of exactly L bytes succeeds (200)
	// and a body of L+1 bytes is rejected (413).
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random limit between 64 bytes and 64KB to keep tests fast
		limit := int64(rapid.IntRange(64, 64<<10).Draw(t, "limit"))

		handler := middleware.BodyLimit(limit)(middleware.BodyLimitErrorHandler(bodyLimitTestHandler()))

		// Exactly at limit — should succeed
		exactBody := make([]byte, limit)
		reqExact := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(exactBody))
		recExact := httptest.NewRecorder()
		handler.ServeHTTP(recExact, reqExact)

		if recExact.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200 for body exactly at limit %d, got %d",
				limit, recExact.Code)
		}

		// One byte over limit — should be rejected
		overBody := make([]byte, limit+1)
		reqOver := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(overBody))
		recOver := httptest.NewRecorder()
		handler.ServeHTTP(recOver, reqOver)

		if recOver.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected HTTP 413 for body at limit+1 (%d bytes) with limit %d, got %d",
				limit+1, limit, recOver.Code)
		}

		// Verify the 413 response contains the correct error message
		var resp map[string]string
		if err := json.NewDecoder(recOver.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode 413 response body: %v", err)
		}
		if resp["error"] != "request body too large" {
			t.Fatalf("expected error 'request body too large', got %q", resp["error"])
		}
	})
}
