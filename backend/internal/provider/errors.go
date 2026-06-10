package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const (
	ErrorTypeAuth            = "auth"
	ErrorTypeRateLimited     = "rate_limited"
	ErrorTypeQuotaExhausted  = "quota_exhausted"
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
	trimmed := trimBody(body)
	lower := strings.ToLower(body)
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		if containsQuotaExhausted(lower) {
			return &Error{Type: ErrorTypeQuotaExhausted, StatusCode: status, Message: trimmed}
		}
		return &Error{Type: ErrorTypeAuth, StatusCode: status, Message: trimmed}
	case http.StatusPaymentRequired:
		return &Error{Type: ErrorTypeQuotaExhausted, StatusCode: status, Message: trimmed}
	case http.StatusTooManyRequests:
		return &Error{Type: ErrorTypeRateLimited, StatusCode: status, Message: trimmed}
	default:
		if containsQuotaExhausted(lower) {
			return &Error{Type: ErrorTypeQuotaExhausted, StatusCode: status, Message: trimmed}
		}
		return &Error{Type: ErrorTypeUpstream, StatusCode: status, Message: trimmed}
	}
}

func ErrorType(err error) string {
	if err == nil {
		return ""
	}
	if providerErr, ok := err.(*Error); ok {
		return providerErr.Type
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return ErrorTypeTimeout
	}
	return ErrorTypeUpstream
}

func containsQuotaExhausted(body string) bool {
	return strings.Contains(body, "insufficientbalance") || strings.Contains(body, "insufficient balance") || strings.Contains(body, "quota") || strings.Contains(body, "credit")
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
