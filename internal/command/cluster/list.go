package cluster

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

func NewListCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List clusters in your organization",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			token, err := f.Auth.RequireToken(ctx)
			if err != nil {
				return err
			}
			clusters, err := f.NewAPIClient(token).ListClusters(ctx)
			if err != nil {
				return fmt.Errorf("list clusters: %w", err)
			}
			return output.Write(f.IOStreams, f.OutputFormat, clusters, f.NewRequestID(), renderClusterList)
		},
	}
}
