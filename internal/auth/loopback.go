package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 2 * time.Second
)

type loopback struct {
	srv         *http.Server
	listener    net.Listener
	redirectURI string
	codeCh      chan string
	errCh       chan error
}

func startLoopback(ctx context.Context, expectedState string) (*loopback, error) {
	lc := &net.ListenConfig{}
	var listener net.Listener
	var addr string
	var lastErr error
	for _, port := range loopbackPorts {
		candidate := net.JoinHostPort(loopbackHost, strconv.Itoa(port))
		l, err := lc.Listen(ctx, "tcp", candidate)
		if err == nil {
			listener = l
			addr = candidate
			break
		}
		lastErr = err
	}
	if listener == nil {
		return nil, fmt.Errorf("loopback listen: every port in %v is busy (last error: %w)", loopbackPorts, lastErr)
	}

	codeCh := make(chan string)
	errCh := make(chan error, 1)
	srv := &http.Server{
		Handler:           callbackHandler(ctx, expectedState, codeCh, errCh),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	go func() {
		if serveErr := srv.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- fmt.Errorf("loopback serve: %w", serveErr)
		}
	}()

	return &loopback{
		srv:         srv,
		listener:    listener,
		redirectURI: "http://" + addr + loopbackPath,
		codeCh:      codeCh,
		errCh:       errCh,
	}, nil
}

func (l *loopback) awaitCode(ctx context.Context, r waitRenderer, interval time.Duration) (string, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			r.onDone()
			return "", ctx.Err()
		case waitErr := <-l.errCh:
			r.onDone()
			return "", waitErr
		case code := <-l.codeCh:
			r.onDone()
			return code, nil
		case <-ticker.C:
			r.onTick()
		}
	}
}

func (l *loopback) close() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	_ = l.srv.Shutdown(shutdownCtx)
	_ = l.listener.Close()
}

func callbackHandler(
	ctx context.Context,
	expectedState string,
	codeCh chan<- string,
	errCh chan<- error,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != loopbackPath {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			msg := fmt.Sprintf("authorization error: %s — %s", e, q.Get("error_description"))
			writeBrowserMessage(w, "Authentication failed", msg, false)
			errCh <- errors.New(msg)
			return
		}
		if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(expectedState)) != 1 {
			const msg = "state mismatch: this callback does not belong to the sign-in in progress"
			writeBrowserMessage(w, "Authentication failed", msg, false)
			errCh <- errors.New(msg)
			return
		}
		code := q.Get("code")
		if code == "" {
			writeBrowserMessage(w, "Authentication failed", "missing authorization code", false)
			errCh <- errors.New("missing authorization code")
			return
		}
		select {
		case codeCh <- code:
			writeBrowserMessage(w, "Authentication complete",
				"You can close this tab and return to your terminal.", true)
		case <-ctx.Done():
			writeBrowserMessage(w, "Sign-in window expired",
				"This sign-in window has expired. Run 'wcloud auth login' again in your terminal.", false)
		}
	})
}
