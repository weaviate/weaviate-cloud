package auth

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	authpkg "github.com/weaviate/weaviate-cloud/internal/auth"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

// loginResult intentionally omits the token, refresh token, and any JWT
// claim that contains an identifier (project ID, client ID, org ID). Email
// is the one user-supplied identity safe to echo back.
type loginResult struct {
	SignedIn bool   `json:"signed_in"`
	Email    string `json:"email,omitempty"`
}

func NewLoginCmd(f *factory.Factory) *cobra.Command {
	var noLaunchBrowser bool
	var timeoutFlag time.Duration

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate via the browser and store credentials locally",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			tokens, err := f.Auth.Login(ctx, authpkg.LoginOptions{
				Timeout:         timeoutFlag,
				NoLaunchBrowser: noLaunchBrowser,
				Now:             f.Now,
			})
			if err != nil {
				return fmt.Errorf("login: %w", err)
			}
			if _, saveErr := f.Auth.Save(tokens); saveErr != nil {
				return fmt.Errorf("persist credentials: %w", saveErr)
			}

			fmt.Fprintln(f.IOStreams.Err, "Signed in.")

			creds, _ := f.Auth.Load()
			result := loginResult{SignedIn: true}
			if creds != nil {
				result.Email = creds.Email()
			}

			return output.Write(f.IOStreams, f.OutputFormat, result, f.NewRequestID(), renderLogin)
		},
	}

	cmd.Flags().BoolVar(&noLaunchBrowser, "no-launch-browser", false,
		"Do not attempt to launch a browser; print the sign-in URL instead")
	cmd.Flags().DurationVar(&timeoutFlag, "timeout", 0,
		"Maximum time to wait for the browser sign-in to complete (default: 5m)")

	return cmd
}
