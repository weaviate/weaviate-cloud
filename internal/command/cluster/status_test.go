package cluster_test

import (
	"errors"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/cmdtest"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

//nolint:paralleltest // t.Setenv via cmdtest.NewFactory; incompatible with t.Parallel
func TestClusterStatusNotAuthenticated(t *testing.T) {
	f, _, _ := cmdtest.NewFactory(t, false)

	err := cmdtest.Run(t, f, "cluster", "status", "cid-1")
	var e *errcode.Error
	if !errors.As(err, &e) || e.Code != errcode.CodeAuthRequired {
		t.Fatalf("err = %v, want CodeAuthRequired", err)
	}
	if exit := errcode.ExitCodeFor(err); exit != errcode.AuthRequired {
		t.Fatalf("exit code = %d, want %d (AuthRequired)", exit, errcode.AuthRequired)
	}
}
