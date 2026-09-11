package cluster_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/command/cluster"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory/mocks"
)

// makeClockSequence returns a nowFn that returns values in sequence,
// repeating the last value indefinitely.
func makeClockSequence(values ...time.Time) func() time.Time {
	i := 0
	return func() time.Time {
		v := values[i]
		if i < len(values)-1 {
			i++
		}
		return v
	}
}

func TestPollUntilReadyLineRendererSanitizesMaliciousStatus(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	malicious := api.ClusterStatus("CREATING\x1b[2K\r\x1b[32mSPOOFED\x1b[0m")
	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-mal").Return(malicious, nil).Once()
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-mal").Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-mal").
		Return(&api.Cluster{ID: "cid-mal", Status: api.StatusReady}, nil).Once()

	var buf bytes.Buffer
	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-mal", Deadline: farFuture,
		NowFn: func() time.Time { return t0 }, TickInterval: 0, Progress: &buf, StderrTTY: false,
	}
	if _, err := cluster.PollUntilReady(context.Background(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if strings.ContainsAny(out, "\x1b\r") {
		t.Fatalf("non-TTY poll output contains raw ESC/CR bytes from a malicious status: %q", out)
	}
	if !strings.Contains(out, "status: CREATINGSPOOFED\n") {
		t.Fatalf("expected sanitized status line, got %q", out)
	}
}

func TestPollUntilReadyTTYSpinnerSanitizesMaliciousStatusButKeepsOwnEraseSequence(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	malicious := api.ClusterStatus("CREATING\x1b[2K\r\x1b[32mSPOOFED\x1b[0m")
	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-mal-tty").Return(malicious, nil).Once()
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-mal-tty").Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-mal-tty").
		Return(&api.Cluster{ID: "cid-mal-tty", Status: api.StatusReady}, nil).Once()

	var buf bytes.Buffer
	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-mal-tty", Deadline: farFuture,
		NowFn: func() time.Time { return t0 }, TickInterval: 0, Progress: &buf, StderrTTY: true,
	}
	if _, err := cluster.PollUntilReady(context.Background(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "\x1b[K") {
		t.Fatalf("expected the CLI's OWN erase-to-EOL escape to survive sanitisation, got %q", out)
	}
	if strings.Contains(out, "\x1b[2K") || strings.Contains(out, "\x1b[32m") {
		t.Fatalf("malicious escape codes must not survive, got %q", out)
	}
	if !strings.Contains(out, "CREATINGSPOOFED") {
		t.Fatalf("expected sanitized status text present, got %q", out)
	}
}

func TestPollUntilReadyLineRendererHeartbeatEmitSanitizesMaliciousStatus(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	malicious := api.ClusterStatus("CREATING\x1b[2K\r\x1b[32mSPOOFED\x1b[0m")
	apiMock := mocks.NewMockAPIClient(t)
	for range 11 {
		apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-heartbeat").Return(malicious, nil).Once()
	}
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-heartbeat").Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-heartbeat").
		Return(&api.Cluster{ID: "cid-heartbeat", Status: api.StatusReady}, nil).Once()

	var buf bytes.Buffer
	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-heartbeat", Deadline: farFuture,
		NowFn: func() time.Time { return t0 }, TickInterval: 0, Progress: &buf, StderrTTY: false,
	}
	if _, err := cluster.PollUntilReady(context.Background(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if strings.ContainsAny(out, "\x1b\r") {
		t.Fatalf("non-TTY poll output contains raw ESC/CR bytes from a malicious status: %q", out)
	}
	want := "status: CREATINGSPOOFED\nstatus: CREATINGSPOOFED\nstatus: READY\n"
	if out != want {
		t.Fatalf("progress = %q, want %q", out, want)
	}
}

func TestPollUntilReadyTTYSpinnerOnDoneSanitizesMaliciousLastStatusOnError(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	malicious := api.ClusterStatus("CREATING\x1b[2K\r\x1b[32mSPOOFED\x1b[0m")
	pollErr := errors.New("transient poll failure")
	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-ondone-tty").Return(malicious, nil).Once()
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-ondone-tty").Return(api.ClusterStatus(""), pollErr).Once()

	var buf bytes.Buffer
	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-ondone-tty", Deadline: farFuture,
		NowFn: func() time.Time { return t0 }, TickInterval: 0, Progress: &buf, StderrTTY: true,
	}
	_, err := cluster.PollUntilReady(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected the second GetClusterStatus error to propagate")
	}
	out := buf.String()
	if !strings.Contains(out, "\x1b[K") {
		t.Fatalf("expected the CLI's OWN erase-to-EOL escape to survive in the onDone line, got %q", out)
	}
	if strings.Contains(out, "\x1b[2K") || strings.Contains(out, "\x1b[32m") {
		t.Fatalf("malicious escape codes must not survive in onDone's rendering of lastStatus, got %q", out)
	}
	if !strings.Contains(out, "status: CREATINGSPOOFED  elapsed:") {
		t.Fatalf("expected onDone to render sanitized lastStatus, got %q", out)
	}
}

//nolint:gocognit,nestif // table-driven poll helper tests require checking multiple error conditions per case
func TestPollUntilReady(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	cases := []struct {
		name               string
		statusSeq          []api.ClusterStatus
		clusterResult      *api.Cluster
		nowFn              func() time.Time
		deadline           time.Time
		cancelBefore       bool
		wantErr            bool
		wantErrCode        string
		wantLastStatus     string
		wantTerminalStatus string
	}{
		{
			name:      "ready after two status polls",
			statusSeq: []api.ClusterStatus{api.StatusCreating, api.StatusReady},
			clusterResult: &api.Cluster{
				ID:     "cid-1",
				Status: api.StatusReady,
				APIKey: &api.KeyInfo{Value: "secret-key"},
			},
			nowFn:    func() time.Time { return t0 },
			deadline: farFuture,
			wantErr:  false,
		},
		{
			name:               "failed immediate",
			statusSeq:          []api.ClusterStatus{api.StatusFailed},
			nowFn:              func() time.Time { return t0 },
			deadline:           farFuture,
			wantErr:            true,
			wantErrCode:        errcode.CodeInternalError,
			wantTerminalStatus: "FAILED",
		},
		{
			name:      "deleted immediate",
			statusSeq: []api.ClusterStatus{api.StatusDeleted},
			nowFn:     func() time.Time { return t0 },
			deadline:  farFuture,
			wantErr:   true,
		},
		{
			name:      "expired immediate",
			statusSeq: []api.ClusterStatus{api.StatusExpired},
			nowFn:     func() time.Time { return t0 },
			deadline:  farFuture,
			wantErr:   true,
		},
		{
			name:      "suspended immediate",
			statusSeq: []api.ClusterStatus{api.StatusSuspended},
			nowFn:     func() time.Time { return t0 },
			deadline:  farFuture,
			wantErr:   true,
		},
		{
			// nowFn: first call returns t0 (before deadline), second returns past deadline.
			// Flow: deadline computed from first call in RunE is separate; here we test
			// the poll loop itself. First iteration: t0 ≤ deadline → polls status (CREATING).
			// Second iteration: t0+1h > deadline → timeout.
			name:           "timeout with last status in error",
			statusSeq:      []api.ClusterStatus{api.StatusCreating},
			nowFn:          makeClockSequence(t0, t0.Add(time.Hour)),
			deadline:       t0.Add(time.Millisecond),
			wantErr:        true,
			wantErrCode:    errcode.CodeInternalError,
			wantLastStatus: "CREATING",
		},
		{
			name:         "context cancelled",
			statusSeq:    []api.ClusterStatus{},
			nowFn:        func() time.Time { return t0 },
			deadline:     farFuture,
			cancelBefore: true,
			wantErr:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			apiMock := mocks.NewMockAPIClient(t)
			for _, s := range tc.statusSeq {
				apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-1").Return(s, nil).Once()
			}
			if tc.clusterResult != nil {
				apiMock.EXPECT().GetCluster(mock.Anything, "cid-1").Return(tc.clusterResult, nil).Once()
			}

			ctx := context.Background()
			var cancel context.CancelFunc
			if tc.cancelBefore {
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			cfg := cluster.PollConfig{
				Client:       apiMock,
				ClusterID:    "cid-1",
				Deadline:     tc.deadline,
				NowFn:        tc.nowFn,
				TickInterval: 0,
			}

			got, err := cluster.PollUntilReady(ctx, cfg)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.wantErrCode != "" {
					var ec *errcode.Error
					if !errors.As(err, &ec) {
						t.Fatalf("error is not *errcode.Error: %T %v", err, err)
					}
					if ec.Code != tc.wantErrCode {
						t.Fatalf("error code = %q, want %q", ec.Code, tc.wantErrCode)
					}
					if tc.wantLastStatus != "" {
						if ec.Details == nil {
							t.Fatal("Details is nil, want last_status")
						}
						if v, ok := ec.Details["last_status"].(string); !ok || v != tc.wantLastStatus {
							t.Fatalf("Details[last_status] = %v, want %q", ec.Details["last_status"], tc.wantLastStatus)
						}
					}
					if tc.wantTerminalStatus != "" {
						if ec.Details == nil {
							t.Fatal("Details is nil, want terminal_status")
						}
						if v, ok := ec.Details["terminal_status"].(string); !ok || v != tc.wantTerminalStatus {
							t.Fatalf("Details[terminal_status] = %v, want %q",
								ec.Details["terminal_status"], tc.wantTerminalStatus)
						}
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("expected cluster, got nil")
			}
		})
	}
}

func TestPollUntilReadyProgress(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	t.Run("transition lines", func(t *testing.T) {
		t.Parallel()
		apiMock := mocks.NewMockAPIClient(t)
		apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-p").Return(api.StatusCreating, nil).Once()
		apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-p").Return(api.StatusReady, nil).Once()
		apiMock.EXPECT().
			GetCluster(mock.Anything, "cid-p").
			Return(&api.Cluster{ID: "cid-p", Status: api.StatusReady}, nil).
			Once()

		var buf bytes.Buffer
		cfg := cluster.PollConfig{
			Client: apiMock, ClusterID: "cid-p", Deadline: farFuture,
			NowFn: func() time.Time { return t0 }, TickInterval: 0, Progress: &buf,
		}
		if _, err := cluster.PollUntilReady(context.Background(), cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "status: CREATING\nstatus: READY\n"
		if buf.String() != want {
			t.Fatalf("progress = %q, want %q", buf.String(), want)
		}
	})

	t.Run("heartbeat on unchanged status", func(t *testing.T) {
		t.Parallel()
		apiMock := mocks.NewMockAPIClient(t)
		// 11 CREATING polls (1 initial + 10 unchanged, triggering one heartbeat at the
		// 10th unchanged poll), then READY.
		for range 11 {
			apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-h").Return(api.StatusCreating, nil).Once()
		}
		apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-h").Return(api.StatusReady, nil).Once()
		apiMock.EXPECT().
			GetCluster(mock.Anything, "cid-h").
			Return(&api.Cluster{ID: "cid-h", Status: api.StatusReady}, nil).
			Once()

		var buf bytes.Buffer
		cfg := cluster.PollConfig{
			Client: apiMock, ClusterID: "cid-h", Deadline: farFuture,
			NowFn: func() time.Time { return t0 }, TickInterval: 0, Progress: &buf,
		}
		if _, err := cluster.PollUntilReady(context.Background(), cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "status: CREATING\nstatus: CREATING\nstatus: READY\n"
		if buf.String() != want {
			t.Fatalf("progress = %q, want %q", buf.String(), want)
		}
	})
}

func TestPollUntilReadyImmediateFirstPoll(t *testing.T) {
	t.Parallel()

	const tickInterval = 300 * time.Millisecond
	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	apiMock := mocks.NewMockAPIClient(t)
	start := time.Now()
	var firstCallAt time.Duration
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-immediate").
		Run(func(_ context.Context, _ string) { firstCallAt = time.Since(start) }).
		Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-immediate").
		Return(&api.Cluster{ID: "cid-immediate", Status: api.StatusReady}, nil).Once()

	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-immediate", Deadline: farFuture,
		NowFn: func() time.Time { return t0 }, TickInterval: tickInterval,
	}
	if _, err := cluster.PollUntilReady(context.Background(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if firstCallAt >= tickInterval/2 {
		t.Fatalf("first GetClusterStatus call happened after %v, want well under %v — opening silence regressed",
			firstCallAt, tickInterval)
	}
}

func TestPollUntilReadyImmediateReadyNoTransition(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-degenerate").Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-degenerate").
		Return(&api.Cluster{ID: "cid-degenerate", Status: api.StatusReady}, nil).Once()

	var buf bytes.Buffer
	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-degenerate", Deadline: farFuture,
		NowFn: func() time.Time { return t0 }, TickInterval: 0, Progress: &buf,
	}
	if _, err := cluster.PollUntilReady(context.Background(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "status: READY\n"
	if buf.String() != want {
		t.Fatalf("progress = %q, want %q", buf.String(), want)
	}
}

func TestPollUntilReadyAnimationTicksSuppressedForNonTTY(t *testing.T) {
	t.Parallel()
	const tickInterval = 120 * time.Millisecond
	const animationInterval = 20 * time.Millisecond
	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-clean").Return(api.StatusCreating, nil).Once()
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-clean").Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-clean").
		Return(&api.Cluster{ID: "cid-clean", Status: api.StatusReady}, nil).Once()

	var buf bytes.Buffer
	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-clean", Deadline: farFuture,
		NowFn:             func() time.Time { return t0 },
		TickInterval:      tickInterval,
		AnimationInterval: animationInterval,
		Progress:          &buf,
		StderrTTY:         false,
	}
	if _, err := cluster.PollUntilReady(context.Background(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes.ContainsAny(buf.Bytes(), "\x1b\r") {
		t.Fatalf("non-TTY output contains control/escape bytes: %q", buf.String())
	}
	want := "status: CREATING\nstatus: READY\n"
	if buf.String() != want {
		t.Fatalf("progress = %q, want %q", buf.String(), want)
	}
}

func TestPollUntilReadyTTYAnimatesInPlace(t *testing.T) {
	t.Parallel()
	const tickInterval = 150 * time.Millisecond
	const animationInterval = 20 * time.Millisecond
	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-tty").Return(api.StatusCreating, nil).Once()
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-tty").Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-tty").
		Return(&api.Cluster{ID: "cid-tty", Status: api.StatusReady}, nil).Once()

	var buf bytes.Buffer
	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-tty", Deadline: farFuture,
		NowFn:             func() time.Time { return t0 },
		TickInterval:      tickInterval,
		AnimationInterval: animationInterval,
		Progress:          &buf,
		StderrTTY:         true,
	}
	if _, err := cluster.PollUntilReady(context.Background(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if strings.Count(out, "\r") < 2 {
		t.Fatalf("expected at least 2 carriage-return redraws, got %d in %q", strings.Count(out, "\r"), out)
	}
	if !strings.Contains(out, "\x1b[K") {
		t.Fatalf("expected erase-to-EOL escape sequence, got %q", out)
	}
	if out == "status: CREATING\nstatus: READY\n" {
		t.Fatal("TTY output must not be the plain discrete-line format")
	}
	lastCR := strings.LastIndex(out, "\r")
	if lastCR < 0 {
		t.Fatalf("expected at least one carriage return before the settled line, got %q", out)
	}
	settled := out[lastCR+1:]
	if !strings.HasPrefix(settled, "status: READY") {
		t.Fatalf(
			"expected settled line to start with \"status: READY\" (same field order as the live redraw), got %q",
			settled,
		)
	}
	if !strings.HasSuffix(settled, "\x1b[K\n") {
		t.Fatalf("expected settled line to end with erase + newline, got %q", settled)
	}
}

func TestPollUntilReadyTTYSettlesOnTerminalError(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-fail").Return(api.StatusFailed, nil).Once()

	var buf bytes.Buffer
	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-fail", Deadline: farFuture,
		NowFn: func() time.Time { return t0 }, TickInterval: 0,
		Progress: &buf, StderrTTY: true,
	}
	if _, err := cluster.PollUntilReady(context.Background(), cfg); err == nil {
		t.Fatal("expected error on FAILED status")
	}
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected settled output ending in newline, got %q", out)
	}
	if !strings.Contains(out, "FAILED") {
		t.Fatalf("expected settled line to mention FAILED status, got %q", out)
	}
}

func TestPollUntilReadyCancelDuringWait(t *testing.T) {
	t.Parallel()
	const tickInterval = 500 * time.Millisecond
	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)

	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-cancel").Return(api.StatusCreating, nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-cancel", Deadline: farFuture,
		NowFn: func() time.Time { return t0 }, TickInterval: tickInterval,
	}
	_, err := cluster.PollUntilReady(ctx, cfg)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error on context cancellation during wait")
	}
	if elapsed >= tickInterval {
		t.Fatalf("cancellation took %v, want well under the %v tick interval", elapsed, tickInterval)
	}
}

func detailsOf(t *testing.T, err error) map[string]any {
	t.Helper()
	var ec *errcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("error is not *errcode.Error: %T %v", err, err)
	}
	if ec.Details == nil {
		t.Fatalf("error details are nil: %v", err)
	}
	return ec.Details
}

func assertClusterID(t *testing.T, err error, want string) {
	t.Helper()
	details := detailsOf(t, err)
	got, ok := details["cluster_id"].(string)
	if !ok || got != want {
		t.Fatalf("details[cluster_id] = %v, want %q (an agent may not read the ID out of error.message)",
			details["cluster_id"], want)
	}
}

func TestPollUntilReadyFailuresCarryClusterID(t *testing.T) {
	t.Parallel()

	t0 := time.Unix(0, 0).UTC()
	farFuture := t0.Add(15 * time.Minute)
	pollErr := errors.New("decode status: unexpected shape")

	cases := []struct {
		name        string
		statusSeq   []api.ClusterStatus
		statusErr   error
		getErr      error
		nowFn       func() time.Time
		deadline    time.Time
		wantDetails map[string]any
	}{
		{
			name:      "timeout",
			statusSeq: []api.ClusterStatus{api.StatusCreating},
			nowFn:     makeClockSequence(t0, t0.Add(time.Hour)),
			deadline:  t0.Add(time.Millisecond),
			wantDetails: map[string]any{
				"last_status":        "CREATING",
				"still_provisioning": true,
			},
		},
		{
			name:        "terminal not ready",
			statusSeq:   []api.ClusterStatus{api.StatusFailed},
			nowFn:       func() time.Time { return t0 },
			deadline:    farFuture,
			wantDetails: map[string]any{"terminal_status": "FAILED"},
		},
		{
			name:        "poll failure on first status call",
			statusErr:   pollErr,
			nowFn:       func() time.Time { return t0 },
			deadline:    farFuture,
			wantDetails: map[string]any{"still_provisioning": true},
		},
		{
			name:      "poll failure after a status was seen",
			statusSeq: []api.ClusterStatus{api.StatusCreating},
			statusErr: pollErr,
			nowFn:     func() time.Time { return t0 },
			deadline:  farFuture,
			wantDetails: map[string]any{
				"last_status":        "CREATING",
				"still_provisioning": true,
			},
		},
		{
			name:        "get cluster fails after READY",
			statusSeq:   []api.ClusterStatus{api.StatusReady},
			getErr:      errors.New("boom"),
			nowFn:       func() time.Time { return t0 },
			deadline:    farFuture,
			wantDetails: map[string]any{"last_status": "READY"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			apiMock := mocks.NewMockAPIClient(t)
			for _, s := range tc.statusSeq {
				apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-orphan").Return(s, nil).Once()
			}
			if tc.statusErr != nil {
				apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-orphan").
					Return(api.ClusterStatus(""), tc.statusErr).Once()
			}
			if tc.getErr != nil {
				apiMock.EXPECT().GetCluster(mock.Anything, "cid-orphan").Return(nil, tc.getErr).Once()
			}

			cfg := cluster.PollConfig{
				Client: apiMock, ClusterID: "cid-orphan", Deadline: tc.deadline,
				NowFn: tc.nowFn, TickInterval: 0,
			}
			_, err := cluster.PollUntilReady(context.Background(), cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			assertClusterID(t, err, "cid-orphan")
			details := detailsOf(t, err)
			for key, want := range tc.wantDetails {
				if got := details[key]; got != want {
					t.Fatalf("details[%s] = %v, want %v", key, got, want)
				}
			}
		})
	}
}

func TestPollUntilReadyPollFailurePreservesUpstreamCode(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()

	apiErr := &api.Error{Code: errcode.CodeRateLimited, Message: "slow down", RetryAfter: 30 * time.Second}
	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-rl").Return(api.ClusterStatus(""), apiErr).Once()

	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-rl", Deadline: t0.Add(15 * time.Minute),
		NowFn: func() time.Time { return t0 }, TickInterval: 0,
	}
	_, err := cluster.PollUntilReady(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertClusterID(t, err, "cid-rl")
	if exit := errcode.ExitCodeFor(err); exit != errcode.RateLimited {
		t.Fatalf("exit code = %d, want %d — the upstream error code must survive the cluster-ID wrap", exit,
			errcode.RateLimited)
	}
	var gotAPIErr *api.Error
	if !errors.As(err, &gotAPIErr) {
		t.Fatalf("the upstream *api.Error must stay in the chain (retry_after depends on it), got %T", err)
	}
}

func TestPollUntilReadyCancellationCarriesClusterID(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()

	apiMock := mocks.NewMockAPIClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-ctrlc", Deadline: t0.Add(15 * time.Minute),
		NowFn: func() time.Time { return t0 }, TickInterval: 0,
	}
	_, err := cluster.PollUntilReady(ctx, cfg)
	if err == nil {
		t.Fatal("expected error on a cancelled context")
	}
	assertClusterID(t, err, "cid-ctrlc")
	if got := detailsOf(t, err)["still_provisioning"]; got != true {
		t.Fatalf("details[still_provisioning] = %v, want true", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("context.Canceled must stay in the chain, got %v", err)
	}
	if code, ok := errcode.CodeFor(err); !ok || code != errcode.CodeInternalError {
		t.Fatalf("code = %q (ok=%v), want %q", code, ok, errcode.CodeInternalError)
	}
}

func TestPollUntilReadyCancelDuringWaitCarriesClusterID(t *testing.T) {
	t.Parallel()
	const tickInterval = 500 * time.Millisecond
	t0 := time.Unix(0, 0).UTC()

	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-ctrlc-wait").Return(api.StatusCreating, nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-ctrlc-wait", Deadline: t0.Add(15 * time.Minute),
		NowFn: func() time.Time { return t0 }, TickInterval: tickInterval,
	}
	_, err := cluster.PollUntilReady(ctx, cfg)
	if err == nil {
		t.Fatal("expected error on cancellation during the inter-poll wait")
	}
	assertClusterID(t, err, "cid-ctrlc-wait")
	details := detailsOf(t, err)
	if got := details["last_status"]; got != "CREATING" {
		t.Fatalf("details[last_status] = %v, want CREATING", got)
	}
	if got := details["still_provisioning"]; got != true {
		t.Fatalf("details[still_provisioning] = %v, want true", got)
	}
}

func TestPollUntilReadyTimeoutNamesUnrecognizedStatus(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()

	for _, status := range []api.ClusterStatus{api.ClusterStatus("MIGRATING"), api.StatusUnknown} {
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()

			// WHY: an unrecognized status must keep polling, since a future backend value may still reach READY.
			apiMock := mocks.NewMockAPIClient(t)
			apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-future").Return(status, nil).Twice()

			cfg := cluster.PollConfig{
				Client: apiMock, ClusterID: "cid-future", Deadline: t0.Add(time.Millisecond),
				NowFn: makeClockSequence(t0, t0, t0.Add(time.Hour)), TickInterval: 0,
			}
			_, err := cluster.PollUntilReady(context.Background(), cfg)
			if err == nil {
				t.Fatal("expected a timeout error")
			}
			assertClusterID(t, err, "cid-future")
			details := detailsOf(t, err)
			if got := details["unrecognized_status"]; got != string(status) {
				t.Fatalf("details[unrecognized_status] = %v, want %q — a timeout on a status the CLI "+
					"does not understand must not read as an ordinary timeout", got, status)
			}
			if got := details["last_status"]; got != string(status) {
				t.Fatalf("details[last_status] = %v, want %q", got, status)
			}
		})
	}
}

func TestPollUntilReadySpinnerElapsedUsesInjectedClock(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()

	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-elapsed").Return(api.StatusCreating, nil).Once()
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-elapsed").Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-elapsed").
		Return(&api.Cluster{ID: "cid-elapsed", Status: api.StatusReady}, nil).Once()

	// WHY: clock order is renderer start, deadline check, CREATING redraw, deadline check, READY redraw, then the settled line.
	clock := makeClockSequence(t0, t0, t0.Add(3*time.Second), t0, t0.Add(7*time.Second))

	var buf bytes.Buffer
	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-elapsed", Deadline: t0.Add(15 * time.Minute),
		NowFn: clock, TickInterval: 0, Progress: &buf, StderrTTY: true,
	}
	if _, err := cluster.PollUntilReady(context.Background(), cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "status: CREATING  elapsed: 3s") {
		t.Fatalf("expected the injected clock's elapsed time on the first redraw, got %q", out)
	}
	if !strings.HasSuffix(out, "status: READY  elapsed: 7s\x1b[K\n") {
		t.Fatalf("expected the settled line to carry the injected clock's elapsed time, got %q", out)
	}
}

func TestPollUntilReadyTimeoutOnKnownStatusHasNoUnrecognizedDetail(t *testing.T) {
	t.Parallel()
	t0 := time.Unix(0, 0).UTC()

	apiMock := mocks.NewMockAPIClient(t)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-known").Return(api.StatusCreating, nil).Once()

	cfg := cluster.PollConfig{
		Client: apiMock, ClusterID: "cid-known", Deadline: t0.Add(time.Millisecond),
		NowFn: makeClockSequence(t0, t0.Add(time.Hour)), TickInterval: 0,
	}
	_, err := cluster.PollUntilReady(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if _, present := detailsOf(t, err)["unrecognized_status"]; present {
		t.Fatal("CREATING is a known status; details must not claim it is unrecognized")
	}
}
