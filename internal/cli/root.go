package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/cirrusdata/datasim/internal/app"
	"github.com/cirrusdata/datasim/internal/fileset"
	"github.com/spf13/cobra"
)

// rootOptions stores persistent options shared across the CLI.
type rootOptions struct {
	configFile    string
	color         string
	noProgressBar bool
}

// NewRootCmd constructs the top-level datasim command tree.
func NewRootCmd(bootstrap *app.Bootstrap) *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:   "datasim",
		Short: "Create and rotate synthetic datasets for migration testing",
		Long: `datasim is a cross-platform CLI for preparing synthetic datasets for
migration, sync, and integrity validation workflows.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateColorMode(opts.color); err != nil {
				return err
			}
			if opts.configFile == "" {
				return nil
			}
			return bootstrap.Reload(opts.configFile)
		},
	}

	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.PersistentFlags().StringVar(&opts.configFile, "config", "", "Use a specific config file")
	cmd.PersistentFlags().StringVar(&opts.color, "color", colorModeAuto, "Colorize terminal output: auto, always, never")
	cmd.PersistentFlags().BoolVar(&opts.noProgressBar, "no-progress-bar", false, "Disable live progress rendering for long-running commands")

	cmd.AddGroup(
		&cobra.Group{ID: "core", Title: "Workload Commands"},
		&cobra.Group{ID: "storage", Title: "Storage Commands"},
		&cobra.Group{ID: "aux", Title: "Auxiliary Commands"},
	)

	cmd.AddCommand(
		newFilesetCmd(bootstrap),
		newBlockDeviceCmd(bootstrap),
		newUpdateCmd(bootstrap),
		newVersionCmd(bootstrap),
		newCompletionCmd(cmd),
	)

	cmd.SetHelpCommandGroupID("aux")
	return cmd
}

// profileCompletion returns fileset profile names for shell completion.
func profileCompletion(bootstrap *app.Bootstrap, toComplete string) ([]string, cobra.ShellCompDirective) {
	names := bootstrap.Fileset.Catalog().Names()
	results := make([]string, 0, len(names))
	for _, name := range names {
		if toComplete == "" || strings.HasPrefix(name, toComplete) {
			profile, err := bootstrap.Fileset.Catalog().Get(name)
			if err != nil {
				continue
			}
			results = append(results, name+"\t"+profile.Description)
		}
	}

	return results, cobra.ShellCompDirectiveNoFileComp
}

// profileFlagUsage returns a user-facing profile flag description.
func profileFlagUsage(bootstrap *app.Bootstrap) string {
	names := bootstrap.Fileset.Catalog().Names()
	parts := make([]string, 0, len(names))
	for _, name := range names {
		profile, err := bootstrap.Fileset.Catalog().Get(name)
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", name, profile.Description))
	}

	return "Fileset profile to initialize. Supported values: " + strings.Join(parts, "; ")
}

// strategyFlagUsage returns a user-facing strategy flag description.
func strategyFlagUsage() string {
	strategies := fileset.SupportedStrategies()
	parts := make([]string, 0, len(strategies))
	for _, name := range strategies {
		parts = append(parts, fmt.Sprintf("%s (%s)", name, fileset.DescribeStrategy(name)))
	}

	return "Fileset planning strategy. Supported values: " + strings.Join(parts, "; ")
}

// strategyCompletion returns strategy names for shell completion.
func strategyCompletion(toComplete string) ([]string, cobra.ShellCompDirective) {
	strategies := fileset.SupportedStrategies()
	results := make([]string, 0, len(strategies))
	for _, name := range strategies {
		if toComplete == "" || strings.HasPrefix(name, toComplete) {
			results = append(results, name+"\t"+fileset.DescribeStrategy(name))
		}
	}

	return results, cobra.ShellCompDirectiveNoFileComp
}

// progressBarDisabled returns whether live progress rendering should be disabled.
func progressBarDisabled(cmd *cobra.Command) bool {
	disabled, _ := cmd.Flags().GetBool("no-progress-bar")
	return disabled
}

// printCommandResult writes a formatted one-line result to stdout.
func printCommandResult(cmd *cobra.Command, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	theme := newTerminalTheme(cmd)
	if !theme.stdoutTTY {
		fmt.Fprintln(cmd.OutOrStdout(), message)
		return
	}

	printBlock(cmd, theme.symbolSuccess(), theme.bold(message))
}
