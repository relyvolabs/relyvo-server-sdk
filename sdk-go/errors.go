package relyvo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrNotConfigured is returned by chat methods when Config.ChatURL is empty.
// It is a configuration error, not a transient one: it is never retried.
var ErrNotConfigured = errors.New("relyvo: chat is not configured; set Config.ChatURL")

// Error is a failed Relyvo call.
//
// StatusCode is 0 when the request never completed (DNS, connection, timeout,
// context cancellation). In that case Unwrap returns the transport cause.
type Error struct {
	// StatusCode is the HTTP status, or 0 for a transport failure.
	StatusCode int

	// Code is the service's stable machine code, for example
	// `insufficient_credits`, `chat_disabled` or `concurrent_limit`.
	Code string

	// Message is the human-readable text from the service.
	Message string

	// RequestID correlates with server logs and support.
	RequestID string

	// Details carries the service's optional structured detail.
	Details any

	// RetryAfter is what the service asked for, when it said.
	RetryAfter time.Duration

	// err is the transport cause, if any.
	err error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	var b strings.Builder
	b.WriteString("relyvo: ")

	if e.Code != "" {
		b.WriteString(e.Code)
	} else {
		b.WriteString("request failed")
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}

	if e.StatusCode != 0 {
		fmt.Fprintf(&b, " (status %d", e.StatusCode)
		if e.RequestID != "" {
			b.WriteString(", request ")
			b.WriteString(e.RequestID)
		}
		b.WriteString(")")
	} else if e.RequestID != "" {
		b.WriteString(" (request ")
		b.WriteString(e.RequestID)
		b.WriteString(")")
	}

	return b.String()
}

// Unwrap exposes the transport cause so errors.Is/As can reach it.
func (e *Error) Unwrap() error { return e.err }

// Retryable reports whether a later attempt could succeed.
//
// This describes the failure, not the call: the caller still decides whether the
// operation itself is safe to repeat.
func (e *Error) Retryable() bool {
	if e == nil {
		return false
	}
	if e.StatusCode == 0 {
		return true
	}

	switch e.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// AsError extracts a *Error from an error chain.
func AsError(err error) (*Error, bool) {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

// IsCode reports whether err is a Relyvo error with the given code.
func IsCode(err error, code string) bool {
	apiErr, ok := AsError(err)
	return ok && apiErr.Code == code
}

// isRetryable is the transport-level check used by do.
func isRetryable(err error) bool {
	apiErr, ok := AsError(err)
	if !ok {
		return false
	}
	return apiErr.Retryable()
}

// errorEnvelope accepts both shapes in use: Relyvo API puts `request_id` inside
// `error`, Relyvo Chat puts it at the top level. A client that only understood one
// would lose the correlation id for half its calls.
type errorEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Details   any    `json:"details"`
		RequestID string `json:"request_id"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

// newAPIError maps a non-2xx response onto Error.
func newAPIError(response *http.Response, body []byte) *Error {
	apiErr := &Error{
		StatusCode: response.StatusCode,
		RetryAfter: parseRetryAfter(response.Header.Get("Retry-After")),
	}

	var envelope errorEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Code != "" {
		apiErr.Code = envelope.Error.Code
		apiErr.Message = envelope.Error.Message
		apiErr.Details = envelope.Error.Details
		apiErr.RequestID = envelope.Error.RequestID
		if apiErr.RequestID == "" {
			apiErr.RequestID = envelope.RequestID
		}
		return apiErr
	}

	// A proxy or a crashed process can answer with HTML or an empty body. Report
	// the status honestly instead of inventing a service code.
	apiErr.Code = "unexpected_response"
	apiErr.Message = fmt.Sprintf("the service returned HTTP %d", response.StatusCode)
	return apiErr
}

// parseRetryAfter accepts both documented forms: delay in seconds and HTTP date.
func parseRetryAfter(value string) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(trimmed); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}

	if when, err := http.ParseTime(trimmed); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}

	return 0
}
