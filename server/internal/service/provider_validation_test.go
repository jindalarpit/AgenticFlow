package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

// mockNetError implements net.Error for testing.
type mockNetError struct {
	msg     string
	timeout bool
}

func (e *mockNetError) Error() string   { return e.msg }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return false }

// Verify mockNetError implements net.Error
var _ net.Error = (*mockNetError)(nil)

func TestClassifyValidationError_Timeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "context deadline exceeded",
			err:  context.DeadlineExceeded,
			want: "connection timeout after 30 seconds",
		},
		{
			name: "wrapped context deadline exceeded",
			err:  fmt.Errorf("request failed: %w", context.DeadlineExceeded),
			want: "connection timeout after 30 seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyValidationError(tt.err)
			if got != tt.want {
				t.Errorf("classifyValidationError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyValidationError_NetworkError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "net.Error with timeout",
			err:  &mockNetError{msg: "i/o timeout", timeout: true},
			want: "connection timeout after 30 seconds",
		},
		{
			name: "net.Error without timeout",
			err:  &mockNetError{msg: "connection reset by peer", timeout: false},
			want: "provider endpoint unreachable: connection reset by peer",
		},
		{
			name: "connection refused in message",
			err:  errors.New("dial tcp 127.0.0.1:8080: connection refused"),
			want: "provider endpoint unreachable: dial tcp 127.0.0.1:8080: connection refused",
		},
		{
			name: "no such host in message",
			err:  errors.New("dial tcp: lookup api.example.com: no such host"),
			want: "provider endpoint unreachable: dial tcp: lookup api.example.com: no such host",
		},
		{
			name: "TLS error in message",
			err:  errors.New("tls: failed to verify certificate"),
			want: "provider endpoint unreachable: tls: failed to verify certificate",
		},
		{
			name: "DNS error in message",
			err:  errors.New("dns resolution failed for api.openai.com"),
			want: "provider endpoint unreachable: dns resolution failed for api.openai.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyValidationError(tt.err)
			if got != tt.want {
				t.Errorf("classifyValidationError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyValidationError_AuthFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "401 in error message",
			err:  errors.New("HTTP 401: Unauthorized"),
			want: "invalid credentials: authentication failed",
		},
		{
			name: "403 in error message",
			err:  errors.New("HTTP 403: Forbidden"),
			want: "invalid credentials: authentication failed",
		},
		{
			name: "authentication keyword",
			err:  errors.New("authentication failed: invalid API key"),
			want: "invalid credentials: authentication failed",
		},
		{
			name: "unauthorized keyword",
			err:  errors.New("request unauthorized"),
			want: "invalid credentials: authentication failed",
		},
		{
			name: "forbidden keyword",
			err:  errors.New("access forbidden"),
			want: "invalid credentials: authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyValidationError(tt.err)
			if got != tt.want {
				t.Errorf("classifyValidationError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyValidationError_OtherErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "generic error",
			err:  errors.New("something went wrong"),
			want: "something went wrong",
		},
		{
			name: "rate limit error",
			err:  errors.New("HTTP 429: Too Many Requests"),
			want: "HTTP 429: Too Many Requests",
		},
		{
			name: "server error",
			err:  errors.New("HTTP 500: Internal Server Error"),
			want: "HTTP 500: Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyValidationError(tt.err)
			if got != tt.want {
				t.Errorf("classifyValidationError() = %q, want %q", got, tt.want)
			}
		})
	}
}
