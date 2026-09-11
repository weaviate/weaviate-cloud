package region

import (
	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
)

func NewRegionCmd(f *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "region",
		Short: "Inspect Weaviate Cloud regions",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(NewListCmd(f))
	return cmd
}
