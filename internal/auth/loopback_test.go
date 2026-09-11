//nolint:testpackage // these tests cover unexported helpers per .claude/rules/testing.md
package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCallbackHandlerSuccessWritesTheSuccessPage(t *testing.T) {
	t.Parallel()

	codeCh := make(chan string)
	errCh := make(chan error, 1)
	srv := httptest.NewServer(callbackHandler(context.Background(), "state-xyz", codeCh, errCh))
	defer srv.Close()

	type result struct {
		body string
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		resp, err := http.Get(srv.URL + loopbackPath + "?code=AUTH_CODE&state=state-xyz")
		if err != nil {
			resCh <- result{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		raw, readErr := io.ReadAll(resp.Body)
		resCh <- result{body: string(raw), err: readErr}
	}()

	select {
	case code := <-codeCh:
		if code != "AUTH_CODE" {
			t.Fatalf("code = %q", code)
		}
	case e := <-errCh:
		t.Fatalf("unexpected err: %v", e)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for code")
	}

	res := <-resCh
	if res.err != nil {
		t.Fatalf("GET: %v", res.err)
	}
	if !strings.Contains(res.body, "Authentication complete") {
		t.Fatalf("expected the success page once a waiter took the code, got %q", res.body)
	}
}

func TestCallbackHandlerAfterWaitEndsShowsTheExpiredPage(t *testing.T) {
	t.Parallel()

	codeCh := make(chan string)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := httptest.NewServer(callbackHandler(ctx, "state-xyz", codeCh, errCh))
	defer srv.Close()

	resp, err := http.Get(srv.URL + loopbackPath + "?code=AUTH_CODE&state=state-xyz")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	body := string(raw)

	if strings.Contains(body, "Authentication complete") {
		t.Fatalf("a callback after the wait ended must not claim success, got %q", body)
	}
	if !strings.Contains(body, "expired") || !strings.Contains(body, "wcloud auth login") {
		t.Fatalf("expected the expired-window page naming the re-run command, got %q", body)
	}

	select {
	case code := <-codeCh:
		t.Fatalf("no code may be published after the wait ended, got %q", code)
	default:
	}
}

func TestCallbackHandlerStateMismatch(t *testing.T) {
	t.Parallel()

	codeCh := make(chan string)
	errCh := make(chan error, 1)
	h := callbackHandler(context.Background(), "expected", codeCh, errCh)

	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + loopbackPath + "?code=AUTH_CODE&state=wrong")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	_ = resp.Body.Close()

	select {
	case e := <-errCh:
		if !strings.Contains(e.Error(), "state mismatch") {
			t.Fatalf("err = %v, want state mismatch", e)
		}
	case <-codeCh:
		t.Fatal("expected error, got code")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for error")
	}
}

func TestCallbackHandlerStateMismatchDoesNotEchoStateValues(t *testing.T) {
	t.Parallel()

	const expected = "EXPECTED_STATE_VALUE"
	codeCh := make(chan string)
	errCh := make(chan error, 1)
	h := callbackHandler(context.Background(), expected, codeCh, errCh)

	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(
		srv.URL + loopbackPath + "?code=AUTH_CODE&state=" + url.QueryEscape("ATTACKER_STATE\x1b[2K"),
	)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	_ = resp.Body.Close()

	select {
	case e := <-errCh:
		if strings.Contains(e.Error(), expected) {
			t.Fatalf("the expected state is a session secret and must not be echoed, got %q", e)
		}
		if strings.Contains(e.Error(), "ATTACKER_STATE") || strings.Contains(e.Error(), "\x1b") {
			t.Fatalf("the received state is unsanitised input and must not be echoed, got %q", e)
		}
	case <-codeCh:
		t.Fatal("expected error, got code")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for error")
	}
}

func TestCallbackHandlerOAuthError(t *testing.T) {
	t.Parallel()

	codeCh := make(chan string)
	errCh := make(chan error, 1)
	h := callbackHandler(context.Background(), "any", codeCh, errCh)

	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + loopbackPath + "?error=access_denied&error_description=user_cancelled&state=any")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	_ = resp.Body.Close()

	select {
	case e := <-errCh:
		if !strings.Contains(e.Error(), "access_denied") {
			t.Fatalf("err = %v", e)
		}
	case <-codeCh:
		t.Fatal("expected error, got code")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for error")
	}
}

type countingRenderer struct {
	ticks int
	dones int
}

func (c *countingRenderer) onTick() { c.ticks++ }
func (c *countingRenderer) onDone() { c.dones++ }

func TestAwaitCodeTicksTheRendererWhileWaiting(t *testing.T) {
	t.Parallel()

	l := &loopback{
		codeCh: make(chan string),
		errCh:  make(chan error, 1),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	r := &countingRenderer{}
	if _, err := l.awaitCode(ctx, r, 20*time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if r.ticks < 2 {
		t.Fatalf("onTick fired %d times over ~6 intervals, want at least 2 — the ticker is not wired", r.ticks)
	}
	if r.dones != 1 {
		t.Fatalf("onDone fired %d times, want exactly 1", r.dones)
	}
}

func TestAwaitCodeRespectsCtx(t *testing.T) {
	t.Parallel()

	l := &loopback{
		codeCh: make(chan string),
		errCh:  make(chan error, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	r, interval := newWaitRenderer(&buf, "https://example.test/x", time.Now().Add(time.Minute), time.Now, false)
	_, err := l.awaitCode(ctx, r, interval)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestCallbackHandlerMissingCodeFailsTheLogin(t *testing.T) {
	t.Parallel()

	codeCh := make(chan string)
	errCh := make(chan error, 1)
	srv := httptest.NewServer(callbackHandler(context.Background(), "state-xyz", codeCh, errCh))
	defer srv.Close()

	go func() {
		resp, err := http.Get(srv.URL + loopbackPath + "?state=state-xyz")
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	select {
	case err := <-errCh:
		if !strings.Contains(err.Error(), "missing authorization code") {
			t.Fatalf("err = %v, want it to name the missing authorization code", err)
		}
	case code := <-codeCh:
		t.Fatalf("a callback with no code published %q instead of failing", code)
	case <-time.After(2 * time.Second):
		t.Fatal("a callback with no code neither failed nor published; the login would hang")
	}
}
