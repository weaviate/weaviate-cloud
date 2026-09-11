package errcode

import "errors"

const (
	Success            = 0
	GenericError       = 1
	UsageError         = 2
	AuthRequired       = 3
	NotFound           = 4
	PermissionDenied   = 5
	Conflict           = 6
	QuotaExceeded      = 7
	RateLimited        = 8
	ServiceUnavailable = 9
)

const (
	CodeValidationFailed    = "validation_failed"
	CodeAuthRequired        = "auth_required"
	CodeClusterNotFound     = "cluster_not_found"
	CodePermissionDenied    = "permission_denied"
	CodeAccessRestricted    = "access_restricted"
	CodeClusterAlreadyExist = "cluster_already_exists"
	CodeQuotaExceeded       = "quota_exceeded"
	CodeRateLimited         = "rate_limited"
	CodeServiceUnavailable  = "service_unavailable"
	CodeInternalError       = "internal_error"
)

// Coder is any error carrying one of the frozen wire error codes, letting the
// CLI recover the code from an *Error or an api.Error without coupling the two.
type Coder interface {
	ErrorCode() string
}

type Error struct {
	Code    string
	Message string
	Details map[string]any
	Cause   error
}

func (e *Error) Error() string     { return e.Message }
func (e *Error) ErrorCode() string { return e.Code }
func (e *Error) Unwrap() error     { return e.Cause }

func New(code, message string) *Error { return &Error{Code: code, Message: message} }

func ExitCodeFor(err error) int {
	if err == nil {
		return Success
	}
	if code, ok := CodeFor(err); ok {
		return exitForCode(code)
	}
	return GenericError
}

func CodeFor(err error) (string, bool) {
	var c Coder
	if err != nil && errors.As(err, &c) {
		return c.ErrorCode(), true
	}
	return "", false
}

func exitForCode(code string) int {
	switch code {
	case CodeValidationFailed:
		return UsageError
	case CodeAuthRequired:
		return AuthRequired
	case CodeClusterNotFound:
		return NotFound
	case CodePermissionDenied:
		return PermissionDenied
	case CodeAccessRestricted:
		return PermissionDenied
	case CodeClusterAlreadyExist:
		return Conflict
	case CodeQuotaExceeded:
		return QuotaExceeded
	case CodeRateLimited:
		return RateLimited
	case CodeServiceUnavailable:
		return ServiceUnavailable
	default:
		return GenericError
	}
}
