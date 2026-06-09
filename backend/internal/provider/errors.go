package provider

import (
	"fmt"
	"net/http"
)

const (
	ErrorTypeAuth            = "auth"
	ErrorTypeRateLimited     = "rate_limited"
	ErrorTypeUpstream        = "upstream"
	ErrorTypeTimeout         = "timeout"
	ErrorTypeInvalidResponse = "invalid_response"
	ErrorTypeNoKey           = "no_key"
)

type Error struct {
	Type       string
	StatusCode int
	Message    string
}

func (e *Error) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s: %s (%d)", e.Type, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func ClassifyHTTPError(status int, body string) *Error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &Error{Type: ErrorTypeAuth, StatusCode: status, Message: trimBody(body)}
	case http.StatusTooManyRequests:
		return &Error{Type: ErrorTypeRateLimited, StatusCode: status, Message: trimBody(body)}
	default:
		return &Error{Type: ErrorTypeUpstream, StatusCode: status, Message: trimBody(body)}
	}
}

func ErrorType(err error) string {
	if err == nil {
		return ""
	}
	if providerErr, ok := err.(*Error); ok {
		return providerErr.Type
	}
	return ErrorTypeUpstream
}

func trimBody(body string) string {
	if body == "" {
		return "upstream request failed"
	}
	if len(body) > 300 {
		return body[:300]
	}
	return body
}
