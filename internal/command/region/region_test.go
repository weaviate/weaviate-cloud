package region_test

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
func TestRegionList(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().ListRegions(mock.Anything).Return([]api.Region{{}, {}, {}}, nil)

	if err := cmdtest.Run(t, f, "region", "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	env := cmdtest.DecodeEnvelope(t, stdout)
	var got []api.Region
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("regions = %d, want 3", len(got))
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestRegionListNotAuthenticated(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, false)
	err := cmdtest.Run(t, f, "region", "list")
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want CodeAuthRequired", err)
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestRegionListAPIError(t *testing.T) {
	f, apiMock, _ := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().ListRegions(mock.Anything).Return(nil, errors.New("boom"))

	if err := cmdtest.Run(t, f, "region", "list"); err == nil {
		t.Fatal("expected error from API failure")
	}
}

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestRegionListTextOutput(t *testing.T) {
	f, apiMock, stdout := cmdtest.NewFactory(t, true)
	apiMock.EXPECT().ListRegions(mock.Anything).Return([]api.Region{
		{ID: "us-east1", Name: "US East", CloudProvider: "gcp", Status: "AVAILABLE", IsDefault: true},
	}, nil)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"--output", "text", "region", "list"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "ID") || !strings.Contains(got, "NAME") || !strings.Contains(got, "CLOUD_PROVIDER") {
		t.Fatalf("expected table headers, got %q", got)
	}
	if !strings.Contains(got, "us-east1") {
		t.Fatalf("expected region data, got %q", got)
	}
	if strings.Contains(got, "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
}
