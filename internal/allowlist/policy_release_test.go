//go:build !wcloud_dev

package allowlist_test

import (
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/allowlist"
)

func TestProductionCheck_LoopbackRejectedWithoutDevTag(t *testing.T) {
	t.Parallel()
	for _, u := range []string{"http://127.0.0.1:53682/", "http://[::1]:53682/"} {
		if err := allowlist.Production().Check(u); err == nil {
			t.Errorf("Check(%q) = nil, want rejection (this is a release build)", u)
		}
	}
}
