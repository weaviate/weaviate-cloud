// Package profile owns the "wcloud profile" command group, an internal
// surface (Hidden: true) used by Weaviate engineering to point the CLI at
// non-production environments without baking environment names into the
// public binary.
package profile

import (
	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
)

func NewProfileCmd(f *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "profile",
		Short:  "Manage CLI profiles (internal)",
		Hidden: true,
		Args:   errcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(NewListCmd(f))
	cmd.AddCommand(NewCreateCmd(f))
	cmd.AddCommand(NewUseCmd(f))
	cmd.AddCommand(NewDeleteCmd(f))
	cmd.AddCommand(NewShowCmd(f))
	return cmd
}
