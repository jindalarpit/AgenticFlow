package service

// ErrorKind classifies service-layer errors so that HTTP handlers can map them
// to the appropriate status code without inspecting error messages.
type ErrorKind int

const (
	ErrNotFound      ErrorKind = iota // 404
	ErrValidation                     // 400
	ErrForbidden                      // 403
	ErrConflict                       // 409
	ErrUnprocessable                  // 422
	ErrBadGateway                     // 502
	ErrInternal                       // 500
)

// ServiceError is a typed error returned by service-layer methods. Handlers
// inspect Kind to determine the HTTP status code and use Message for the
// response body.
type ServiceError struct {
	Kind    ErrorKind
	Message string
	Err     error // optional wrapped error
}

func (e *ServiceError) Error() string { return e.Message }
func (e *ServiceError) Unwrap() error { return e.Err }

// NotFound returns a ServiceError indicating the requested resource was not found.
func NotFound(msg string) *ServiceError {
	return &ServiceError{Kind: ErrNotFound, Message: msg}
}

// Validation returns a ServiceError indicating invalid input or a business rule violation.
func Validation(msg string) *ServiceError {
	return &ServiceError{Kind: ErrValidation, Message: msg}
}

// Forbidden returns a ServiceError indicating the caller lacks permission.
func Forbidden(msg string) *ServiceError {
	return &ServiceError{Kind: ErrForbidden, Message: msg}
}

// Conflict returns a ServiceError indicating a state conflict (e.g. duplicate resource).
func Conflict(msg string) *ServiceError {
	return &ServiceError{Kind: ErrConflict, Message: msg}
}

// Unprocessable returns a ServiceError indicating a semantic validation failure (422).
func Unprocessable(msg string) *ServiceError {
	return &ServiceError{Kind: ErrUnprocessable, Message: msg}
}

// BadGateway returns a ServiceError indicating an upstream service failure (502).
func BadGateway(msg string) *ServiceError {
	return &ServiceError{Kind: ErrBadGateway, Message: msg}
}

// Internal returns a ServiceError indicating an unexpected internal failure.
func Internal(msg string) *ServiceError {
	return &ServiceError{Kind: ErrInternal, Message: msg}
}

// HTTPStatus maps an ErrorKind to the corresponding HTTP status code.
func (k ErrorKind) HTTPStatus() int {
	switch k {
	case ErrNotFound:
		return 404
	case ErrValidation:
		return 400
	case ErrForbidden:
		return 403
	case ErrConflict:
		return 409
	case ErrUnprocessable:
		return 422
	case ErrBadGateway:
		return 502
	case ErrInternal:
		return 500
	default:
		return 500
	}
}
