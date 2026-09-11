package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, srv *httptest.Server, opts ...api.Option) *api.Client {
	t.Helper()
	all := append([]api.Option{
		api.WithRequestIDFunc(func() string { return "req-fixed" }),
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
	}, opts...)
	return api.NewClient(srv.URL, "test-token", all...)
}

func TestClient_AttachesBearerAndRequestID(t *testing.T) {
	t.Parallel()
	var (
		gotAuth      string
		gotRequestID string
	)
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotRequestID = r.Header.Get("X-Request-Id")
		writeEnvelope(w, http.StatusOK, &api.WhoAmI{UserID: "u", Email: "e", OrgID: "o"}, "req-fixed")
	})

	c := newTestClient(t, srv)
	if _, err := c.Whoami(context.Background()); err != nil {
		t.Fatalf("whoami: %v", err)
	}

	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-token")
	}
	if gotRequestID != "req-fixed" {
		t.Errorf("X-Request-Id = %q, want %q", gotRequestID, "req-fixed")
	}
}

func TestClient_OmitsAuthorizationWhenTokenEmpty(t *testing.T) {
	t.Parallel()
	var gotAuth string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeEnvelope(w, http.StatusOK, &api.WhoAmI{UserID: "u"}, "req-fixed")
	})

	c := api.NewClient(srv.URL, "",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "req-fixed" }))
	if _, err := c.Whoami(context.Background()); err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty when token unset", gotAuth)
	}
}

func TestClient_RetriesOn5xx(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeEnvelope(w, http.StatusOK, &api.WhoAmI{UserID: "u"}, "req-fixed")
	})

	c := newTestClient(t, srv)
	got, err := c.Whoami(context.Background())
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3 (two 503s + one success)", calls.Load())
	}
	if got.UserID != "u" {
		t.Errorf("UserID = %q, want %q", got.UserID, "u")
	}
}

func TestClient_HonorsRetryAfterOn429(t *testing.T) {
	t.Parallel()
	var (
		calls    atomic.Int32
		stamps   []time.Time
		stampsMu = make(chan struct{}, 1)
	)
	stampsMu <- struct{}{}
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		<-stampsMu
		stamps = append(stamps, time.Now())
		stampsMu <- struct{}{}
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeEnvelope(w, http.StatusOK, &api.WhoAmI{UserID: "u"}, "req-fixed")
	})

	c := newTestClient(t, srv)
	if _, err := c.Whoami(context.Background()); err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
	if len(stamps) >= 2 {
		gap := stamps[1].Sub(stamps[0])
		if gap < 900*time.Millisecond {
			t.Errorf("client did not wait for Retry-After: gap=%s", gap)
		}
	}
}

func TestClient_DecodesErrorEnvelope(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(api.ErrorEnvelope{
			Error:    api.ErrorDetail{Code: "cluster_not_found", Message: "no such cluster"},
			Metadata: api.Metadata{APIVersion: "v1", RequestID: "req-fixed"},
		})
	})

	c := newTestClient(t, srv)
	_, err := c.GetCluster(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *api.Error", err)
	}
	if apiErr.Code != "cluster_not_found" {
		t.Errorf("code = %q, want cluster_not_found", apiErr.Code)
	}
	if apiErr.HTTPStatus != http.StatusNotFound {
		t.Errorf("status = %d, want 404", apiErr.HTTPStatus)
	}
	if apiErr.RequestID != "req-fixed" {
		t.Errorf("request_id = %q, want req-fixed", apiErr.RequestID)
	}
}

func TestDecodeError_QuotaExceededFreeTierLimit(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(api.ErrorEnvelope{
			Error: api.ErrorDetail{
				Code:    "quota_exceeded",
				Message: "free-tier cluster limit reached: delete your existing free-tier cluster to create a new one",
			},
			Metadata: api.Metadata{APIVersion: "v1", RequestID: "req-fixed"},
		})
	})

	c := newTestClient(t, srv,
		api.WithMaxRetries(0),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.CreateCluster(context.Background(), api.CreateClusterRequest{}, api.CreateClusterOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *api.Error", err)
	}
	if apiErr.Code != "quota_exceeded" {
		t.Errorf("Code = %q, want quota_exceeded", apiErr.Code)
	}
	if apiErr.HTTPStatus != http.StatusTooManyRequests {
		t.Errorf("HTTPStatus = %d, want 429", apiErr.HTTPStatus)
	}
	if apiErr.Message != "free-tier cluster limit reached: delete your existing free-tier cluster to create a new one" {
		t.Errorf("Message = %q", apiErr.Message)
	}
	if got := errcode.ExitCodeFor(err); got != errcode.QuotaExceeded {
		t.Errorf("ExitCodeFor = %d, want %d (QuotaExceeded)", got, errcode.QuotaExceeded)
	}
}

func TestClient_CreateClusterSendsIdempotencyKey(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotKey    string
		gotBody   api.CreateClusterRequest
	)
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotKey = r.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeEnvelope(w, http.StatusCreated,
			&api.Cluster{ID: "c-1", Name: "x", Status: api.StatusCreating}, "req-fixed")
	})

	c := newTestClient(t, srv)
	cluster, err := c.CreateCluster(context.Background(),
		api.CreateClusterRequest{Name: "x", Tier: api.TierFree},
		api.CreateClusterOptions{IdempotencyKey: "key-1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotKey != "key-1" {
		t.Errorf("Idempotency-Key = %q, want key-1", gotKey)
	}
	if gotBody.Name != "x" || gotBody.Tier != "free" {
		t.Errorf("body = %+v", gotBody)
	}
	if cluster.ID != "c-1" {
		t.Errorf("cluster id = %q, want c-1", cluster.ID)
	}
}

func TestClient_CreateClusterOmitsIdempotencyKeyWhenEmpty(t *testing.T) {
	t.Parallel()
	var gotKey string
	var keyPresent bool
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("Idempotency-Key")
		_, keyPresent = r.Header["Idempotency-Key"]
		writeEnvelope(w, http.StatusCreated, &api.Cluster{ID: "c-1"}, "req-fixed")
	})

	c := newTestClient(t, srv)
	if _, err := c.CreateCluster(context.Background(),
		api.CreateClusterRequest{}, api.CreateClusterOptions{}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if keyPresent || gotKey != "" {
		t.Errorf("Idempotency-Key sent unexpectedly: present=%v value=%q", keyPresent, gotKey)
	}
}

func TestClient_ListClusters(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.ListEnvelope[api.Cluster]{
			Data:     []api.Cluster{{ID: "c-1"}},
			Metadata: api.ListMetadata{APIVersion: "v1", RequestID: "req-fixed"},
		})
	})

	c := newTestClient(t, srv)
	got, err := c.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if gotPath != "/v1/clusters" {
		t.Errorf("path = %q, want /v1/clusters", gotPath)
	}
	if len(got) != 1 || got[0].ID != "c-1" {
		t.Errorf("data = %+v", got)
	}
}

func TestClient_GetClusterStatus(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeEnvelope(w, http.StatusOK, &api.ClusterStatusOnly{Status: api.StatusReady}, "req-fixed")
	})
	c := newTestClient(t, srv)
	got, err := c.GetClusterStatus(context.Background(), "c-1")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if got != api.StatusReady {
		t.Errorf("status = %q, want READY", got)
	}
}

func TestClient_ListRegions(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.ListEnvelope[api.Region]{
			Data:     []api.Region{{ID: "us-central1", IsDefault: true}},
			Metadata: api.ListMetadata{APIVersion: "v1", RequestID: "req-fixed"},
		})
	})
	c := newTestClient(t, srv)
	got, err := c.ListRegions(context.Background())
	if err != nil {
		t.Fatalf("regions: %v", err)
	}
	if len(got) != 1 || got[0].ID != "us-central1" || !got[0].IsDefault {
		t.Errorf("regions = %+v", got)
	}
}

func TestClient_GivesUpAfterMaxRetries(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusBadGateway)
	})
	c := newTestClient(t, srv, api.WithMaxRetries(2))
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected error after retry budget")
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3 (1 + 2 retries)", calls.Load())
	}
}

func writeEnvelope[T any](w http.ResponseWriter, status int, data *T, reqID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.Envelope[T]{
		Data:     *data,
		Metadata: api.Metadata{APIVersion: "v1", RequestID: reqID},
	})
}

func TestClient_JitteredBackoffSchedule(t *testing.T) {
	// Verify that randInt64n is called with doubling caps for successive 5xx retries.
	t.Parallel()
	var randArgs []int64

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // always 503
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(3),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(n int64) int64 {
			randArgs = append(randArgs, n)
			return n / 2 // deterministic: half the cap
		}),
	)
	_, _ = c.Whoami(context.Background())

	// 3 retries -> 3 calls to randInt64n.
	// base for 5xx = 200ms. caps: attempt 0->1: min(30s, 200ms*2^1)=400ms,
	// attempt 1->2: min(30s, 200ms*2^2)=800ms, attempt 2->3: min(30s, 200ms*2^3)=1600ms.
	want := []int64{
		int64(400 * time.Millisecond),
		int64(800 * time.Millisecond),
		int64(1600 * time.Millisecond),
	}
	if len(randArgs) != len(want) {
		t.Fatalf("randInt64n called %d times, want %d; args=%v", len(randArgs), len(want), randArgs)
	}
	for i, w := range want {
		if randArgs[i] != w {
			t.Errorf("randArgs[%d] = %d, want %d", i, randArgs[i], w)
		}
	}
}

func TestClient_429BackoffBase(t *testing.T) {
	// Verify 429 uses 1s base, not 200ms.
	t.Parallel()
	var randArgs []int64

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(2),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(n int64) int64 {
			randArgs = append(randArgs, n)
			return 0
		}),
	)
	_, _ = c.Whoami(context.Background())

	// 2 retries -> 2 randInt64n calls.
	// base for 429 = 1s. caps: attempt 0->1: min(30s,1s*2^1)=2s, attempt 1->2: min(30s,1s*2^2)=4s.
	want := []int64{
		int64(2 * time.Second),
		int64(4 * time.Second),
	}
	if len(randArgs) != len(want) {
		t.Fatalf("randInt64n called %d times, want %d; args=%v", len(randArgs), len(want), randArgs)
	}
	for i, w := range want {
		if randArgs[i] != w {
			t.Errorf("randArgs[%d] = %d, want %d", i, randArgs[i], w)
		}
	}
}

func TestClient_BackoffCappedAt30s(t *testing.T) {
	t.Parallel()
	var maxSleep time.Duration

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(10),
		api.WithSleepFunc(func(_ context.Context, d time.Duration) bool {
			if d > maxSleep {
				maxSleep = d
			}
			return true
		}),
		// Identity: return the cap itself (upper-bound of jitter range).
		api.WithRandInt64n(func(n int64) int64 { return n }),
	)
	_, _ = c.Whoami(context.Background())

	if maxSleep > 30*time.Second {
		t.Errorf("sleepFunc called with %s, want ≤ 30s", maxSleep)
	}
}

func TestClient_InjectableSleepAndRand(t *testing.T) {
	t.Parallel()
	var sleepCalled int
	var randCalled int
	var lastRandArg int64

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(1),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool {
			sleepCalled++
			return true
		}),
		api.WithRandInt64n(func(n int64) int64 {
			randCalled++
			lastRandArg = n
			return 0
		}),
	)
	_, _ = c.Whoami(context.Background())

	if sleepCalled == 0 {
		t.Error("sleepFunc was never called")
	}
	if randCalled == 0 {
		t.Error("randInt64n was never called")
	}
	if lastRandArg <= 0 {
		t.Errorf("randInt64n arg = %d, want > 0", lastRandArg)
	}
}

func TestClient_PostNotRetried(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
	}{
		{"429", http.StatusTooManyRequests},
		{"5xx", http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
			})

			c := api.NewClient(srv.URL, "tok",
				api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
				api.WithRequestIDFunc(func() string { return "r" }),
				api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
				api.WithRandInt64n(func(_ int64) int64 { return 0 }),
			)
			_, err := c.CreateCluster(context.Background(), api.CreateClusterRequest{}, api.CreateClusterOptions{})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if calls.Load() != 1 {
				t.Errorf("server calls = %d, want 1 (POST must not retry on %d)", calls.Load(), tc.status)
			}
		})
	}
}

func TestClient_GetRetriedOn429(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(3),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls.Load() != 4 {
		t.Errorf("server calls = %d, want 4 (1+3 retries for GET on 429)", calls.Load())
	}
}

func TestClient_GetRetriedOnRetryable5xx(t *testing.T) {
	t.Parallel()
	for _, status := range []int{500, 502, 503, 504} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
			})

			c := api.NewClient(srv.URL, "tok",
				api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
				api.WithRequestIDFunc(func() string { return "r" }),
				api.WithMaxRetries(3),
				api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
				api.WithRandInt64n(func(_ int64) int64 { return 0 }),
			)
			_, err := c.Whoami(context.Background())
			if err == nil {
				t.Fatal("expected error")
			}
			if calls.Load() != 4 {
				t.Errorf("status %d: calls = %d, want 4", status, calls.Load())
			}
		})
	}
}

func TestClient_NonRetryable5xxNotRetried(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotImplemented) // 501 — not in retryable set
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Errorf("501 should not be retried: calls = %d, want 1", calls.Load())
	}
}

func TestClient_ExactlyFourTotalAttempts(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(3),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 4 {
		t.Errorf("calls = %d, want exactly 4 (1 original + 3 retries)", calls.Load())
	}
}

func TestClient_PostNotRetriedOnTransportError(t *testing.T) {
	t.Parallel()
	var sleepCalls atomic.Int32

	// WHY: a closed server provokes a connection-refused transport error.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(2),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool {
			sleepCalls.Add(1)
			return true
		}),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.CreateCluster(context.Background(), api.CreateClusterRequest{}, api.CreateClusterOptions{})
	if err == nil {
		t.Fatal("expected error from the transport failure")
	}
	if sleepCalls.Load() != 0 {
		t.Errorf("sleepCalls = %d, want 0 (a POST is never replayed after a transport error)", sleepCalls.Load())
	}
}

func TestClient_GetStillRetriedOnTransportError(t *testing.T) {
	t.Parallel()
	var sleepCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(2),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool {
			sleepCalls.Add(1)
			return true
		}),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected error from the transport failure")
	}
	if sleepCalls.Load() != 2 {
		t.Errorf("sleepCalls = %d, want 2 (an idempotent GET keeps its transport retries)", sleepCalls.Load())
	}
}

func TestClient_PostNotReplayedAfterResponseTimeout(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})

	c := newTestClient(t, srv,
		api.WithHTTPClient(&http.Client{Timeout: 100 * time.Millisecond}),
		api.WithMaxRetries(3),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.CreateCluster(context.Background(),
		api.CreateClusterRequest{Tier: api.TierFree},
		api.CreateClusterOptions{IdempotencyKey: "key-1"})
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if calls.Load() != 1 {
		t.Errorf("POSTs on the wire = %d, want exactly 1 (a create must never be replayed)", calls.Load())
	}
}

func TestClient_WriteRequestsGetLongerTimeoutThanReads(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		wantErr bool
		call    func(c *api.Client) error
	}{
		{
			name:    "GET uses the read client and times out",
			wantErr: true,
			call: func(c *api.Client) error {
				_, err := c.Whoami(context.Background())
				return err
			},
		},
		{
			name: "POST uses the write client and succeeds",
			call: func(c *api.Client) error {
				_, err := c.CreateCluster(context.Background(), api.CreateClusterRequest{}, api.CreateClusterOptions{})
				return err
			},
		},
		{
			name: "PUT uses the write client and succeeds",
			call: func(c *api.Client) error {
				return c.DoForTesting(context.Background(), http.MethodPut)
			},
		},
		{
			name: "DELETE uses the write client and succeeds",
			call: func(c *api.Client) error {
				return c.DoForTesting(context.Background(), http.MethodDelete)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(150 * time.Millisecond)
				if r.Method == http.MethodPost {
					writeEnvelope(w, http.StatusCreated, &api.Cluster{ID: "c-1"}, "req-fixed")
					return
				}
				writeEnvelope(w, http.StatusOK, &api.WhoAmI{UserID: "u"}, "req-fixed")
			})
			c := newTestClient(t, srv,
				api.WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond}),
				api.WithWriteHTTPClient(&http.Client{Timeout: 500 * time.Millisecond}),
				api.WithMaxRetries(0),
			)

			err := tc.call(c)
			if tc.wantErr && err == nil {
				t.Fatal("expected the shorter read timeout to fire before the 150ms handler responds")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("%v, want success under the longer write timeout", err)
			}
		})
	}
}

func TestClient_RetryAfterInErrorOnExhaustion(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(3),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *api.Error", err)
	}
	if apiErr.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter = %s, want 5s", apiErr.RetryAfter)
	}
}

func TestClient_RetryAfterZeroWhenNoHeader(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(3),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *api.Error", err)
	}
	if apiErr.RetryAfter != 0 {
		t.Errorf("RetryAfter = %s, want 0 (no header)", apiErr.RetryAfter)
	}
}

func TestClient_RetryAfterBeatsJitter(t *testing.T) {
	t.Parallel()
	var sleptFor []time.Duration

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(1),
		api.WithSleepFunc(func(_ context.Context, d time.Duration) bool {
			sleptFor = append(sleptFor, d)
			return true
		}),
		api.WithRandInt64n(func(_ int64) int64 {
			return int64(500 * time.Millisecond) // jitter = 500ms < 5s Retry-After
		}),
	)
	_, _ = c.Whoami(context.Background())

	if len(sleptFor) != 1 {
		t.Fatalf("sleepFunc calls = %d, want 1", len(sleptFor))
	}
	if sleptFor[0] != 5*time.Second {
		t.Errorf("slept for %s, want 5s (Retry-After wins over jitter)", sleptFor[0])
	}
}

func TestClient_RetryAfterCapAt60s(t *testing.T) {
	t.Parallel()
	cases := []struct {
		header    string
		wantSleep time.Duration
	}{
		{"30", 30 * time.Second},
		{"60", 60 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			t.Parallel()
			var sleptFor []time.Duration

			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", tc.header)
				w.WriteHeader(http.StatusTooManyRequests)
			})

			c := api.NewClient(srv.URL, "tok",
				api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
				api.WithRequestIDFunc(func() string { return "r" }),
				api.WithMaxRetries(1),
				api.WithSleepFunc(func(_ context.Context, d time.Duration) bool {
					sleptFor = append(sleptFor, d)
					return true
				}),
				api.WithRandInt64n(func(_ int64) int64 { return 0 }),
			)
			_, _ = c.Whoami(context.Background())

			if len(sleptFor) != 1 || sleptFor[0] != tc.wantSleep {
				t.Errorf("slept for %v, want %s", sleptFor, tc.wantSleep)
			}
		})
	}
}

func TestClient_RetryAfterExceeds60sStopsRetry(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "61")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(3),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 (Retry-After > 60s must stop immediately)", calls.Load())
	}
}

// TestClient_JitterWinsOverSmallRetryAfter pins the max(computed, header) policy:
// when the computed jitter backoff exceeds the Retry-After header value, the larger
// computed value must be used so a large jitter is never discarded by a small header.
func TestClient_JitterWinsOverSmallRetryAfter(t *testing.T) {
	t.Parallel()
	var sleptFor []time.Duration

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "1") // server says wait 1s
		w.WriteHeader(http.StatusTooManyRequests)
	})

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(1),
		api.WithSleepFunc(func(_ context.Context, d time.Duration) bool {
			sleptFor = append(sleptFor, d)
			return true
		}),
		// jitter stub returns 8s — larger than the Retry-After: 1 header
		api.WithRandInt64n(func(_ int64) int64 {
			return int64(8 * time.Second)
		}),
	)
	_, _ = c.Whoami(context.Background())

	if len(sleptFor) != 1 {
		t.Fatalf("sleepFunc calls = %d, want 1", len(sleptFor))
	}
	if sleptFor[0] != 8*time.Second {
		t.Errorf("slept for %s, want 8s (jitter wins over small Retry-After: 1)", sleptFor[0])
	}
}

func TestClient_CreateClusterSendsProvisionedByHeader(t *testing.T) {
	t.Parallel()
	var gotHeader string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Provisioned-By")
		writeEnvelope(w, http.StatusCreated, &api.Cluster{ID: "c-pb"}, "req-fixed")
	})

	c := newTestClient(t, srv)
	if _, err := c.CreateCluster(context.Background(),
		api.CreateClusterRequest{},
		api.CreateClusterOptions{ProvisionedBy: "cli"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if gotHeader != "cli" {
		t.Errorf("X-Provisioned-By = %q, want cli", gotHeader)
	}
}

func TestDecodeError_MalformedBodyDoesNotReflectRawBytesIntoMessage(t *testing.T) {
	t.Parallel()
	payload := "not json, not the expected envelope shape, just a malformed vendor response body"
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(payload))
	})

	c := newTestClient(t, srv)
	_, err := c.GetCluster(context.Background(), "any")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *api.Error", err)
	}
	if strings.Contains(apiErr.Message, payload) {
		t.Fatalf("Message still reflects the raw response body verbatim: %q", apiErr.Message)
	}
	if !strings.Contains(apiErr.Message, "502") {
		t.Fatalf("Message should state the HTTP status it observed, got %q", apiErr.Message)
	}
	if apiErr.Code != errcode.CodeServiceUnavailable {
		t.Errorf("Code = %q, want %q (derived from the 502)", apiErr.Code, errcode.CodeServiceUnavailable)
	}
	if !strings.Contains(apiErr.RawExcerpt, payload) {
		t.Fatalf("RawExcerpt should retain the bounded original body for diagnosis, got %q", apiErr.RawExcerpt)
	}
}

func TestDecodeError_MalformedBodyBoundsExcerptLength(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("A", 5000)
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(huge))
	})

	c := newTestClient(t, srv)
	_, err := c.GetCluster(context.Background(), "any")
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *api.Error", err)
	}
	if len(apiErr.RawExcerpt) > 515 {
		t.Fatalf("RawExcerpt is not bounded: got %d bytes: %q", len(apiErr.RawExcerpt), apiErr.RawExcerpt)
	}
	if !strings.HasSuffix(apiErr.RawExcerpt, "(truncated)") {
		t.Fatalf("expected a truncation marker, got suffix %q", apiErr.RawExcerpt[len(apiErr.RawExcerpt)-20:])
	}
	if len(apiErr.Message) > 200 {
		t.Fatalf("Message must stay wcloud-authored and short, got %d bytes: %q", len(apiErr.Message), apiErr.Message)
	}
}

func TestDecodeError_MalformedBodyEmptyBodyHasEmptyExcerpt(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	c := newTestClient(t, srv)
	_, err := c.GetCluster(context.Background(), "any")
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *api.Error", err)
	}
	if apiErr.RawExcerpt != "" {
		t.Errorf("RawExcerpt = %q, want empty for an empty body", apiErr.RawExcerpt)
	}
}

func TestClient_CreateClusterOmitsProvisionedByWhenEmpty(t *testing.T) {
	t.Parallel()
	var headerPresent bool
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, headerPresent = r.Header["X-Provisioned-By"]
		writeEnvelope(w, http.StatusCreated, &api.Cluster{ID: "c-pb"}, "req-fixed")
	})

	c := newTestClient(t, srv)
	if _, err := c.CreateCluster(context.Background(),
		api.CreateClusterRequest{}, api.CreateClusterOptions{}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if headerPresent {
		t.Error("X-Provisioned-By header sent when ProvisionedBy is empty")
	}
}

func TestClient_RejectsZeroValuedClusterPayload(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{"null data", `{"data":null,"metadata":{"api_version":"v1","request_id":"req-fixed"}}`},
		{"empty object", `{}`},
		{"empty data object", `{"data":{},"metadata":{"api_version":"v1","request_id":"req-fixed"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})

			c := newTestClient(t, srv)
			got, err := c.GetCluster(context.Background(), "c-1")
			if err == nil {
				t.Fatalf("expected an error, got cluster %+v", got)
			}
			var apiErr *api.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("error type = %T (%v), want *api.Error", err, err)
			}
			if apiErr.Code != errcode.CodeInternalError {
				t.Errorf("Code = %q, want %q", apiErr.Code, errcode.CodeInternalError)
			}
			if !strings.Contains(apiErr.RawExcerpt, strings.TrimSpace(tc.body)) {
				t.Errorf("RawExcerpt = %q, want the bounded response body", apiErr.RawExcerpt)
			}
		})
	}
}

func TestClient_RejectsZeroValuedCreateResponse(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{},"metadata":{"api_version":"v1","request_id":"req-fixed"}}`))
	})

	c := newTestClient(t, srv)
	got, err := c.CreateCluster(context.Background(), api.CreateClusterRequest{}, api.CreateClusterOptions{})
	if err == nil {
		t.Fatalf("expected an error, got cluster %+v", got)
	}
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T (%v), want *api.Error", err, err)
	}
	if apiErr.Code != errcode.CodeInternalError {
		t.Errorf("Code = %q, want %q", apiErr.Code, errcode.CodeInternalError)
	}
}

func TestClient_RejectsEmptyClusterStatus(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{},"metadata":{"api_version":"v1","request_id":"req-fixed"}}`))
	})

	c := newTestClient(t, srv)
	got, err := c.GetClusterStatus(context.Background(), "c-1")
	if err == nil {
		t.Fatalf("expected an error, got status %q", got)
	}
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T (%v), want *api.Error", err, err)
	}
	if apiErr.Code != errcode.CodeInternalError {
		t.Errorf("Code = %q, want %q", apiErr.Code, errcode.CodeInternalError)
	}
}

func TestClient_RejectsZeroValuedWhoamiPayload(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{"null data", `{"data":null,"metadata":{"api_version":"v1","request_id":"req-fixed"}}`},
		{"empty data object", `{"data":{},"metadata":{"api_version":"v1","request_id":"req-fixed"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})

			c := newTestClient(t, srv)
			got, err := c.Whoami(context.Background())
			if err == nil {
				t.Fatalf("expected an error, got identity %+v", got)
			}
			var apiErr *api.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("error type = %T (%v), want *api.Error", err, err)
			}
			if apiErr.Code != errcode.CodeInternalError {
				t.Errorf("Code = %q, want %q", apiErr.Code, errcode.CodeInternalError)
			}
		})
	}
}

func TestClient_EmptyClusterListStaysSuccessful(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"metadata":{"api_version":"v1","request_id":"req-fixed"}}`))
	})

	c := newTestClient(t, srv)
	got, err := c.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("an empty list is a legitimate success, got error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("clusters = %+v, want none", got)
	}
}

func TestDecodeError_DerivesCodeFromStatusWhenBodyUnusable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status   int
		wantCode string
		wantExit int
	}{
		{http.StatusTooManyRequests, errcode.CodeRateLimited, errcode.RateLimited},
		{http.StatusBadGateway, errcode.CodeServiceUnavailable, errcode.ServiceUnavailable},
		{http.StatusServiceUnavailable, errcode.CodeServiceUnavailable, errcode.ServiceUnavailable},
		{http.StatusGatewayTimeout, errcode.CodeServiceUnavailable, errcode.ServiceUnavailable},
		{http.StatusUnauthorized, errcode.CodeAuthRequired, errcode.AuthRequired},
		{http.StatusNotFound, errcode.CodeClusterNotFound, errcode.NotFound},
		{http.StatusInternalServerError, errcode.CodeInternalError, errcode.GenericError},
	}
	const body = "<html><body>edge proxy says no</body></html>"
	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(body))
			})

			c := newTestClient(t, srv,
				api.WithMaxRetries(0),
				api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
				api.WithRandInt64n(func(_ int64) int64 { return 0 }),
			)
			_, err := c.GetCluster(context.Background(), "c-1")
			var apiErr *api.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("error type = %T (%v), want *api.Error", err, err)
			}
			if apiErr.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", apiErr.Code, tc.wantCode)
			}
			if got := errcode.ExitCodeFor(err); got != tc.wantExit {
				t.Errorf("exit code = %d, want %d", got, tc.wantExit)
			}
			if !strings.Contains(apiErr.RawExcerpt, body) {
				t.Errorf("RawExcerpt = %q, want the original body preserved", apiErr.RawExcerpt)
			}
		})
	}
}

func TestClient_SurfacedRetryAfterIsCapped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		header string
	}{
		{"overflowing seconds", "99999999999999"},
		{"seconds past int64 nanoseconds", "9223372037"},
		{"a plausible but hostile day", "86400"},
		{"far future date", "Fri, 31 Dec 9999 23:59:59 GMT"},
		{"just over the cap", "61"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", tc.header)
				w.WriteHeader(http.StatusTooManyRequests)
			})

			c := newTestClient(t, srv,
				api.WithMaxRetries(0),
				api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
				api.WithRandInt64n(func(_ int64) int64 { return 0 }),
			)
			_, err := c.Whoami(context.Background())
			var apiErr *api.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("error type = %T (%v), want *api.Error", err, err)
			}
			if apiErr.RetryAfter != 60*time.Second {
				t.Errorf("RetryAfter = %s, want 60s (the surfaced value is capped)", apiErr.RetryAfter)
			}
		})
	}
}

func TestClient_RejectsOversizeResponseBody(t *testing.T) {
	t.Parallel()
	chunk := []byte(strings.Repeat("A", 64*1024))
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		for range 128 {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	})

	c := newTestClient(t, srv)
	_, err := c.GetCluster(context.Background(), "c-1")
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T (%v), want *api.Error", err, err)
	}
	if apiErr.Code != errcode.CodeInternalError {
		t.Errorf("Code = %q, want %q", apiErr.Code, errcode.CodeInternalError)
	}
	if !strings.Contains(apiErr.Message, "large") {
		t.Errorf("Message = %q, want it to name the size limit", apiErr.Message)
	}
}

type hostRewriteTransport struct {
	host string
}

func (rt hostRewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = rt.host
	return http.DefaultTransport.RoundTrip(clone)
}

func TestClient_RefusesRedirectToDisallowedHost(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Location", "https://evil.example/v1/whoami")
		w.WriteHeader(http.StatusFound)
	})

	// WHY: strict policy + a dialing transport, so the request URL is allowed and the redirect target is not.
	c := api.NewClient("https://api-cloud.weaviate.cloud", "SECRET_TOKEN",
		api.WithHTTPClient(&http.Client{Transport: hostRewriteTransport{host: strings.TrimPrefix(srv.URL, "http://")}}),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(0),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected the redirect to a disallowed host to be refused")
	}
	cause := errors.Unwrap(err)
	if cause == nil || !strings.Contains(cause.Error(), "refusing to follow redirect") {
		t.Errorf("error = %v, want the redirect refusal reachable through the wrapped cause", err)
	}
	if calls.Load() != 1 {
		t.Errorf("requests = %d, want 1 (the redirect hop must not be followed)", calls.Load())
	}
	if strings.Contains(err.Error(), "SECRET_TOKEN") {
		t.Errorf("error message leaks the credential: %v", err)
	}
}

func TestClient_StopsAfterRedirectCap(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/v1/whoami", http.StatusFound)
	})

	c := newTestClient(t, srv,
		api.WithMaxRetries(0),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool { return true }),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected the redirect loop to be stopped")
	}
	if calls.Load() != 3 {
		t.Errorf("requests = %d, want 3 (the hop cap must stop the loop)", calls.Load())
	}
}

func TestClient_DecodeFailureIsWcloudAuthored(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		contentType string
		body        string
		wantInMsg   string
	}{
		{"captive portal html", "text/html", "<html><body>sign in to the wifi</body></html>", "text/html"},
		{"array where an object is expected", "application/json", `[{"id":"c-1"}]`, "JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				_, _ = w.Write([]byte(tc.body))
			})

			c := newTestClient(t, srv)
			_, err := c.GetCluster(context.Background(), "c-1")
			var apiErr *api.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("error type = %T (%v), want *api.Error", err, err)
			}
			if apiErr.Code != errcode.CodeInternalError {
				t.Errorf("Code = %q, want %q", apiErr.Code, errcode.CodeInternalError)
			}
			if strings.Contains(apiErr.Message, "api.Envelope") || strings.Contains(apiErr.Message, "internal/api") {
				t.Errorf("Message leaks an internal Go type: %q", apiErr.Message)
			}
			if !strings.Contains(apiErr.Message, tc.wantInMsg) {
				t.Errorf("Message = %q, want it to mention %q", apiErr.Message, tc.wantInMsg)
			}
			if !strings.Contains(apiErr.RawExcerpt, tc.body) {
				t.Errorf("RawExcerpt = %q, want the bounded response body", apiErr.RawExcerpt)
			}
		})
	}
}

func TestClient_SendsDefaultUserAgent(t *testing.T) {
	t.Parallel()
	var gotUA string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		writeEnvelope(w, http.StatusOK, &api.WhoAmI{UserID: "u"}, "req-fixed")
	})

	c := newTestClient(t, srv)
	if _, err := c.Whoami(context.Background()); err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if !strings.HasPrefix(gotUA, "wcloud/") || !strings.HasSuffix(gotUA, "("+runtime.GOOS+")") {
		t.Errorf("User-Agent = %q, want wcloud/<version> (%s)", gotUA, runtime.GOOS)
	}
}

func TestClient_SendsCallerSuppliedVersionInUserAgent(t *testing.T) {
	t.Parallel()
	var gotUA string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		writeEnvelope(w, http.StatusOK, &api.WhoAmI{UserID: "u"}, "req-fixed")
	})

	c := newTestClient(t, srv, api.WithVersion("1.2.3"))
	if _, err := c.Whoami(context.Background()); err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if want := "wcloud/1.2.3 (" + runtime.GOOS + ")"; gotUA != want {
		t.Errorf("User-Agent = %q, want %q", gotUA, want)
	}
}

func TestClient_DrainsRetriedBodyForConnectionReuse(t *testing.T) {
	t.Parallel()
	var (
		calls       atomic.Int32
		gotConns    atomic.Int32
		reusedConns atomic.Int32
		pooled      = make(chan struct{}, 1)
	)
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(strings.Repeat("B", 4096)))
			return
		}
		writeEnvelope(w, http.StatusOK, &api.WhoAmI{UserID: "u"}, "req-fixed")
	})

	// WHY: a dedicated pool, so a fresh connection on retry can only mean the undrained body shut the old one.
	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)

	c := newTestClient(t, srv,
		api.WithHTTPClient(&http.Client{Transport: transport}),
		api.WithMaxRetries(1),
		api.WithSleepFunc(func(_ context.Context, _ time.Duration) bool {
			select {
			case <-pooled:
			case <-time.After(2 * time.Second):
			}
			return true
		}),
		api.WithRandInt64n(func(_ int64) int64 { return 0 }),
	)

	ctx := httptrace.WithClientTrace(context.Background(), &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			gotConns.Add(1)
			if info.Reused {
				reusedConns.Add(1)
			}
		},
		PutIdleConn: func(_ error) {
			select {
			case pooled <- struct{}{}:
			default:
			}
		},
	})
	if _, err := c.Whoami(ctx); err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if calls.Load() != 2 || gotConns.Load() != 2 {
		t.Fatalf("requests = %d, connections acquired = %d, want 2 and 2", calls.Load(), gotConns.Load())
	}
	if reusedConns.Load() != 1 {
		t.Errorf("reused connections = %d, want 1 (the retried body must be drained so the connection is reused)",
			reusedConns.Load())
	}
}

func TestClient_NoResponseCarriesFailureStage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(0),
	)
	_, err := c.ListClusters(context.Background())
	if err == nil {
		t.Fatal("expected a transport error from a closed server")
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Fatalf("err type = %T, want *errcode.Error", err)
	}
	if got := e.Details[errcode.DetailFailureStage]; got != errcode.FailureStageTransport {
		t.Errorf("Details[%q] = %v, want %q", errcode.DetailFailureStage, got, errcode.FailureStageTransport)
	}
	if !strings.Contains(e.Message, "api-cloud.weaviate.cloud") {
		t.Errorf("message must name the API host: %s", e.Message)
	}
}

func TestClient_CancelledContextHasNoFailureStage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := api.NewClient(srv.URL, "tok",
		api.WithAllowlistPolicy(allowlist.NewPermissiveForTesting()),
		api.WithRequestIDFunc(func() string { return "r" }),
		api.WithMaxRetries(0),
	)
	_, err := c.ListClusters(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if _, ok := errcode.CodeFor(err); ok {
		t.Errorf("a cancelled command must not acquire an error code: %v", err)
	}
}
