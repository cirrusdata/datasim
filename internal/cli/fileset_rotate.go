package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/cirrusdata/datasim/internal/app"
	"github.com/cirrusdata/datasim/internal/fileset"
	"github.com/spf13/cobra"
)

// newFilesetRotateCmd builds the fileset rotate command.
func newFilesetRotateCmd(bootstrap *app.Bootstrap) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate an existing fileset by creating, deleting, and modifying files",
		Long: `Rotate an existing fileset by applying create, delete, and modify work
against the manifest-backed dataset state.

Strategies:
  balanced   Default profile-shaped churn with steady create, delete, and modify behavior.
  random     Higher-variance churn with more irregular work distribution.`,
		Example: strings.Join([]string{
			"  datasim fileset rotate --fs /mnt/datasim-source",
			"  datasim fileset rotate --fs /mnt/datasim-source --strategy random --create-pct 10 --delete-pct 2 --modify-pct 20",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := readFilesetRotateOptions(cmd)
			reporter := newProgressRenderer(cmd)
			opts.Progress = reporter.Update
			defer reporter.Finish()
			doc, err := bootstrap.Fileset.Rotate(context.Background(), opts)
			if err != nil {
				return err
			}
			reporter.Finish()

			history := doc.History[len(doc.History)-1]
			beforeCount := doc.Status.FileCount + history.Deleted - history.Created
			printSuccessBlock(
				cmd,
				"Rotated fileset",
				detailRow{Label: "root", Value: doc.Filesystem.Root},
				detailRow{Label: "created", Value: fmt.Sprintf("%d", history.Created), Value2: rotatePercent(history.Created, beforeCount)},
				detailRow{Label: "deleted", Value: fmt.Sprintf("%d", history.Deleted), Value2: rotatePercent(history.Deleted, beforeCount)},
				detailRow{Label: "modified", Value: fmt.Sprintf("%d", history.Modified), Value2: rotatePercent(history.Modified, beforeCount)},
				detailRow{Label: "total files", Value: fmt.Sprintf("%d", doc.Status.FileCount)},
				detailRow{Label: "directories", Value: fmt.Sprintf("%d", countFilesetDirectories(doc.Files))},
			)
			return nil
		},
	}

	bindFilesetRotateFlags(cmd)
	cmd.AddCommand(newFilesetRotateLoopCmd(bootstrap))
	return cmd
}

// rotatePercent returns a one-decimal percentage string for a rotate summary row.
func rotatePercent(count int, total int) string {
	if total <= 0 {
		return ""
	}

	return fmt.Sprintf("%4.1f%%", (100*float64(count))/float64(total))
}

// bindFilesetRotateFlags registers shared fileset rotate flags.
func bindFilesetRotateFlags(cmd *cobra.Command) {
	cmd.Flags().String("fs", "", "Mounted filesystem root to rotate")
	cmd.Flags().Float64("create-pct", 5, "Percentage of files to create")
	cmd.Flags().Float64("delete-pct", 5, "Percentage of files to delete")
	cmd.Flags().Float64("modify-pct", 10, "Percentage of files to modify")
	cmd.Flags().Int64("seed", 0, "Random seed for reproducible rotation")
	cmd.Flags().String("strategy", fileset.StrategyBalanced, strategyFlagUsage())
	_ = cmd.MarkFlagRequired("fs")
	_ = cmd.RegisterFlagCompletionFunc("strategy", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return strategyCompletion(toComplete)
	})
}

// readFilesetRotateOptions reads fileset rotate options from a command.
func readFilesetRotateOptions(cmd *cobra.Command) fileset.RotateOptions {
	root, _ := cmd.Flags().GetString("fs")
	createPct, _ := cmd.Flags().GetFloat64("create-pct")
	deletePct, _ := cmd.Flags().GetFloat64("delete-pct")
	modifyPct, _ := cmd.Flags().GetFloat64("modify-pct")
	seed, _ := cmd.Flags().GetInt64("seed")
	strategy, _ := cmd.Flags().GetString("strategy")

	return fileset.RotateOptions{
		Root:      root,
		CreatePct: createPct,
		DeletePct: deletePct,
		ModifyPct: modifyPct,
		Seed:      seed,
		Strategy:  strategy,
	}
}
