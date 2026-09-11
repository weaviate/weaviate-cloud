package cluster

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/command/skill"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

const defaultWaitTimeout = 15 * time.Minute

func NewCreateCmd(f *factory.Factory) *cobra.Command {
	var req api.CreateClusterRequest
	var doWait bool
	var timeoutFlag time.Duration

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a cluster",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			token, err := f.Auth.RequireToken(ctx)
			if err != nil {
				return err
			}
			provisionedBy := "cli"
			if skill.IsAgentDriven() {
				provisionedBy = "agent"
			}
			opts := api.CreateClusterOptions{
				IdempotencyKey: f.NewRequestID(),
				ProvisionedBy:  provisionedBy,
			}
			cluster, err := f.NewAPIClient(token).CreateCluster(ctx, req, opts)
			if err != nil {
				return createFailure(opts.IdempotencyKey, err)
			}

			if !doWait {
				return output.Write(f.IOStreams, f.OutputFormat, cluster, f.NewRequestID(), renderCluster)
			}

			timeout := timeoutFlag
			if timeout == 0 {
				timeout = defaultWaitTimeout
			}
			ready, err := pollUntilReady(ctx, pollConfig{
				client:            f.NewAPIClient(token),
				clusterID:         cluster.ID,
				deadline:          f.Now().Add(timeout),
				nowFn:             f.Now,
				tickInterval:      f.PollInterval,
				animationInterval: defaultAnimationInterval,
				progress:          f.IOStreams.Err,
				stderrTTY:         f.IOStreams.IsStderrTTY() && output.HumanReaderLikely(),
			})
			if err != nil {
				return err
			}
			return output.Write(f.IOStreams, f.OutputFormat, ready, f.NewRequestID(), renderCluster)
		},
	}

	cmd.Flags().StringVar(&req.Name, "name", "", "Cluster name (default: automatically generated)")
	cmd.Flags().StringVar(&req.Region, "region", "",
		"Region ID (optional; if omitted, your account's default region is used)")
	cmd.Flags().StringVar(&req.Tier, "tier", "", "Cluster tier (default: free)")
	cmd.Flags().BoolVarP(&doWait, "wait", "w", false, "Block until the cluster is READY")
	cmd.Flags().DurationVar(&timeoutFlag, "timeout", 0,
		"Maximum time to wait for READY (default: 15m; requires --wait)")
	return cmd
}

// createFailure reports an unconfirmed create as recoverable: a request the server
// never answered may still have been accepted, and the idempotency key is the
// caller's only handle on a retry that deduplicates. A coded error is an answer
// from the server, so it passes through untouched.
func createFailure(idempotencyKey string, cause error) error {
	if _, coded := errcode.CodeFor(cause); coded {
		return fmt.Errorf("create cluster: %w", cause)
	}
	return detailedFailure(
		map[string]any{"idempotency_key": idempotencyKey, "cluster_may_exist": true},
		"could not confirm whether the cluster was created; run 'wcloud cluster list' before retrying, "+
			"and resend the idempotency_key from error.details so a retry is likely to be deduplicated",
		fmt.Errorf("create cluster: %w", cause),
	)
}
