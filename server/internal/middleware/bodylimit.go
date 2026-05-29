package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
)

// BodyLimit returns middleware that limits request body size using
// http.MaxBytesReader. If the body exceeds maxBytes, subsequent reads
// will return an error that BodyLimitErrorHandler can detect.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// BodyLimitErrorHandler is middleware that wraps handlers and detects
// *http.MaxBytesError when the request body exceeds the configured limit.
// It intercepts the response to return HTTP 413 with a JSON error body
// {"error":"request body too large"}.
//
// When http.MaxBytesReader detects an oversized body, it writes a 413 status
// to the ResponseWriter. This middleware intercepts that status code and
// replaces the response with a consistent JSON error format.
func BodyLimitErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bw := &bodyLimitResponseWriter{
			ResponseWriter: w,
		}
		next.ServeHTTP(bw, r)
	})
}

// IsMaxBytesError checks whether an error is caused by exceeding the
// request body size limit. This can be used by handlers when decoding
// JSON request bodies to return an appropriate 413 response.
func IsMaxBytesError(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

// WriteBodyTooLargeError writes an HTTP 413 response with a JSON error body.
func WriteBodyTooLargeError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	json.NewEncoder(w).Encode(map[string]string{"error": "request body too large"})
}

// bodyLimitResponseWriter wraps http.ResponseWriter. When http.MaxBytesReader
// detects an oversized body, it sets the response status to 413 automatically.
// This writer intercepts that to ensure we return our JSON error format.
type bodyLimitResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
	intercepted bool
}

func (w *bodyLimitResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if statusCode == http.StatusRequestEntityTooLarge {
		// Intercept the 413 from http.MaxBytesReader and replace with JSON error.
		w.intercepted = true
		w.ResponseWriter.Header().Set("Content-Type", "application/json")
		w.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w.ResponseWriter).Encode(map[string]string{"error": "request body too large"})
		return
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *bodyLimitResponseWriter) Write(b []byte) (int, error) {
	if w.intercepted {
		// Discard any writes from the handler after we've intercepted with 413.
		return len(b), nil
	}
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}
