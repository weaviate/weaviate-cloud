package cluster_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/cmdtest"
	"github.com/weaviate/weaviate-cloud/internal/command/cluster"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
)

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateWait(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	f.PollInterval = 0

	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Return(&api.Cluster{ID: "cid-1"}, nil)
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-1").
		Return(api.StatusCreating, nil).Once()
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-1").
		Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().
		GetCluster(mock.Anything, "cid-1").
		Return(&api.Cluster{
			ID:     "cid-1",
			Status: api.StatusReady,
			APIKey: &api.KeyInfo{Value: "secret-key"},
		}, nil).Times(1)

	if err := cmdtest.Run(t, f, "cluster", "create", "--wait"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	env := cmdtest.DecodeEnvelope(t, stdout)
	var got api.Cluster
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if got.ID != "cid-1" {
		t.Fatalf("id = %q, want cid-1", got.ID)
	}
	if got.APIKey == nil || got.APIKey.Value != "secret-key" {
		t.Fatalf("api_key.value = %v, want secret-key", got.APIKey)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateWaitFailed(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	f.PollInterval = 0

	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Return(&api.Cluster{ID: "cid-2"}, nil)
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-2").
		Return(api.StatusFailed, nil).Once()

	err := cmdtest.Run(t, f, "cluster", "create", "--wait")
	if err == nil {
		t.Fatal("expected error on FAILED status")
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errcode.Error, got %T %v", err, err)
	}
	if errcode.ExitCodeFor(err) == errcode.Success {
		t.Fatal("expected non-zero exit code")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateWaitTimeout(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	f.PollInterval = 0

	// Clock: first call computes deadline (t0 + 1ms), second call (in pollUntilReady first
	// iteration) is before deadline, third call is past deadline.
	t0 := time.Unix(0, 0).UTC()
	var callCount atomic.Int64
	f.Now = func() time.Time {
		n := callCount.Add(1)
		if n <= 2 {
			return t0
		}
		return t0.Add(time.Hour)
	}

	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Return(&api.Cluster{ID: "cid-3"}, nil)
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-3").
		Return(api.StatusCreating, nil).Once()

	err := cmdtest.Run(t, f, "cluster", "create", "--wait", "--timeout", "1ms")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errcode.Error, got %T %v", err, err)
	}
	if e.Code != errcode.CodeInternalError {
		t.Fatalf("code = %q, want %q", e.Code, errcode.CodeInternalError)
	}
	if errcode.ExitCodeFor(err) == errcode.Success {
		t.Fatal("expected non-zero exit code")
	}
	// Verify Details carries last_status.
	if e.Details == nil {
		t.Fatal("Details is nil, want last_status")
	}
	if v, ok := e.Details["last_status"].(string); !ok || v != "CREATING" {
		t.Fatalf("Details[last_status] = %v, want CREATING", e.Details["last_status"])
	}
	// Stdout is empty here: cmdtest.Run returns the error from RunE directly and
	// never calls writeErrorEnvelope. The JSON serialization of the error envelope
	// (including error.details.last_status and the absence of a "data" key) is
	// covered by TestWriteErrorEnvelopeDetails in cmd/wcloud/main_test.go.
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateWaitHonorsTimeoutFlag(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	f.PollInterval = 0

	// WHY: the clock passes the 500ms deadline before the first poll, so an ignored --timeout would keep polling on the 15m default.
	t0 := time.Unix(0, 0).UTC()
	var calls atomic.Int64
	f.Now = func() time.Time {
		if calls.Add(1) == 1 {
			return t0
		}
		return t0.Add(time.Second)
	}

	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Return(&api.Cluster{ID: "cid-timeout-flag"}, nil)

	err := cmdtest.Run(t, f, "cluster", "create", "--wait", "--timeout", "500ms")
	if err == nil {
		t.Fatal("expected --timeout=500ms to expire before the first poll")
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errcode.Error, got %T %v", err, err)
	}
	if got, ok := e.Details["cluster_id"].(string); !ok || got != "cid-timeout-flag" {
		t.Fatalf("details[cluster_id] = %v, want cid-timeout-flag", e.Details["cluster_id"])
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateNotAuthenticated(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, false)

	err := cmdtest.Run(t, f, "cluster", "create")
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want CodeAuthRequired — create provisions billable infrastructure", err)
	}
	if exit := errcode.ExitCodeFor(err); exit != errcode.AuthRequired {
		t.Fatalf("exit code = %d, want %d (AuthRequired)", exit, errcode.AuthRequired)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateUnconfirmedOutcomeReportsIdempotencyKey(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)

	var sentKey string
	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ api.CreateClusterRequest, o api.CreateClusterOptions) {
			sentKey = o.IdempotencyKey
		}).
		Return(nil, errors.New(`transport: Post "https://example.com/v1/clusters": context deadline exceeded`))

	err := cmdtest.Run(t, f, "cluster", "create")
	if err == nil {
		t.Fatal("expected the transport failure to propagate")
	}
	if sentKey == "" {
		t.Fatal("create sent an empty Idempotency-Key")
	}
	var e *errcode.Error
	if !errors.As(err, &e) {
		t.Fatalf("expected *errcode.Error, got %T %v", err, err)
	}
	if got, _ := e.Details["idempotency_key"].(string); got != sentKey {
		t.Fatalf("details[idempotency_key] = %v, want %q — a deliberate retry cannot deduplicate without the key",
			e.Details["idempotency_key"], sentKey)
	}
	if got := e.Details["cluster_may_exist"]; got != true {
		t.Fatalf("details[cluster_may_exist] = %v, want true — the POST may have been accepted", got)
	}
	if !strings.Contains(err.Error(), "cluster list") {
		t.Fatalf("error = %q, want it to name 'cluster list' as the reconciliation step", err.Error())
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateRejectedRequestKeepsItsUpstreamError(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)

	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &api.Error{HTTPStatus: 400, Code: errcode.CodeValidationFailed, Message: "unknown tier"})

	err := cmdtest.Run(t, f, "cluster", "create", "--tier", "nope")
	if err == nil {
		t.Fatal("expected the rejection to propagate")
	}
	if exit := errcode.ExitCodeFor(err); exit != errcode.UsageError {
		t.Fatalf("exit code = %d, want %d (UsageError)", exit, errcode.UsageError)
	}
	var e *errcode.Error
	if errors.As(err, &e) {
		t.Fatalf("a server-rejected create is a confirmed outcome and must not be dressed as an "+
			"ambiguous one: details = %v", e.Details)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateWaitCancellation(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	f.PollInterval = 0

	ctx, cancel := context.WithCancel(context.Background())

	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ api.CreateClusterRequest, _ api.CreateClusterOptions) {
			cancel()
		}).
		Return(&api.Cluster{ID: "cid-4"}, nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"cluster", "create", "--wait", "-o", "json"})
	err := root.ExecuteContext(ctx)
	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
}

// WHY: --timeout's own help text says it requires --wait; passing it alone used to be
// silently accepted and ignored (no CreateCluster mock expectation set here — a call
// would fail the test) rather than rejected, contradicting that help text.
//
//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateTimeoutWithoutWait(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, true)

	err := cmdtest.Run(t, f, "cluster", "create", "--timeout", "5s")
	if err == nil {
		t.Fatal("expected --timeout without --wait to be rejected")
	}
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeValidationFailed {
		t.Fatalf("err = %v, want CodeValidationFailed", err)
	}
	if exit := errcode.ExitCodeFor(err); exit != errcode.UsageError {
		t.Fatalf("exit code = %d, want %d (UsageError)", exit, errcode.UsageError)
	}
	if !strings.Contains(err.Error(), "--wait") {
		t.Fatalf("message = %q, want it to name the missing --wait", err.Error())
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreate_ProvisionedBy_DefaultCLI(t *testing.T) {
	for _, key := range []string{
		"CLAUDECODE", "CLAUDE_CODE", "CURSOR_AGENT", "GEMINI_CLI",
		"AGENT",
		"CODEX", "OPENAI_CODEX", "AIDER", "CLINE", "WINDSURF_AGENT",
		"GITHUB_COPILOT", "AMAZON_Q", "AWS_Q_DEVELOPER", "GEMINI_CODE_ASSIST",
		"OPENCODE", "SRC_CODY", "PI_CODING_AGENT", "FORCE_AGENT_MODE",
	} {
		t.Setenv(key, "")
	}
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.MatchedBy(func(o api.CreateClusterOptions) bool {
			return o.ProvisionedBy == "cli"
		})).
		Return(&api.Cluster{ID: "c-pb"}, nil)

	if err := cmdtest.Run(t, f, "cluster", "create"); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestClusterCreate_ProvisionedBy_AgentMode(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.MatchedBy(func(o api.CreateClusterOptions) bool {
			return o.ProvisionedBy == "agent"
		})).
		Return(&api.Cluster{ID: "c-agent"}, nil)

	if err := cmdtest.Run(t, f, "cluster", "create"); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateWaitTerminalStates(t *testing.T) {
	terminalStates := []api.ClusterStatus{
		api.StatusDeleted,
		api.StatusExpired,
		api.StatusSuspended,
	}
	for _, status := range terminalStates {
		t.Run(string(status), func(t *testing.T) {
			//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
			f, apiMock, _ := cmdtest.NewFactory(t, true)
			f.PollInterval = 0

			apiMock.EXPECT().
				CreateCluster(mock.Anything, mock.Anything, mock.Anything).
				Return(&api.Cluster{ID: "cid-term"}, nil)
			apiMock.EXPECT().
				GetClusterStatus(mock.Anything, "cid-term").
				Return(status, nil).Once()

			err := cmdtest.Run(t, f, "cluster", "create", "--wait")
			if err == nil {
				t.Fatalf("status %s: expected non-nil error", status)
			}
		})
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateWaitShortFlag(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	f.PollInterval = 0

	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Return(&api.Cluster{ID: "cid-w"}, nil)
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-w").
		Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().
		GetCluster(mock.Anything, "cid-w").
		Return(&api.Cluster{ID: "cid-w", Status: api.StatusReady}, nil).Times(1)

	if err := cmdtest.Run(t, f, "cluster", "create", "-w"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	env := cmdtest.DecodeEnvelope(t, stdout)
	var got api.Cluster
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if got.ID != "cid-w" {
		t.Fatalf("id = %q, want cid-w", got.ID)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateAwaitFlagRemoved(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, true)

	err := cmdtest.Run(t, f, "cluster", "create", "--await")
	if err == nil {
		t.Fatal("expected error: --await should no longer be a recognized flag")
	}
	if !strings.Contains(err.Error(), "unknown flag: --await") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "unknown flag: --await")
	}
}

func TestClusterCreateWaitFlagDefinitions(t *testing.T) {
	t.Parallel()
	f := &factory.Factory{}
	cmd := cluster.NewCreateCmd(f)

	waitFlag := cmd.Flags().Lookup("wait")
	if waitFlag == nil {
		t.Fatal("expected --wait flag to be registered")
	}
	if waitFlag.Shorthand != "w" {
		t.Fatalf("--wait shorthand = %q, want %q", waitFlag.Shorthand, "w")
	}
	if waitFlag.DefValue != "false" {
		t.Fatalf("--wait default = %q, want %q", waitFlag.DefValue, "false")
	}
	if cmd.Flags().Lookup("await") != nil {
		t.Fatal("expected --await flag to be gone")
	}
	timeoutFlag := cmd.Flags().Lookup("timeout")
	if timeoutFlag == nil || !strings.Contains(timeoutFlag.Usage, "requires --wait") {
		t.Fatalf("--timeout usage = %q, want it to contain %q", timeoutFlag.Usage, "requires --wait")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateWaitProgressStderr(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	f.PollInterval = 0
	stderr, ok := f.IOStreams.Err.(*bytes.Buffer)
	if !ok {
		t.Fatalf(
			"f.IOStreams.Err is %T, want *bytes.Buffer (cmdtest.NewFactory should build via iostreams.Test())",
			f.IOStreams.Err,
		)
	}

	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Return(&api.Cluster{ID: "cid-s"}, nil)
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-s").
		Return(api.StatusCreating, nil).Once()
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-s").
		Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().
		GetCluster(mock.Anything, "cid-s").
		Return(&api.Cluster{ID: "cid-s", Status: api.StatusReady, APIKey: &api.KeyInfo{Value: "k"}}, nil).Times(1)

	if err := cmdtest.Run(t, f, "cluster", "create", "--wait"); err != nil {
		t.Fatalf("execute: %v", err)
	}

	wantErr := "status: CREATING\nstatus: READY\n"
	if stderr.String() != wantErr {
		t.Fatalf("stderr = %q, want %q", stderr.String(), wantErr)
	}
	if bytes.ContainsAny(stderr.Bytes(), "\x1b\r") {
		t.Fatalf("stderr contains control/escape bytes: %q", stderr.String())
	}
	if strings.Contains(stdout.String(), "status:") {
		t.Fatalf("stdout leaked a progress line: %q", stdout.String())
	}
	env := cmdtest.DecodeEnvelope(t, stdout)
	var got api.Cluster
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if got.APIKey == nil || got.APIKey.Value != "k" {
		t.Fatalf("api_key.value = %v, want k", got.APIKey)
	}
}

func TestClusterCreateWaitTTYSpinner(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("TERM", "xterm-256color")
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	f.PollInterval = 200 * time.Millisecond
	f.IOStreams.SetStderrTTY(true)
	stderr, ok := f.IOStreams.Err.(*bytes.Buffer)
	if !ok {
		t.Fatalf("f.IOStreams.Err is %T, want *bytes.Buffer", f.IOStreams.Err)
	}

	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Return(&api.Cluster{ID: "cid-spin"}, nil)
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-spin").
		Return(api.StatusCreating, nil).Once()
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-spin").
		Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().
		GetCluster(mock.Anything, "cid-spin").
		Return(&api.Cluster{ID: "cid-spin", Status: api.StatusReady, APIKey: &api.KeyInfo{Value: "k"}}, nil).
		Times(1)

	if err := cmdtest.Run(t, f, "cluster", "create", "--wait"); err != nil {
		t.Fatalf("execute: %v", err)
	}

	out := stderr.String()
	if !strings.Contains(out, "\r") {
		t.Fatalf("expected TTY stderr to contain carriage-return redraws, got %q", out)
	}
	if strings.Contains(out, "status: CREATING\nstatus: READY\n") {
		t.Fatal("TTY output must not fall back to the plain non-TTY line format")
	}
	env := cmdtest.DecodeEnvelope(t, stdout)
	var got api.Cluster
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if got.APIKey == nil || got.APIKey.Value != "k" {
		t.Fatalf("api_key.value = %v, want k", got.APIKey)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateWaitNonTTYByteCleanWithLiveTicker(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	f.PollInterval = 260 * time.Millisecond // > create.go's defaultAnimationInterval (120ms)
	f.IOStreams.SetStderrTTY(false)
	stderr, ok := f.IOStreams.Err.(*bytes.Buffer)
	if !ok {
		t.Fatalf("f.IOStreams.Err is %T, want *bytes.Buffer", f.IOStreams.Err)
	}

	apiMock.EXPECT().
		CreateCluster(mock.Anything, mock.Anything, mock.Anything).
		Return(&api.Cluster{ID: "cid-ticker"}, nil)
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-ticker").
		Return(api.StatusCreating, nil).Once()
	apiMock.EXPECT().
		GetClusterStatus(mock.Anything, "cid-ticker").
		Return(api.StatusReady, nil).Once()
	apiMock.EXPECT().
		GetCluster(mock.Anything, "cid-ticker").
		Return(&api.Cluster{ID: "cid-ticker", Status: api.StatusReady}, nil).Times(1)

	if err := cmdtest.Run(t, f, "cluster", "create", "--wait"); err != nil {
		t.Fatalf("execute: %v", err)
	}

	out := stderr.String()
	if bytes.ContainsAny(stderr.Bytes(), "\x1b\r") {
		t.Fatalf("non-TTY stderr contains control/escape bytes with the ticker live: %q", out)
	}
	want := "status: CREATING\nstatus: READY\n"
	if out != want {
		t.Fatalf("stderr = %q, want %q", out, want)
	}
}
