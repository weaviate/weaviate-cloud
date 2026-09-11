package errcode_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

func TestExitCodeFor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, errcode.Success},
		{"validation_failed", errcode.New(errcode.CodeValidationFailed, "x"), errcode.UsageError},
		{"auth_required", errcode.New(errcode.CodeAuthRequired, "x"), errcode.AuthRequired},
		{"cluster_not_found", errcode.New(errcode.CodeClusterNotFound, "x"), errcode.NotFound},
		{"permission_denied", errcode.New(errcode.CodePermissionDenied, "x"), errcode.PermissionDenied},
		{"access_restricted", errcode.New(errcode.CodeAccessRestricted, "x"), errcode.PermissionDenied},
		{"cluster_already_exists", errcode.New(errcode.CodeClusterAlreadyExist, "x"), errcode.Conflict},
		{"quota_exceeded", errcode.New(errcode.CodeQuotaExceeded, "x"), errcode.QuotaExceeded},
		{"rate_limited", errcode.New(errcode.CodeRateLimited, "x"), errcode.RateLimited},
		{"service_unavailable", errcode.New(errcode.CodeServiceUnavailable, "x"), errcode.ServiceUnavailable},
		{"unknown_structured_code", errcode.New("something_weird", "x"), errcode.GenericError},
		{"plain_error", errors.New("boom"), errcode.GenericError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := errcode.ExitCodeFor(tc.err); got != tc.want {
				t.Errorf("ExitCodeFor(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestErrorIsWrappable(t *testing.T) {
	t.Parallel()
	base := errcode.New(errcode.CodeAuthRequired, "login required")
	wrapped := errors.Join(errors.New("context"), base)

	if got := errcode.ExitCodeFor(wrapped); got != errcode.AuthRequired {
		t.Errorf("wrapped exit = %d, want %d", got, errcode.AuthRequired)
	}
}

type coderError struct{ code string }

func (f coderError) Error() string     { return f.code }
func (f coderError) ErrorCode() string { return f.code }

func TestExitCodeForRecognizesCoder(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("whoami: %w", coderError{errcode.CodeAuthRequired})
	if got := errcode.ExitCodeFor(wrapped); got != errcode.AuthRequired {
		t.Errorf("ExitCodeFor(coder) = %d, want %d", got, errcode.AuthRequired)
	}
}

func TestCodeFor(t *testing.T) {
	t.Parallel()
	if code, ok := errcode.CodeFor(coderError{errcode.CodeAuthRequired}); !ok || code != errcode.CodeAuthRequired {
		t.Errorf("CodeFor(coder) = (%q, %v), want (%q, true)", code, ok, errcode.CodeAuthRequired)
	}
	if code, ok := errcode.CodeFor(errors.New("boom")); ok {
		t.Errorf("CodeFor(plain) = (%q, %v), want (\"\", false)", code, ok)
	}
}
