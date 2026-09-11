package cluster

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

type pollClient interface {
	GetClusterStatus(ctx context.Context, id string) (api.ClusterStatus, error)
	GetCluster(ctx context.Context, id string) (*api.Cluster, error)
}

type pollConfig struct {
	client            pollClient
	clusterID         string
	deadline          time.Time
	nowFn             func() time.Time
	tickInterval      time.Duration
	animationInterval time.Duration
	progress          io.Writer
	stderrTTY         bool
}

type statusClass int

const (
	classPolling statusClass = iota
	classReady
	classTerminal
	classUnrecognized
)

func classifyStatus(s api.ClusterStatus) statusClass {
	switch s {
	case api.StatusReady:
		return classReady
	case api.StatusPending, api.StatusCreating, api.StatusUpdating, api.StatusWaiting, api.StatusDeleting:
		return classPolling
	case api.StatusFailed, api.StatusDeleted, api.StatusExpired, api.StatusSuspended:
		return classTerminal
	case api.StatusUnknown:
		return classUnrecognized
	}
	return classUnrecognized
}

// detailedFailure builds the machine-readable error envelope and keeps the cause in
// the chain, so its error code, retry-after and raw excerpt survive the wrap.
func detailedFailure(details map[string]any, message string, cause error) error {
	code := errcode.CodeInternalError
	if c, ok := errcode.CodeFor(cause); ok {
		code = c
	}
	failure := &errcode.Error{Code: code, Message: message, Details: details}
	if cause == nil {
		return failure
	}
	return fmt.Errorf("%w: %w", failure, cause)
}

// waitFailure carries the cluster ID in error.details on every --wait failure:
// the ID is the caller's only handle on a cluster that was created but never
// observed READY, and error.message is not a machine-readable field.
func waitFailure(clusterID string, details map[string]any, message string, cause error) error {
	details["cluster_id"] = clusterID
	return detailedFailure(details, message, cause)
}

func provisioningDetails(lastStatus api.ClusterStatus) map[string]any {
	details := map[string]any{"still_provisioning": true}
	if lastStatus != "" {
		details["last_status"] = string(lastStatus)
	}
	return details
}

// waitForNextPoll blocks until tickInterval elapses or ctx is cancelled,
// whichever comes first, calling onTick every animationInterval in between.
func waitForNextPoll(ctx context.Context, tickInterval, animationInterval time.Duration, onTick func()) error {
	if tickInterval <= 0 {
		return nil
	}

	pollTimer := time.NewTimer(tickInterval)
	defer pollTimer.Stop()

	if animationInterval <= 0 || animationInterval >= tickInterval {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTimer.C:
			return nil
		}
	}

	animTicker := time.NewTicker(animationInterval)
	defer animTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTimer.C:
			return nil
		case <-animTicker.C:
			onTick()
		}
	}
}

func pollUntilReady(ctx context.Context, cfg pollConfig) (*api.Cluster, error) {
	var lastStatus api.ClusterStatus
	renderer := newProgressRenderer(cfg.progress, cfg.nowFn, cfg.stderrTTY)

	for {
		if cfg.nowFn().After(cfg.deadline) {
			renderer.onDone(lastStatus)
			return nil, timeoutFailure(cfg.clusterID, lastStatus)
		}

		select {
		case <-ctx.Done():
			renderer.onDone(lastStatus)
			return nil, cancelledWaitFailure(cfg.clusterID, lastStatus, ctx.Err())
		default:
		}

		status, err := cfg.client.GetClusterStatus(ctx, cfg.clusterID)
		if err != nil {
			renderer.onDone(lastStatus)
			return nil, waitFailure(cfg.clusterID, provisioningDetails(lastStatus),
				fmt.Sprintf("cluster %s was created but polling its status failed", cfg.clusterID),
				fmt.Errorf("get cluster status: %w", err))
		}
		lastStatus = status
		renderer.onPoll(status)

		switch classifyStatus(status) {
		case classReady:
			ready, getErr := cfg.client.GetCluster(ctx, cfg.clusterID)
			if getErr != nil {
				renderer.onDone(status)
				return nil, waitFailure(cfg.clusterID, map[string]any{"last_status": string(status)},
					fmt.Sprintf("cluster %s became READY but fetching it failed", cfg.clusterID),
					fmt.Errorf("get cluster after ready: %w", getErr))
			}
			renderer.onDone(status)
			return ready, nil
		case classTerminal:
			renderer.onDone(status)
			return nil, waitFailure(cfg.clusterID, map[string]any{"terminal_status": string(status)},
				fmt.Sprintf("cluster %s reached terminal status %s", cfg.clusterID, status), nil)
		case classPolling, classUnrecognized:
		}

		if waitErr := waitForNextPoll(ctx, cfg.tickInterval, cfg.animationInterval, renderer.onTick); waitErr != nil {
			renderer.onDone(lastStatus)
			return nil, cancelledWaitFailure(cfg.clusterID, lastStatus, waitErr)
		}
	}
}

// timeoutFailure names an unrecognized last status instead of reporting the
// bare timeout: waiting out the clock on a status this build has never heard of
// is not the same failure as a cluster that stalled in CREATING.
func timeoutFailure(clusterID string, lastStatus api.ClusterStatus) error {
	details := provisioningDetails(lastStatus)
	message := fmt.Sprintf("timed out waiting for cluster %s to become READY", clusterID)
	if lastStatus != "" && classifyStatus(lastStatus) == classUnrecognized {
		details["unrecognized_status"] = string(lastStatus)
		message = fmt.Sprintf(
			"timed out waiting for cluster %s to become READY; its last status %s is not recognized by this wcloud version",
			clusterID,
			lastStatus,
		)
	}
	return waitFailure(clusterID, details, message, nil)
}

func cancelledWaitFailure(clusterID string, lastStatus api.ClusterStatus, cause error) error {
	return waitFailure(clusterID, provisioningDetails(lastStatus),
		fmt.Sprintf("stopped waiting for cluster %s before it became READY; it is still provisioning", clusterID),
		cause)
}
