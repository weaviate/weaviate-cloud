package cluster

import (
	"context"
	"io"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/api"
)

// PollClient is the test-visible alias for the internal pollClient interface.
type PollClient = pollClient

// PollConfig is the test-visible counterpart of the internal pollConfig.
type PollConfig struct {
	Client            PollClient
	ClusterID         string
	Deadline          time.Time
	NowFn             func() time.Time
	TickInterval      time.Duration
	AnimationInterval time.Duration
	Progress          io.Writer
	StderrTTY         bool
}

// PollUntilReady is the test-visible entry point for pollUntilReady.
func PollUntilReady(ctx context.Context, cfg PollConfig) (*api.Cluster, error) {
	return pollUntilReady(ctx, pollConfig{
		client:            cfg.Client,
		clusterID:         cfg.ClusterID,
		deadline:          cfg.Deadline,
		nowFn:             cfg.NowFn,
		tickInterval:      cfg.TickInterval,
		animationInterval: cfg.AnimationInterval,
		progress:          cfg.Progress,
		stderrTTY:         cfg.StderrTTY,
	})
}
