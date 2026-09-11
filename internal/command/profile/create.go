package profile

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

type profileCreateData struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

func NewCreateCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new profile via interactive wizard",
		Args:  errcode.Args(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			io := f.IOStreams
			if !io.IsStdinTTY() {
				return errcode.New(errcode.CodeValidationFailed,
					"profile create requires an interactive terminal")
			}

			wiz := newWizardIO(io.In, io.Err)

			name, err := resolveProfileName(wiz, args)
			if err != nil {
				return err
			}

			profiles, err := config.LoadProfiles()
			if err != nil {
				return fmt.Errorf("load profiles: %w", err)
			}
			if _, exists := profiles.Profiles[name]; exists {
				return errcode.New(errcode.CodeValidationFailed,
					fmt.Sprintf("profile %q already exists", name))
			}

			profile, err := runProfileWizard(wiz)
			if err != nil {
				return err
			}
			if validateErr := config.ValidateProfile(profile); validateErr != nil {
				return errcode.New(errcode.CodeValidationFailed, validateErr.Error())
			}

			setActive, err := wiz.confirm("Set as active profile now?", true)
			if err != nil {
				return err
			}

			if profiles.Profiles == nil {
				profiles.Profiles = map[string]config.Profile{}
			}
			profiles.Profiles[name] = profile
			if setActive {
				profiles.Active = name
			}
			if saveErr := config.SaveProfiles(profiles); saveErr != nil {
				return fmt.Errorf("save profiles: %w", saveErr)
			}

			env := output.Envelope[profileCreateData]{
				Data:     profileCreateData{Name: name, Active: setActive},
				Metadata: makeMetadata(f),
			}
			return output.WriteEnv(io, f.OutputFormat, env, renderProfileCreate)
		},
	}
}

func resolveProfileName(wiz *wizardIO, args []string) (string, error) {
	if len(args) == 1 {
		if err := config.ValidateProfileName(args[0]); err != nil {
			return "", errcode.New(errcode.CodeValidationFailed, err.Error())
		}
		return args[0], nil
	}
	for {
		name, err := wiz.prompt("Profile name", "")
		if err != nil {
			return "", err
		}
		if validateErr := config.ValidateProfileName(name); validateErr != nil {
			fmt.Fprintf(wiz.errOut, "%v\n", validateErr)
			continue
		}
		return name, nil
	}
}

func runProfileWizard(wiz *wizardIO) (config.Profile, error) {
	var p config.Profile

	for {
		endpoint, err := wiz.prompt("API endpoint", "leave blank for default")
		if err != nil {
			return p, err
		}
		if endpoint == "" {
			break
		}
		p.Endpoint = endpoint
		if validateErr := config.ValidateProfile(p); validateErr != nil {
			fmt.Fprintf(wiz.errOut, "%v\n", validateErr)
			p.Endpoint = ""
			continue
		}
		break
	}

	customAuth, err := wiz.confirm("Use a custom auth provider?", false)
	if err != nil {
		return p, err
	}
	if !customAuth {
		return p, nil
	}

	auth, err := promptAuth(wiz)
	if err != nil {
		return p, err
	}
	p.Auth = &auth
	return p, nil
}

func promptAuth(wiz *wizardIO) (config.AuthConfig, error) {
	var auth config.AuthConfig
	for {
		baseURL, err := wiz.prompt("Auth base URL", "https://...")
		if err != nil {
			return auth, err
		}
		auth.BaseURL = baseURL
		clientID, err := wiz.prompt("Auth client ID", "")
		if err != nil {
			return auth, err
		}
		auth.ClientID = clientID

		probe := config.Profile{Auth: &auth}
		if validateErr := config.ValidateProfile(probe); validateErr != nil {
			fmt.Fprintf(wiz.errOut, "%v\n", validateErr)
			auth = config.AuthConfig{}
			continue
		}
		return auth, nil
	}
}
