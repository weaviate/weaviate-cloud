package errcode_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

func TestErrorUnwrapsCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("dial tcp 127.0.0.1:1: connect: connection refused")
	err := errcode.NoResponse(errcode.PeerAuthServer, cause)

	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is did not reach the cause through *errcode.Error: %v", err)
	}
}

func TestNoResponseCarriesTransportDiscriminator(t *testing.T) {
	t.Parallel()
	err := errcode.NoResponse(errcode.PeerAuthServer, errors.New("boom"))

	if err.Code != errcode.CodeInternalError {
		t.Errorf("Code = %q, want %q (the frozen code and exit must not move)", err.Code, errcode.CodeInternalError)
	}
	if got := err.Details[errcode.DetailFailureStage]; got != errcode.FailureStageTransport {
		t.Errorf("Details[%q] = %v, want %q", errcode.DetailFailureStage, got, errcode.FailureStageTransport)
	}
	if got := errcode.ExitCodeFor(err); got != errcode.GenericError {
		t.Errorf("ExitCodeFor = %d, want %d", got, errcode.GenericError)
	}
}

func TestCodeForPrefersOuterErrcodeError(t *testing.T) {
	t.Parallel()
	inner := coderError{errcode.CodeAuthRequired}
	outer := &errcode.Error{Code: errcode.CodeInternalError, Message: "outer", Cause: inner}

	code, ok := errcode.CodeFor(outer)
	if !ok || code != errcode.CodeInternalError {
		t.Fatalf("CodeFor = (%q, %v), want (%q, true) — the outer code must win", code, ok, errcode.CodeInternalError)
	}
	if got := errcode.ExitCodeFor(outer); got != errcode.GenericError {
		t.Errorf("ExitCodeFor = %d, want %d", got, errcode.GenericError)
	}
}

func TestNoResponseMessageRegister(t *testing.T) {
	t.Parallel()
	msg := errcode.NoResponse(errcode.PeerAuthServer, errors.New("boom")).Message

	required := []string{
		"no response was received from",
		"neither accepted nor rejected",
		"auth.weaviate.cloud",
		"api-cloud.weaviate.cloud",
		"only an administrator can change it",
		"wcloud guide",
	}
	for _, phrase := range required {
		if !strings.Contains(msg, phrase) {
			t.Errorf("message is missing required content %q\nmessage: %s", phrase, msg)
		}
	}

	rejected := []string{"your credentials are", "never reached", "your editor blocked", "sandbox blocked this"}
	for _, phrase := range rejected {
		if strings.Contains(msg, phrase) {
			t.Errorf("message asserts what the CLI cannot observe: %q\nmessage: %s", phrase, msg)
		}
	}
}
