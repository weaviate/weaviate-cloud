package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/guide"
)

func NewGuideCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "guide",
		Short: "Print the wcloud agent guide (markdown) for LLM coding agents",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(f.IOStreams.Out, guide.Markdown())
			return err
		},
	}
}
