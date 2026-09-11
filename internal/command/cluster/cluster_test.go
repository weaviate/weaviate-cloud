package cluster_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/cmdtest"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterList(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().ListClusters(mock.Anything).Return([]api.Cluster{{}, {}}, nil)

	if err := cmdtest.Run(t, f, "cluster", "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	env := cmdtest.DecodeEnvelope(t, stdout)
	var got []api.Cluster
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("clusters = %d, want 2", len(got))
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreate(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	matchReq := mock.MatchedBy(func(req api.CreateClusterRequest) bool {
		return req.Name == "c1" && req.Region == "us-east1" && req.Tier == "free"
	})
	withKey := mock.MatchedBy(func(opts api.CreateClusterOptions) bool {
		return opts.IdempotencyKey != ""
	})
	apiMock.EXPECT().
		CreateCluster(mock.Anything, matchReq, withKey).
		Return(&api.Cluster{ID: "cid-1"}, nil)

	if err := cmdtest.Run(t, f, "cluster", "create",
		"--name", "c1", "--region", "us-east1", "--tier", "free"); err != nil {
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
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateNoFlags(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	emptyReq := mock.MatchedBy(func(req api.CreateClusterRequest) bool {
		return req == api.CreateClusterRequest{}
	})
	apiMock.EXPECT().
		CreateCluster(mock.Anything, emptyReq, mock.Anything).
		Return(&api.Cluster{ID: "cid-2"}, nil)

	if err := cmdtest.Run(t, f, "cluster", "create"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if cmdtest.DecodeEnvelope(t, stdout).Data == nil {
		t.Fatal("data should be a cluster object")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateNameOnly(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	nameOnly := mock.MatchedBy(func(req api.CreateClusterRequest) bool {
		return req.Name == "c1" && req.Region == "" && req.Tier == ""
	})
	apiMock.EXPECT().
		CreateCluster(mock.Anything, nameOnly, mock.Anything).
		Return(&api.Cluster{ID: "cid-3"}, nil)

	if err := cmdtest.Run(t, f, "cluster", "create", "--name", "c1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if cmdtest.DecodeEnvelope(t, stdout).Data == nil {
		t.Fatal("data should be a cluster object")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterGet(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-1").Return(&api.Cluster{}, nil)

	if err := cmdtest.Run(t, f, "cluster", "get", "cid-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	env := cmdtest.DecodeEnvelope(t, stdout)
	if string(env.Data) == "null" {
		t.Fatal("data should be a cluster object, got null")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterStatus(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-1").Return(api.ClusterStatus("RUNNING"), nil)

	if err := cmdtest.Run(t, f, "cluster", "status", "cid-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	env := cmdtest.DecodeEnvelope(t, stdout)
	var got string
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if got != "RUNNING" {
		t.Fatalf("status = %q, want RUNNING", got)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterListNotAuthenticated(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, false)
	err := cmdtest.Run(t, f, "cluster", "list")
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want CodeAuthRequired", err)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterGetAPIError(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-1").Return(nil, errors.New("boom"))

	if err := cmdtest.Run(t, f, "cluster", "get", "cid-1"); err == nil {
		t.Fatal("expected error from API failure")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterGetRequiresID(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, true)
	if err := cmdtest.Run(t, f, "cluster", "get"); err == nil {
		t.Fatal("expected error: get requires a cluster id")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterStatusNoArgs_UsageError(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, true)
	err := cmdtest.Run(t, f, "cluster", "status") // no cluster ID
	if err == nil {
		t.Fatal("expected an error for missing cluster ID")
	}
	code, ok := errcode.CodeFor(err)
	if !ok || code != errcode.CodeValidationFailed {
		t.Fatalf("code = %q (ok=%v), want %q", code, ok, errcode.CodeValidationFailed)
	}
	if exit := errcode.ExitCodeFor(err); exit != errcode.UsageError {
		t.Fatalf("exit code = %d, want %d (UsageError)", exit, errcode.UsageError)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterGetBadID_StillClusterNotFound(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().GetCluster(mock.Anything, "nonexistent-cluster-id").
		Return(nil, &api.Error{Code: errcode.CodeClusterNotFound, Message: "no such cluster"})

	err := cmdtest.Run(t, f, "cluster", "get", "nonexistent-cluster-id")
	if err == nil {
		t.Fatal("expected an error for a nonexistent cluster")
	}
	code, ok := errcode.CodeFor(err)
	if !ok || code != errcode.CodeClusterNotFound {
		t.Fatalf("code = %q (ok=%v), want %q; the Args wrap must not touch RunE errors",
			code, ok, errcode.CodeClusterNotFound)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterGetNoArgs_UsageError(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, true)
	err := cmdtest.Run(t, f, "cluster", "get") // no cluster ID
	if err == nil {
		t.Fatal("expected an error for missing cluster ID")
	}
	code, ok := errcode.CodeFor(err)
	if !ok || code != errcode.CodeValidationFailed {
		t.Fatalf("code = %q (ok=%v), want %q", code, ok, errcode.CodeValidationFailed)
	}
	if exit := errcode.ExitCodeFor(err); exit != errcode.UsageError {
		t.Fatalf("exit code = %d, want %d (UsageError)", exit, errcode.UsageError)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterListTextOutput(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().ListClusters(mock.Anything).Return([]api.Cluster{
		{ID: "cid-1", Name: "my-cluster", Status: api.StatusReady, Tier: "free", Region: "us-east1"},
	}, nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "cluster", "list"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "ID") || !strings.Contains(got, "NAME") || !strings.Contains(got, "STATUS") {
		t.Fatalf("expected table headers, got %q", got)
	}
	if !strings.Contains(got, "cid-1") || !strings.Contains(got, "my-cluster") {
		t.Fatalf("expected cluster data, got %q", got)
	}
	if strings.Contains(got, "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterGetTextOutput(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-1").Return(&api.Cluster{
		ID: "cid-1", Name: "my-cluster", Status: api.StatusReady,
	}, nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "cluster", "get", "cid-1"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "id:") || !strings.Contains(got, "cid-1") {
		t.Fatalf("expected id key-value, got %q", got)
	}
	if strings.Contains(got, "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterGetTextOutput_ShowsStatusReason(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-1").Return(&api.Cluster{
		ID: "cid-1", Status: api.StatusFailed,
		StatusReason: "environment provisioning failed",
	}, nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "cluster", "get", "cid-1"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "status_reason:") || !strings.Contains(got, "environment provisioning failed") {
		t.Fatalf("expected status_reason key-value, got %q", got)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterGetTextOutput_OmitsEmptyAPIKey(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-1").Return(&api.Cluster{
		ID: "cid-1", Status: api.StatusReady,
		APIKey: &api.KeyInfo{Value: "", Warning: "retrieve it from your local cache"},
	}, nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "cluster", "get", "cid-1"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if strings.Contains(got, "api_key:") {
		t.Fatalf("expected api_key line to be omitted when value is empty, got %q", got)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterGetTextOutput_ShowsPopulatedAPIKey(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-1").Return(&api.Cluster{
		ID: "cid-1", Status: api.StatusReady,
		APIKey: &api.KeyInfo{Value: "abc123"},
	}, nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "cluster", "get", "cid-1"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "api_key:") || !strings.Contains(got, "abc123") {
		t.Fatalf("expected api_key key-value with populated value, got %q", got)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterCreateRegionFlagHelpHasNoFictitiousDefault(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, true)
	root := cli.NewRootCmd(f)
	cmd, _, err := root.Find([]string{"cluster", "create"})
	if err != nil {
		t.Fatalf("find cluster create: %v", err)
	}
	usage := cmd.Flags().Lookup("region").Usage
	if strings.Contains(usage, "europe-west3") {
		t.Fatalf("region flag help still advertises non-existent region: %q", usage)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterStatusTextOutputSanitizesEscapeSequences(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	malicious := api.ClusterStatus("CREATING\x1b[2K\r\x1b[32m✔ cluster READY — SPOOFED STATUS LINE\x1b[0m")
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-1").Return(malicious, nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "cluster", "status", "cid-1"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if strings.ContainsAny(got, "\x1b\r") {
		t.Fatalf("status text output still contains raw ESC/CR bytes: %q", got)
	}
	if !strings.Contains(got, "CREATING✔ cluster READY — SPOOFED STATUS LINE") {
		t.Fatalf("expected sanitized status line, got %q", got)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterGetTextOutputSanitizesEscapeSequences(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	malicious := "legit-cluster\x1b[2K\r\x1b[32m✔ cluster READY — SPOOFED STATUS LINE\x1b[0m"
	apiMock.EXPECT().GetCluster(mock.Anything, "cid-1").Return(&api.Cluster{
		ID: "cid-1", Name: malicious, Status: api.StatusReady,
	}, nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "cluster", "get", "cid-1"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if strings.ContainsAny(got, "\x1b\r") {
		t.Fatalf("get text output still contains raw ESC/CR bytes: %q", got)
	}
	if !strings.Contains(got, "legit-cluster✔ cluster READY — SPOOFED STATUS LINE") {
		t.Fatalf("expected sanitized cluster name, got %q", got)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterStatusTextOutput(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().GetClusterStatus(mock.Anything, "cid-1").Return(api.ClusterStatus("RUNNING"), nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "cluster", "status", "cid-1"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "RUNNING") {
		t.Fatalf("expected RUNNING in output, got %q", got)
	}
	if strings.Contains(got, "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
}
