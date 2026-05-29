package middleware_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agenticflow/agenticflow/server/internal/middleware"
)

// testHandler reads the full request body and returns 200 with the body content.
func testHandler() http.HandlerFunc {
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

func TestBodyLimit_AllowsRequestWithinLimit(t *testing.T) {
	limit := int64(1024) // 1 KB
	handler := middleware.BodyLimit(limit)(middleware.BodyLimitErrorHandler(testHandler()))

	body := strings.Repeat("a", 512) // 512 bytes, within limit
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if int(resp["size"].(float64)) != 512 {
		t.Errorf("expected body size 512, got %v", resp["size"])
	}
}

func TestBodyLimit_RejectsOversizedRequest(t *testing.T) {
	limit := int64(1024) // 1 KB
	handler := middleware.BodyLimit(limit)(middleware.BodyLimitErrorHandler(testHandler()))

	body := strings.Repeat("a", 2048) // 2 KB, exceeds limit
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "request body too large" {
		t.Errorf("expected error 'request body too large', got %q", resp["error"])
	}
}

func TestBodyLimit_ExactLimitAllowed(t *testing.T) {
	limit := int64(100)
	handler := middleware.BodyLimit(limit)(middleware.BodyLimitErrorHandler(testHandler()))

	body := strings.Repeat("x", 100) // exactly at limit
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestBodyLimit_OneBytePastLimitRejected(t *testing.T) {
	limit := int64(100)
	handler := middleware.BodyLimit(limit)(middleware.BodyLimitErrorHandler(testHandler()))

	body := strings.Repeat("x", 101) // one byte past limit
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "request body too large" {
		t.Errorf("expected error 'request body too large', got %q", resp["error"])
	}
}

func TestBodyLimit_EmptyBodyAllowed(t *testing.T) {
	limit := int64(1024)
	handler := middleware.BodyLimit(limit)(middleware.BodyLimitErrorHandler(testHandler()))

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(nil))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestBodyLimit_ResponseContentTypeIsJSON(t *testing.T) {
	limit := int64(10)
	handler := middleware.BodyLimit(limit)(middleware.BodyLimitErrorHandler(testHandler()))

	body := strings.Repeat("a", 100)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestBodyLimit_DifferentLimitsPerRoute(t *testing.T) {
	tests := []struct {
		name      string
		limit     int64
		bodySize  int
		wantCode  int
	}{
		{"32KB limit - small body", 32 << 10, 1024, http.StatusOK},
		{"32KB limit - oversized body", 32 << 10, 33 * 1024, http.StatusRequestEntityTooLarge},
		{"64KB limit - medium body", 64 << 10, 32 * 1024, http.StatusOK},
		{"64KB limit - oversized body", 64 << 10, 65 * 1024, http.StatusRequestEntityTooLarge},
		{"1MB limit - large body", 1 << 20, 512 * 1024, http.StatusOK},
		{"1MB limit - oversized body", 1 << 20, (1 << 20) + 1, http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := middleware.BodyLimit(tt.limit)(middleware.BodyLimitErrorHandler(testHandler()))

			body := strings.Repeat("a", tt.bodySize)
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("expected status %d, got %d", tt.wantCode, rec.Code)
			}
		})
	}
}

func TestIsMaxBytesError(t *testing.T) {
	limit := int64(10)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			if middleware.IsMaxBytesError(err) {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.BodyLimit(limit)(handler)

	body := strings.Repeat("x", 100)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rec.Code)
	}
}
