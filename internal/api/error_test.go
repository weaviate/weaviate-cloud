package api_test

import (
	"fmt"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

func TestErrorSurfacesWireCode(t *testing.T) {
	t.Parallel()
	apiErr := &api.Error{HTTPStatus: 401, Code: errcode.CodeAuthRequired, Message: "missing or invalid token"}

	if apiErr.ErrorCode() != errcode.CodeAuthRequired {
		t.Fatalf("ErrorCode() = %q, want %q", apiErr.ErrorCode(), errcode.CodeAuthRequired)
	}

	wrapped := fmt.Errorf("whoami: %w", apiErr)
	if code, ok := errcode.CodeFor(wrapped); !ok || code != errcode.CodeAuthRequired {
		t.Fatalf("CodeFor(wrapped) = (%q, %v), want (%q, true)", code, ok, errcode.CodeAuthRequired)
	}
	if got := errcode.ExitCodeFor(wrapped); got != errcode.AuthRequired {
		t.Fatalf("ExitCodeFor(wrapped) = %d, want %d", got, errcode.AuthRequired)
	}
}
