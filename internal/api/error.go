package api

import (
	"fmt"
	"time"
)

// Error preserves the frozen error.code from the wire envelope so callers can
// switch on it (the API contract guarantees the code never changes meaning).
type Error struct {
	HTTPStatus int
	Code       string
	Message    string
	RawExcerpt string
	RequestID  string
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("api: %s: %s (request_id=%s)", e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("api: %s: %s", e.Code, e.Message)
}

func (e *Error) ErrorCode() string { return e.Code }
