package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/command/auth"
	"github.com/weaviate/weaviate-cloud/internal/command/cluster"
	"github.com/weaviate/weaviate-cloud/internal/command/profile"
	"github.com/weaviate/weaviate-cloud/internal/command/region"
	"github.com/weaviate/weaviate-cloud/internal/command/skill"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

type GlobalFlags struct {
	Output  string
	Verbose bool
}

func NewRootCmd(f *factory.Factory) *cobra.Command {
	flags := &GlobalFlags{}

	cmd := &cobra.Command{
		Use:   "wcloud",
		Short: "Weaviate Cloud CLI",
		Long: "wcloud is the Weaviate Cloud CLI — agent-first provisioning over the Weaviate Cloud API.\n\n" +
			"New here? Run `wcloud guide` for the full agent walkthrough: install, auth, provision, and consume.",
		Version:       resolvedVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		// WHY: a group must be runnable or cobra returns help and exit 0 before Args runs, letting an unknown verb read as success.
		Args: errcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			format, err := output.ResolveFormat(flags.Output, f.IOStreams.IsStdoutTTY())
			if err != nil {
				return errcode.New(errcode.CodeValidationFailed, err.Error())
			}
			f.OutputFormat = format
			f.Verbose = flags.Verbose
			return nil
		},
	}

	// WHY: cobra's default help writer is stdout, which corrupts the JSON channel
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if !c.Flags().Changed("help") && f.OutputFormat == output.FormatJSON {
			c.SetOut(f.IOStreams.Err)
			defer c.SetOut(f.IOStreams.Out)
		}
		defaultHelp(c, args)
	})

	cmd.PersistentFlags().StringVarP(&flags.Output, "output", "o", "auto",
		"output format: auto, json, text (auto = text for a human at a terminal, json when piped or under CI)")

	cmd.PersistentFlags().BoolVarP(&flags.Verbose, "verbose", "v", false, "enable verbose logging to stderr")

	cmd.SetHelpCommand(&cobra.Command{
		Use:    "help [command]",
		Short:  "Help about any command",
		Hidden: true,
		RunE: func(c *cobra.Command, args []string) error {
			target, _, findErr := c.Root().Find(args)
			if findErr != nil || (target == c.Root() && len(args) > 0) {
				return errcode.New(errcode.CodeValidationFailed,
					fmt.Sprintf("unknown help topic %q", strings.Join(args, " ")))
			}
			defaultHelp(target, args)
			return nil
		},
	})

	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return errcode.New(errcode.CodeValidationFailed, err.Error())
	})

	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.Err)
	cmd.SetIn(f.IOStreams.In)

	cmd.AddCommand(NewVersionCmd(f))
	cmd.AddCommand(NewGuideCmd(f))
	cmd.AddCommand(auth.NewAuthCmd(f))
	cmd.AddCommand(cluster.NewClusterCmd(f))
	cmd.AddCommand(region.NewRegionCmd(f))
	cmd.AddCommand(profile.NewProfileCmd(f))
	cmd.AddCommand(skill.NewSkillCmd(f))

	login := auth.NewLoginCmd(f)
	login.Use = "login"
	login.Short = "Alias for 'auth login'"
	cmd.AddCommand(login)

	return cmd
}
