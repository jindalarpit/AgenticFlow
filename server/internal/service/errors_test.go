package service

import (
	"errors"
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// **Validates: Requirements 8.5**
// Property 11: Service layer typed error mapping
//
// For any ErrorKind, the corresponding constructor creates a ServiceError with
// the correct Kind, the Error() method returns the message, the Unwrap() method
// returns the wrapped error (or nil), and HTTPStatus() maps correctly.

// allKinds enumerates all defined ErrorKind values.
var allKinds = []ErrorKind{ErrNotFound, ErrValidation, ErrForbidden, ErrConflict, ErrInternal}

// expectedHTTPStatus maps each ErrorKind to its expected HTTP status code.
var expectedHTTPStatus = map[ErrorKind]int{
	ErrNotFound:   404,
	ErrValidation: 400,
	ErrForbidden:  403,
	ErrConflict:   409,
	ErrInternal:   500,
}

// constructorForKind returns the constructor function for a given ErrorKind.
func constructorForKind(k ErrorKind) func(string) *ServiceError {
	switch k {
	case ErrNotFound:
		return NotFound
	case ErrValidation:
		return Validation
	case ErrForbidden:
		return Forbidden
	case ErrConflict:
		return Conflict
	case ErrInternal:
		return Internal
	default:
		return nil
	}
}

// TestProperty_ServiceError_ConstructorSetsCorrectKind verifies that for any
// ErrorKind and any message string, the corresponding constructor creates a
// ServiceError with the correct Kind field.
func TestProperty_ServiceError_ConstructorSetsCorrectKind(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		kind := rapid.SampledFrom(allKinds).Draw(t, "kind")
		msg := rapid.String().Draw(t, "message")

		constructor := constructorForKind(kind)
		err := constructor(msg)

		if err.Kind != kind {
			t.Fatalf("expected Kind=%d, got Kind=%d", kind, err.Kind)
		}
	})
}

// TestProperty_ServiceError_ErrorReturnsMessage verifies that for any ErrorKind
// and any message, Error() returns the exact message string.
func TestProperty_ServiceError_ErrorReturnsMessage(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		kind := rapid.SampledFrom(allKinds).Draw(t, "kind")
		msg := rapid.String().Draw(t, "message")

		constructor := constructorForKind(kind)
		err := constructor(msg)

		if err.Error() != msg {
			t.Fatalf("expected Error()=%q, got %q", msg, err.Error())
		}
	})
}

// TestProperty_ServiceError_UnwrapReturnsNilForConstructors verifies that
// constructors create errors with Unwrap() returning nil (no wrapped error).
func TestProperty_ServiceError_UnwrapReturnsNilForConstructors(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		kind := rapid.SampledFrom(allKinds).Draw(t, "kind")
		msg := rapid.String().Draw(t, "message")

		constructor := constructorForKind(kind)
		err := constructor(msg)

		if err.Unwrap() != nil {
			t.Fatalf("expected Unwrap()=nil, got %v", err.Unwrap())
		}
	})
}

// TestProperty_ServiceError_UnwrapReturnsWrappedError verifies that when a
// ServiceError wraps another error, Unwrap() returns that exact error.
func TestProperty_ServiceError_UnwrapReturnsWrappedError(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		kind := rapid.SampledFrom(allKinds).Draw(t, "kind")
		msg := rapid.String().Draw(t, "message")
		wrappedMsg := rapid.String().Draw(t, "wrappedMsg")

		wrapped := fmt.Errorf("cause: %s", wrappedMsg)
		svcErr := &ServiceError{Kind: kind, Message: msg, Err: wrapped}

		if svcErr.Unwrap() != wrapped {
			t.Fatalf("expected Unwrap() to return the wrapped error")
		}
		if !errors.Is(svcErr, wrapped) {
			t.Fatalf("errors.Is should find the wrapped error")
		}
	})
}

// TestProperty_ServiceError_HTTPStatusMapping verifies that for any ErrorKind,
// HTTPStatus() returns the correct HTTP status code.
func TestProperty_ServiceError_HTTPStatusMapping(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		kind := rapid.SampledFrom(allKinds).Draw(t, "kind")

		got := kind.HTTPStatus()
		expected := expectedHTTPStatus[kind]

		if got != expected {
			t.Fatalf("ErrorKind(%d).HTTPStatus() = %d, want %d", kind, got, expected)
		}
	})
}

// TestProperty_ServiceError_ImplementsErrorInterface verifies that *ServiceError
// satisfies the error interface for any inputs.
func TestProperty_ServiceError_ImplementsErrorInterface(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		kind := rapid.SampledFrom(allKinds).Draw(t, "kind")
		msg := rapid.String().Draw(t, "message")

		constructor := constructorForKind(kind)
		var err error = constructor(msg)

		// Verify it can be type-asserted back to *ServiceError
		var svcErr *ServiceError
		if !errors.As(err, &svcErr) {
			t.Fatalf("errors.As should find *ServiceError")
		}
		if svcErr.Kind != kind {
			t.Fatalf("after errors.As, Kind=%d, want %d", svcErr.Kind, kind)
		}
	})
}
