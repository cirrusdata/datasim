package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/cirrusdata/datasim/internal/app"
	"github.com/cirrusdata/datasim/internal/fileset"
	"github.com/cirrusdata/datasim/pkg/bytefmt"
	"github.com/spf13/cobra"
)

// newFilesetInitCmd builds the fileset init command.
func newFilesetInitCmd(bootstrap *app.Bootstrap) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a fileset dataset and write its manifest",
		Long: `Initialize a fileset dataset in a mounted filesystem and write the
datasim manifest that records the generation parameters and resulting inventory.

Profiles:
  corporate  Corporate share with finance, engineering, legal, marketing, and shared operations data.
  school     District or campus share with departments, classrooms, media labs, and student submissions.
  nasa       Mission-focused dataset with telemetry, experiments, imagery, software, and archive payloads.

Strategies:
  balanced   Default profile-shaped distribution with steadier file counts and sizes.
  random     Higher-variance generation with more irregular file counts and size allocation.`,
		Example: strings.Join([]string{
			"  datasim fileset init --fs /mnt/datasim-source --profile corporate --size 10GiB",
			"  datasim fileset init --fs /mnt/datasim-source --profile school --strategy random --size 2GiB",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := readFilesetInitOptions(cmd, bootstrap)
			reporter := newProgressRenderer(cmd)
			opts.Progress = reporter.Update
			defer reporter.Finish()
			doc, err := bootstrap.Fileset.Init(context.Background(), opts)
			if err != nil {
				return err
			}
			reporter.Finish()

			printSuccessBlock(
				cmd,
				"Initialized fileset",
				detailRow{Label: "profile", Value: doc.Profile},
				detailRow{Label: "root", Value: doc.Filesystem.Root},
				detailRow{Label: "seed", Value: fmt.Sprintf("%d", doc.Seed)},
				detailRow{Label: "files", Value: fmt.Sprintf("%d", doc.Status.FileCount)},
				detailRow{Label: "directories", Value: fmt.Sprintf("%d", countFilesetDirectories(doc.Files))},
				detailRow{Label: "size", Value: fmt.Sprintf("%s (%d bytes)", bytefmt.Format(doc.Status.TotalBytes), doc.Status.TotalBytes)},
			)
			return nil
		},
	}

	bindFilesetInitFlags(cmd, bootstrap)
	return cmd
}

// bindFilesetInitFlags registers shared fileset init flags.
func bindFilesetInitFlags(cmd *cobra.Command, bootstrap *app.Bootstrap) {
	cmd.Flags().String("fs", "", "Mounted filesystem root to populate")
	cmd.Flags().String("profile", bootstrap.Fileset.Catalog().DefaultProfileName(), profileFlagUsage(bootstrap))
	cmd.Flags().String("size", "", "Maximum dataset size; defaults to 80% of filesystem capacity")
	cmd.Flags().Int64("seed", 0, "Random seed for reproducible output")
	cmd.Flags().String("strategy", fileset.StrategyBalanced, strategyFlagUsage())
	_ = cmd.MarkFlagRequired("fs")
	_ = cmd.RegisterFlagCompletionFunc("profile", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return profileCompletion(bootstrap, toComplete)
	})
	_ = cmd.RegisterFlagCompletionFunc("strategy", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return strategyCompletion(toComplete)
	})
}

// readFilesetInitOptions reads fileset init options from a command.
func readFilesetInitOptions(cmd *cobra.Command, bootstrap *app.Bootstrap) fileset.InitOptions {
	root, _ := cmd.Flags().GetString("fs")
	profile, _ := cmd.Flags().GetString("profile")
	totalSize, _ := cmd.Flags().GetString("size")
	seed, _ := cmd.Flags().GetInt64("seed")
	strategy, _ := cmd.Flags().GetString("strategy")

	return fileset.InitOptions{
		Profile:   profile,
		Root:      root,
		TotalSize: totalSize,
		Seed:      seed,
		Strategy:  strategy,
	}
}
