package profile

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

type profileUseData struct {
	Active string `json:"active"`
}

func NewUseCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Switch active profile",
		Args:  errcode.Args(cobra.ExactArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			if name != config.DefaultProfileName {
				if err := config.ValidateProfileName(name); err != nil {
					return errcode.New(errcode.CodeValidationFailed, err.Error())
				}
			}

			profiles, err := config.LoadProfiles()
			if err != nil {
				return fmt.Errorf("load profiles: %w", err)
			}

			if name != config.DefaultProfileName {
				if _, ok := profiles.Profiles[name]; !ok {
					return errcode.New(errcode.CodeValidationFailed,
						fmt.Sprintf("profile %q does not exist", name))
				}
			}

			profiles.Active = name
			if saveErr := config.SaveProfiles(profiles); saveErr != nil {
				return fmt.Errorf("save profiles: %w", saveErr)
			}

			env := output.Envelope[profileUseData]{
				Data:     profileUseData{Active: name},
				Metadata: makeMetadata(f),
			}
			return output.WriteEnv(f.IOStreams, f.OutputFormat, env, renderProfileUse)
		},
	}
}
