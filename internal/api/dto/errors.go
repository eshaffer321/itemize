package dto

// APIError represents a structured error response.
// All error responses from the API use this format for consistency.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Common error codes
const (
	ErrCodeNotFound       = "not_found"
	ErrCodeBadRequest     = "bad_request"
	ErrCodeInternalError  = "internal_error"
	ErrCodeValidation     = "validation_error"
)

// NewAPIError creates a new APIError with the given code and message.
func NewAPIError(code, message string) APIError {
	return APIError{
		Code:    code,
		Message: message,
	}
}

// NotFoundError creates a not found error response.
func NotFoundError(resource string) APIError {
	return NewAPIError(ErrCodeNotFound, resource+" not found")
}

// BadRequestError creates a bad request error response.
func BadRequestError(message string) APIError {
	return NewAPIError(ErrCodeBadRequest, message)
}

// InternalError creates an internal server error response.
func InternalError() APIError {
	return NewAPIError(ErrCodeInternalError, "an internal error occurred")
}

// ValidationError creates a validation error response.
func ValidationError(message string) APIError {
	return NewAPIError(ErrCodeValidation, message)
}
