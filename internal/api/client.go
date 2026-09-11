package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"mime"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

const (
	defaultMaxRetries        = 3
	defaultBackoffInitial    = 200 * time.Millisecond
	defaultBackoffInitial429 = 1 * time.Second
	defaultBackoffMax        = 30 * time.Second
	defaultRetryAfterCap     = 60 * time.Second
	defaultHTTPTimeout       = 30 * time.Second
	// WHY: the server's own write budget can legitimately run past a minute, so a create must not be abandoned client-side before the server can finish it.
	defaultWriteHTTPTimeout = 90 * time.Second
	defaultVersion          = "dev"
	maxRawExcerptBytes      = 500
	maxResponseBytes        = 4 << 20
	maxRedirects            = 3
	maxRetryAfter           = 24 * time.Hour

	contentTypeJSON = "application/json"
)

type Client struct {
	endpoint     string
	token        string
	http         *http.Client
	writeHTTP    *http.Client
	newRequestID func() string
	maxRetries   int
	sleepFunc    func(context.Context, time.Duration) bool
	randInt64n   func(int64) int64
	policy       allowlist.Policy
	userAgent    string
}

type Option func(*Client)

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		c.http = h
		c.writeHTTP = h
	}
}

func WithWriteHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.writeHTTP = h }
}

func WithRequestIDFunc(f func() string) Option {
	return func(c *Client) { c.newRequestID = f }
}

func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

func WithSleepFunc(f func(context.Context, time.Duration) bool) Option {
	return func(c *Client) { c.sleepFunc = f }
}

func WithRandInt64n(f func(int64) int64) Option {
	return func(c *Client) { c.randInt64n = f }
}

func WithAllowlistPolicy(p allowlist.Policy) Option {
	return func(c *Client) { c.policy = p }
}

func WithVersion(version string) Option {
	return func(c *Client) { c.userAgent = userAgent(version) }
}

func userAgent(version string) string {
	return "wcloud/" + version + " (" + runtime.GOOS + ")"
}

func NewClient(endpoint, token string, opts ...Option) *Client {
	c := &Client{
		endpoint:     strings.TrimRight(endpoint, "/"),
		token:        token,
		http:         &http.Client{Timeout: defaultHTTPTimeout},
		writeHTTP:    &http.Client{Timeout: defaultWriteHTTPTimeout},
		newRequestID: uuid.NewString,
		maxRetries:   defaultMaxRetries,
		sleepFunc:    sleep,
		randInt64n:   rand.Int64N,
		policy:       allowlist.Production(),
		userAgent:    userAgent(defaultVersion),
	}
	for _, o := range opts {
		o(c)
	}
	// WHY: copied so a caller-supplied *http.Client gets the redirect policy without being mutated.
	c.http = c.withRedirectPolicy(c.http)
	c.writeHTTP = c.withRedirectPolicy(c.writeHTTP)
	return c
}

func (c *Client) withRedirectPolicy(h *http.Client) *http.Client {
	clone := *h
	clone.CheckRedirect = c.checkRedirect
	return &clone
}

func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if err := c.policy.Check(req.URL.String()); err != nil {
		return fmt.Errorf("refusing to follow redirect: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any, headers map[string]string) error {
	bodyBytes, err := encodeBody(body)
	if err != nil {
		return err
	}

	url := c.endpoint + "/v1" + path
	if policyErr := c.policy.Check(url); policyErr != nil {
		return &Error{
			Code:    errcode.CodeValidationFailed,
			Message: fmt.Sprintf("refusing to send request: %v", policyErr),
		}
	}
	requestID := c.newRequestID()

	// classifyAttempt guarantees done=true once attempt == c.maxRetries, so
	// this loop always returns from inside — no post-loop fallthrough exists.
	for attempt := 0; ; attempt++ {
		resp, sendErr := c.send(ctx, method, url, bodyBytes, requestID, headers)
		done, wait, terminalErr := c.classifyAttempt(ctx, attempt, method, resp, sendErr)
		if done {
			if terminalErr != nil {
				return terminalErr
			}
			return decodeResponse(resp, out)
		}
		if !c.sleepFunc(ctx, wait) {
			return ctx.Err()
		}
	}
}

// done=true means the caller should decode/return; false means sleep then retry.
func (c *Client) classifyAttempt(
	ctx context.Context, attempt int, method string, resp *http.Response, sendErr error,
) (bool, time.Duration, error) {
	if sendErr != nil {
		if ctx.Err() != nil {
			return true, 0, ctx.Err()
		}
		// WHY: a transport error cannot tell a refused connection from a timeout on an accepted request, so replaying a create could bill a second cluster.
		if !isRetryableMethod(method) || attempt == c.maxRetries {
			return true, 0, errcode.NoResponse(errcode.PeerCloudAPI, sendErr)
		}
		return false, c.jitteredBackoff(defaultBackoffInitial, attempt), nil
	}
	if shouldRetry(method, resp.StatusCode) && attempt < c.maxRetries {
		base := defaultBackoffInitial
		if resp.StatusCode == http.StatusTooManyRequests {
			base = defaultBackoffInitial429
		}
		computed := c.jitteredBackoff(base, attempt)
		w := retryAfter(resp.Header, computed)
		if w > defaultRetryAfterCap {
			// Retry-After exceeds cap: stop retrying; let decodeResponse handle the body.
			return true, 0, nil
		}
		// WHY: drained before closing so the connection can be reused for the retry.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		_ = resp.Body.Close()
		return false, w, nil
	}
	return true, 0, nil
}

func (c *Client) jitteredBackoff(base time.Duration, attempt int) time.Duration {
	ceiling := min(base*(1<<uint(attempt+1)), defaultBackoffMax)
	if ceiling <= 0 {
		return 0
	}
	return time.Duration(c.randInt64n(int64(ceiling)))
}

func encodeBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}
	return b, nil
}

func (c *Client) send(
	ctx context.Context,
	method, url string,
	body []byte,
	requestID string,
	headers map[string]string,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", contentTypeJSON)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("X-Request-Id", requestID)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	httpClient := c.http
	if isWriteMethod(method) {
		httpClient = c.writeHTTP
	}
	return httpClient.Do(req)
}

func decodeResponse(resp *http.Response, out any) error {
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if len(raw) > maxResponseBytes {
		return &Error{
			HTTPStatus: resp.StatusCode,
			Code:       errcode.CodeInternalError,
			Message: fmt.Sprintf(
				"unexpected response: HTTP %d returned a body larger than the %d bytes this client will read",
				resp.StatusCode, maxResponseBytes,
			),
		}
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return decodeError(resp.StatusCode, raw, resp.Header)
	}

	if out == nil {
		return nil
	}
	if jsonErr := json.Unmarshal(raw, out); jsonErr != nil {
		return &Error{
			HTTPStatus: resp.StatusCode,
			Code:       errcode.CodeInternalError,
			Message:    undecodableBodyMessage(resp.StatusCode, resp.Header.Get("Content-Type")),
			RawExcerpt: boundedExcerpt(raw),
		}
	}
	if v, ok := out.(payloadValidator); ok {
		if invalid := v.validate(); invalid != nil {
			return &Error{
				HTTPStatus: resp.StatusCode,
				Code:       errcode.CodeInternalError,
				Message:    fmt.Sprintf("unexpected response: HTTP %d %s", resp.StatusCode, invalid),
				RawExcerpt: boundedExcerpt(raw),
			}
		}
	}
	return nil
}

func undecodableBodyMessage(status int, contentType string) string {
	msg := fmt.Sprintf("unexpected response: HTTP %d returned a body that is not valid JSON", status)
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && mediaType != contentTypeJSON {
		msg += fmt.Sprintf(" (content-type: %s)", mediaType)
	}
	return msg
}

func decodeError(status int, raw []byte, header http.Header) error {
	retryAfterVal := surfacedRetryAfter(header)
	var env ErrorEnvelope
	if err := json.Unmarshal(raw, &env); err == nil && env.Error.Code != "" {
		return &Error{
			HTTPStatus: status,
			Code:       env.Error.Code,
			Message:    env.Error.Message,
			RequestID:  env.Metadata.RequestID,
			RetryAfter: retryAfterVal,
		}
	}
	return &Error{
		HTTPStatus: status,
		Code:       codeForStatus(status),
		Message: fmt.Sprintf(
			"unexpected response: HTTP %d returned a body that did not match the expected error format", status,
		),
		RawExcerpt: boundedExcerpt(raw),
		RetryAfter: retryAfterVal,
	}
}

// codeForStatus derives a frozen error code from the status when the body
// carries none, so an edge proxy's HTML 429 still exits as rate_limited.
func codeForStatus(status int) string {
	switch status {
	case http.StatusTooManyRequests:
		return errcode.CodeRateLimited
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return errcode.CodeServiceUnavailable
	case http.StatusUnauthorized:
		return errcode.CodeAuthRequired
	case http.StatusNotFound:
		return errcode.CodeClusterNotFound
	default:
		return errcode.CodeInternalError
	}
}

func boundedExcerpt(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) <= maxRawExcerptBytes {
		return s
	}
	return s[:maxRawExcerptBytes] + "... (truncated)"
}

// parseRetryAfter saturates at maxRetryAfter: an out-of-range integer would
// overflow the [time.Duration] multiply, and the saturated value still reads as
// "longer than we are willing to wait" to every caller.
func parseRetryAfter(h http.Header) (time.Duration, bool) {
	v := h.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
		if secs <= 0 {
			return 0, true
		}
		if secs > int64(maxRetryAfter/time.Second) {
			return maxRetryAfter, true
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(min(time.Until(t), maxRetryAfter), 0), true
	}
	return 0, false
}

func surfacedRetryAfter(h http.Header) time.Duration {
	d, ok := parseRetryAfter(h)
	if !ok {
		return 0
	}
	return min(d, defaultRetryAfterCap)
}

func shouldRetry(method string, status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
	default:
		return false
	}
	return isRetryableMethod(method)
}

func isWriteMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead
}

func isRetryableMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete:
		return true
	default:
		// WHY: a replayed POST can duplicate a billable cluster, so it must never be retried.
		return false
	}
}

func retryAfter(h http.Header, fallback time.Duration) time.Duration {
	d, ok := parseRetryAfter(h)
	if !ok {
		return fallback
	}
	return max(d, fallback)
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
