package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

//nolint:paralleltest // installs a process-wide signal handler; incompatible with t.Parallel
func TestNotifyContextCancelsOnInterrupt(t *testing.T) {
	ctx, stop := notifyContext()
	defer stop()

	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find process: %v", err)
	}
	if sigErr := p.Signal(os.Interrupt); sigErr != nil {
		t.Fatalf("signal self: %v", sigErr)
	}

	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("context was not cancelled by an interrupt: cobra commands would never observe Ctrl-C")
	}
}

func TestWriteErrorEnvelope(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		format  output.Format
		verbose bool
		err     error
		check   func(t *testing.T, stdout, stderr string)
	}{
		{
			name:   "json carries error.details",
			format: output.FormatJSON,
			err: &errcode.Error{
				Code:    errcode.CodeInternalError,
				Message: "timed out waiting for cluster cid-3 to become READY",
				Details: map[string]any{"last_status": "CREATING"},
			},
			check: assertDetailsCarried,
		},
		{
			name:   "json surfaces retry_after_seconds",
			format: output.FormatJSON,
			err: fmt.Errorf("cluster create: %w", &api.Error{
				HTTPStatus: http.StatusTooManyRequests,
				Code:       errcode.CodeRateLimited,
				Message:    "rate limited",
				RetryAfter: 30 * time.Second,
			}),
			check: assertRetryAfterSurfaced,
		},
		{
			name:   "json omits retry_after_seconds when zero",
			format: output.FormatJSON,
			err: fmt.Errorf("cluster create: %w", &api.Error{
				HTTPStatus: http.StatusTooManyRequests,
				Code:       errcode.CodeRateLimited,
				Message:    "rate limited",
				RetryAfter: 0,
			}),
			check: assertRetryAfterAbsent,
		},
		{
			name:   "text sanitizes escape sequences",
			format: output.FormatText,
			err: fmt.Errorf("get cluster: %w", &api.Error{
				HTTPStatus: http.StatusNotFound,
				Code:       "not_found",
				Message:    "not found\x1b[2K\r\x1b[31mFATAL: forged status line\x1b[0m",
				RequestID:  "poc-req",
			}),
			check: assertTextSanitized,
		},
		{
			name:   "json path is unaffected by sanitisation",
			format: output.FormatJSON,
			err:    &api.Error{Code: "not_found", Message: "legit\x1b[31mnot sanitized here\x1b[0m"},
			check:  assertJSONKeepsRawControlBytes,
		},
		{
			name:    "verbose prints a sanitized raw excerpt to stderr only",
			format:  output.FormatJSON,
			verbose: true,
			err: &api.Error{
				HTTPStatus: http.StatusBadGateway,
				Code:       "internal_error",
				Message:    "unexpected response: HTTP 502 returned a body that did not match the expected error format",
				RawExcerpt: "raw diagnostic body\x1b[31mFATAL\x1b[0m",
			},
			check: assertVerboseExcerptStaysOnStderr,
		},
		{
			name:   "non-verbose omits the raw excerpt",
			format: output.FormatJSON,
			err: &api.Error{
				Code:       "internal_error",
				Message:    "unexpected response: HTTP 502 returned a body that did not match the expected error format",
				RawExcerpt: "sensitive raw body",
			},
			check: assertExcerptOmitted,
		},
		{
			name:    "verbose text mode prints the excerpt beside the Error line",
			format:  output.FormatText,
			verbose: true,
			err: &api.Error{
				Code:       "internal_error",
				Message:    "unexpected response: HTTP 502 returned a body that did not match the expected error format",
				RawExcerpt: "diag body",
			},
			check: assertVerboseTextExcerpt,
		},
		{
			name:   "text names the retry delay",
			format: output.FormatText,
			err: fmt.Errorf("cluster create: %w", &api.Error{
				HTTPStatus: http.StatusTooManyRequests,
				Code:       errcode.CodeRateLimited,
				Message:    "rate limited",
				RetryAfter: 30 * time.Second,
			}),
			check: assertTextRetryAfter,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			io, _, stdout, stderr := iostreams.Test()
			f := &factory.Factory{
				IOStreams:    io,
				OutputFormat: tc.format,
				Verbose:      tc.verbose,
				NewRequestID: func() string { return "req-test" },
			}

			writeErrorEnvelope(f, tc.err)

			tc.check(t, stdout.String(), stderr.String())
		})
	}
}

func assertDetailsCarried(t *testing.T, stdout, _ string) {
	t.Helper()
	if stdout == "" {
		t.Fatal("stdout is empty: writeErrorEnvelope wrote nothing")
	}
	m := decodeEnvelope(t, stdout)
	if _, hasData := m["data"]; hasData {
		t.Fatalf("stdout must not contain a 'data' key on error: %s", stdout)
	}
	details, ok := errorObject(t, m, stdout)["details"].(map[string]any)
	if !ok {
		t.Fatalf("error.details must be an object, got: %s", stdout)
	}
	lastStatus, ok := details["last_status"].(string)
	if !ok || lastStatus != "CREATING" {
		t.Fatalf("error.details.last_status = %v, want CREATING", details["last_status"])
	}
}

func assertRetryAfterSurfaced(t *testing.T, stdout, _ string) {
	t.Helper()
	m := decodeEnvelope(t, stdout)
	details, ok := errorObject(t, m, stdout)["details"].(map[string]any)
	if !ok {
		t.Fatalf("error.details must be an object: %s", stdout)
	}
	got, ok := details["retry_after_seconds"].(float64)
	if !ok {
		t.Fatalf("error.details.retry_after_seconds missing or wrong type: %v", details)
	}
	if got != 30.0 {
		t.Errorf("retry_after_seconds = %v, want 30.0", got)
	}
	assertMetadata(t, m, stdout)
}

func assertRetryAfterAbsent(t *testing.T, stdout, _ string) {
	t.Helper()
	m := decodeEnvelope(t, stdout)
	if details, ok := errorObject(t, m, stdout)["details"].(map[string]any); ok {
		if _, has := details["retry_after_seconds"]; has {
			t.Errorf("retry_after_seconds should be absent when RetryAfter=0, got: %v", details)
		}
	}
	assertMetadata(t, m, stdout)
}

func assertTextSanitized(t *testing.T, _, stderr string) {
	t.Helper()
	if strings.ContainsAny(stderr, "\x1b\r") {
		t.Fatalf("text error output still contains raw ESC/CR bytes: %q", stderr)
	}
	if !strings.Contains(stderr, "FATAL: forged status line") {
		t.Fatalf("expected de-fanged message text to still be present, got %q", stderr)
	}
}

func assertJSONKeepsRawControlBytes(t *testing.T, stdout, _ string) {
	t.Helper()
	m := decodeEnvelope(t, stdout)
	msg, ok := errorObject(t, m, stdout)["message"].(string)
	if !ok {
		t.Fatalf("error.message missing or wrong type: %s", stdout)
	}
	if !strings.Contains(msg, string(rune(0x1b))) {
		t.Fatalf("round-tripping the JSON envelope must still yield the raw control byte "+
			"unmodified (sanitisation must not touch the JSON path), got %q", msg)
	}
}

func assertVerboseExcerptStaysOnStderr(t *testing.T, stdout, stderr string) {
	t.Helper()
	if strings.ContainsAny(stderr, "\x1b") {
		t.Fatalf("verbose stderr excerpt still contains raw ESC bytes: %q", stderr)
	}
	if !strings.Contains(stderr, "raw diagnostic body") {
		t.Fatalf("expected sanitized excerpt on stderr, got %q", stderr)
	}
	errObj := errorObject(t, decodeEnvelope(t, stdout), stdout)
	if msg, _ := errObj["message"].(string); strings.Contains(msg, "raw diagnostic body") {
		t.Fatalf("raw excerpt leaked into the JSON decision channel: %q", msg)
	}
	if _, hasDetails := errObj["details"]; hasDetails {
		t.Fatalf("raw excerpt must not be injected into error.details either, got: %v", errObj["details"])
	}
}

func assertExcerptOmitted(t *testing.T, _, stderr string) {
	t.Helper()
	if strings.Contains(stderr, "sensitive raw body") {
		t.Fatalf("non-verbose run must not print the raw excerpt, got stderr %q", stderr)
	}
}

func assertVerboseTextExcerpt(t *testing.T, _, stderr string) {
	t.Helper()
	if !strings.Contains(stderr, "diag body") {
		t.Fatalf("expected verbose excerpt in text-mode stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "Error:") {
		t.Fatalf("expected the normal Error: line to still be present, got %q", stderr)
	}
}

func assertTextRetryAfter(t *testing.T, _, stderr string) {
	t.Helper()
	if !strings.Contains(stderr, "30s") {
		t.Errorf("stderr text should carry the retry delay value, got: %q", stderr)
	}
	if !strings.Contains(stderr, "retry_after") {
		t.Errorf("stderr text should name the retry_after field, got: %q", stderr)
	}
}

func decodeEnvelope(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw=%s", err, stdout)
	}
	return m
}

func errorObject(t *testing.T, m map[string]any, stdout string) map[string]any {
	t.Helper()
	errObj, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("stdout must have an 'error' object, got: %s", stdout)
	}
	return errObj
}

func assertMetadata(t *testing.T, m map[string]any, stdout string) {
	t.Helper()
	meta, ok := m["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("envelope must have a 'metadata' object: %s", stdout)
	}
	if v, _ := meta["api_version"].(string); v == "" {
		t.Errorf("metadata.api_version is empty, want non-empty")
	}
	if v, _ := meta["request_id"].(string); v == "" {
		t.Errorf("metadata.request_id is empty, want non-empty")
	}
}

//nolint:paralleltest // stateless; consistent with existing pattern
func TestWriteErrorEnvelopeNoResponseJSON(t *testing.T) {
	io, _, stdout, _ := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    io,
		OutputFormat: output.FormatJSON,
		NewRequestID: func() string { return "req-test" },
	}

	inner := errcode.NoResponse(errcode.PeerAuthServer, errors.New("connection refused"))
	writeErrorEnvelope(f, fmt.Errorf("refresh access token: %w", inner))

	var m map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw=%s", err, stdout.Bytes())
	}
	errObj, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("no 'error' object: %s", stdout.Bytes())
	}
	if got, _ := errObj["code"].(string); got != errcode.CodeInternalError {
		t.Errorf("error.code = %q, want %q", got, errcode.CodeInternalError)
	}
	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("error.details must be an object: %s", stdout.Bytes())
	}
	if got, _ := details[errcode.DetailFailureStage].(string); got != errcode.FailureStageTransport {
		t.Errorf("error.details.failure_stage = %v, want %q", details[errcode.DetailFailureStage],
			errcode.FailureStageTransport)
	}
	if _, has := details["retry_after_seconds"]; has {
		t.Errorf("a transport failure must not acquire retry_after_seconds: %v", details)
	}
	if got := errcode.ExitCodeFor(fmt.Errorf("refresh access token: %w", inner)); got != errcode.GenericError {
		t.Errorf("exit = %d, want %d", got, errcode.GenericError)
	}
}

//nolint:paralleltest // stateless; consistent with existing pattern
func TestWriteErrorEnvelopeNoResponseText(t *testing.T) {
	io, _, _, stderr := iostreams.Test()
	f := &factory.Factory{
		IOStreams:    io,
		OutputFormat: output.FormatText,
		NewRequestID: func() string { return "req-test" },
	}

	writeErrorEnvelope(f, errcode.NoResponse(errcode.PeerAuthServer, errors.New("connection refused")))

	line := stderr.String()
	for _, phrase := range []string{"auth.weaviate.cloud", "api-cloud.weaviate.cloud", "wcloud guide",
		"only an administrator can change it"} {
		if !strings.Contains(line, phrase) {
			t.Errorf("text-mode stderr is missing %q\ngot: %s", phrase, line)
		}
	}
}
