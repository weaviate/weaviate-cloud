package cluster

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

func NewStatusCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "status <cluster-id>",
		Short: "Get the lifecycle status of a cluster",
		Args:  errcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			token, err := f.Auth.RequireToken(ctx)
			if err != nil {
				return err
			}
			status, err := f.NewAPIClient(token).GetClusterStatus(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get cluster status: %w", err)
			}
			return output.Write(f.IOStreams, f.OutputFormat, status, f.NewRequestID(), renderClusterStatus)
		},
	}
}
