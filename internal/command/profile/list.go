package profile

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

type profileListItem struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type profileListData struct {
	Active   string            `json:"active"`
	Profiles []profileListItem `json:"profiles"`
}

func NewListCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available profiles",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(_ *cobra.Command, _ []string) error {
			profiles, err := config.LoadProfiles()
			if err != nil {
				return fmt.Errorf("load profiles: %w", err)
			}

			active := config.ActiveProfile(profiles)

			names := []string{config.DefaultProfileName}
			for name := range profiles.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)

			items := make([]profileListItem, 0, len(names))
			for _, name := range names {
				items = append(items, profileListItem{Name: name, Active: name == active})
			}

			env := output.Envelope[profileListData]{
				Data:     profileListData{Active: active, Profiles: items},
				Metadata: makeMetadata(f),
			}
			return output.WriteEnv(f.IOStreams, f.OutputFormat, env, renderProfileList)
		},
	}
}
