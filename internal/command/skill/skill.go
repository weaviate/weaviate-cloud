package skill

import (
	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
)

func NewSkillCmd(f *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install the wcloud agent skill into coding harnesses",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(NewInstallCmd(f))
	return cmd
}
