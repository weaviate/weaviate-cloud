package profile

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

// profileShowData deliberately surfaces presence booleans only — endpoint
// URLs, auth base URL, and client ID are never echoed back to stdout.
type profileShowData struct {
	Name               string `json:"name"`
	Active             bool   `json:"active"`
	EndpointConfigured bool   `json:"endpoint_configured"`
	AuthConfigured     bool   `json:"auth_configured"`
}

func NewShowCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show profile configuration (presence only — no values)",
		Args:  errcode.Args(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			profiles, err := config.LoadProfiles()
			if err != nil {
				return fmt.Errorf("load profiles: %w", err)
			}

			active := config.ActiveProfile(profiles)
			name := active
			if len(args) == 1 {
				name = args[0]
			}

			data := profileShowData{Name: name, Active: name == active}

			if name == config.DefaultProfileName {
				data.EndpointConfigured = true
				data.AuthConfigured = true
			} else {
				prof, ok := profiles.Profiles[name]
				if !ok {
					return errcode.New(errcode.CodeValidationFailed,
						fmt.Sprintf("profile %q does not exist", name))
				}
				data.EndpointConfigured = prof.Endpoint != ""
				data.AuthConfigured = prof.Auth != nil
			}

			env := output.Envelope[profileShowData]{
				Data:     data,
				Metadata: makeMetadata(f),
			}
			return output.WriteEnv(f.IOStreams, f.OutputFormat, env, renderProfileShow)
		},
	}
}
