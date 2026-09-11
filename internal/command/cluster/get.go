package cluster

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

func NewGetCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <cluster-id>",
		Short: "Get a cluster by ID",
		Args:  errcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			token, err := f.Auth.RequireToken(ctx)
			if err != nil {
				return err
			}
			cluster, err := f.NewAPIClient(token).GetCluster(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get cluster: %w", err)
			}
			return output.Write(f.IOStreams, f.OutputFormat, cluster, f.NewRequestID(), renderCluster)
		},
	}
}
