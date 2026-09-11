package auth

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

type logoutResult struct {
	SignedOut bool `json:"signed_out"`
}

func NewLogoutCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Forget the local credentials for the active profile",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := f.Auth.Logout(cmd.Context()); err != nil {
				return fmt.Errorf("logout: %w", err)
			}
			fmt.Fprintln(f.IOStreams.Err, "Signed out.")

			result := logoutResult{SignedOut: true}
			return output.Write(f.IOStreams, f.OutputFormat, result, f.NewRequestID(), renderLogout)
		},
	}
}
