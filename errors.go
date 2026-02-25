package cronbeatsgo

import "fmt"

type ApiErrorCode string

const (
	CodeValidation ApiErrorCode = "VALIDATION_ERROR"
	CodeNotFound   ApiErrorCode = "NOT_FOUND"
	CodeRateLimit  ApiErrorCode = "RATE_LIMITED"
	CodeServer     ApiErrorCode = "SERVER_ERROR"
	CodeNetwork    ApiErrorCode = "NETWORK_ERROR"
	CodeUnknown    ApiErrorCode = "UNKNOWN_ERROR"
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type SdkError struct {
	Message string
	Cause   error
}

func (e *SdkError) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

func (e *SdkError) Unwrap() error {
	return e.Cause
}

type ApiError struct {
	Code       ApiErrorCode
	HTTPStatus *int
	Retryable  bool
	Message    string
	Raw        any
}

func (e *ApiError) Error() string {
	return e.Message
}
