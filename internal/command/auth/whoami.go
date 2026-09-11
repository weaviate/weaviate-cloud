package auth

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

func NewWhoamiCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the identity behind the active session",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			token, err := f.Auth.RequireToken(ctx)
			if err != nil {
				return err
			}

			info, err := f.NewAPIClient(token).Whoami(ctx)
			if err != nil {
				return fmt.Errorf("whoami: %w", err)
			}

			return output.Write(f.IOStreams, f.OutputFormat, info, f.NewRequestID(), renderWhoAmI)
		},
	}
}
