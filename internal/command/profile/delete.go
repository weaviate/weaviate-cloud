package profile

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

type profileDeleteData struct {
	Deleted string `json:"deleted"`
	Active  string `json:"active"`
}

func NewDeleteCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a profile (interactive confirm)",
		Args:  errcode.Args(cobra.ExactArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			io := f.IOStreams

			if name == config.DefaultProfileName {
				return errcode.New(errcode.CodeValidationFailed,
					"the 'default' profile is built-in and cannot be deleted")
			}

			profiles, err := config.LoadProfiles()
			if err != nil {
				return fmt.Errorf("load profiles: %w", err)
			}
			if _, ok := profiles.Profiles[name]; !ok {
				return errcode.New(errcode.CodeValidationFailed,
					fmt.Sprintf("profile %q does not exist", name))
			}

			if !io.IsStdinTTY() {
				return errcode.New(errcode.CodeValidationFailed,
					"profile delete requires an interactive terminal for confirmation")
			}

			fmt.Fprintf(io.Err, "Delete profile %q? [y/N]: ", name)
			reader := bufio.NewReader(io.In)
			answer, readErr := reader.ReadString('\n')
			if readErr != nil {
				return fmt.Errorf("read confirmation: %w", readErr)
			}
			if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
				return errcode.New(errcode.CodeValidationFailed, "deletion cancelled")
			}

			delete(profiles.Profiles, name)
			if profiles.Active == name {
				profiles.Active = config.DefaultProfileName
			}
			if saveErr := config.SaveProfiles(profiles); saveErr != nil {
				return fmt.Errorf("save profiles: %w", saveErr)
			}
			if rmErr := config.DeleteCredentials(name); rmErr != nil {
				fmt.Fprintf(io.Err, "warning: failed to remove credentials for %q: %v\n", name, rmErr)
			}

			env := output.Envelope[profileDeleteData]{
				Data:     profileDeleteData{Deleted: name, Active: profiles.Active},
				Metadata: makeMetadata(f),
			}
			return output.WriteEnv(io, f.OutputFormat, env, renderProfileDelete)
		},
	}
}
