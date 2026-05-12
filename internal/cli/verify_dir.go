package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cirrusdata/datasim/internal/app"
	verifysvc "github.com/cirrusdata/datasim/internal/verify"
	"github.com/cirrusdata/datasim/pkg/bytefmt"
	"github.com/spf13/cobra"
)

// newVerifyDirCmd builds the directory verification command.
func newVerifyDirCmd(bootstrap *app.Bootstrap) *cobra.Command {
	var (
		metadata   bool
		jsonOutput bool
		excludes   []string
	)

	cmd := &cobra.Command{
		Use:   "dir <source> <destination>",
		Short: "Compare two directory trees for migration-style integrity verification",
		Long: `Compare a source directory tree against a destination directory tree.

Verification checks relative paths, entry types, file sizes, content hashes, and,
by default, normalized file mode and modified time metadata.`,
		Example: strings.Join([]string{
			"  datasim verify dir /mnt/source /mnt/destination",
			"  datasim verify dir /mnt/source /mnt/destination --exclude .cirrusdata-datasim",
			"  datasim verify dir /mnt/source /mnt/destination --metadata=false --json",
		}, "\n"),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			reporter := newProgressRenderer(cmd)
			defer reporter.Finish()

			opts := verifysvc.DirOptions{
				SourceRoot:      args[0],
				DestinationRoot: args[1],
				Metadata:        metadata,
				Excludes:        excludes,
				Workers:         readVerifyWorkers(cmd),
				Progress:        newVerifyProgressFunc(reporter),
			}

			result, err := bootstrap.Verifier.VerifyDir(cmd.Context(), opts)
			if err != nil {
				return err
			}
			reporter.Finish()

			if jsonOutput {
				if err := printVerifyJSON(cmd, result); err != nil {
					return err
				}
			} else {
				printVerifySummary(cmd, result)
			}

			if !result.Matched {
				return newExitError(1, "")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&metadata, "metadata", true, "Compare normalized file mode and modified time metadata in addition to content")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print the verification result as JSON")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "Exclude a relative-path or glob pattern from verification; may be repeated")
	cmd.Flags().Int("concurrent", verifysvc.DefaultWorkerCount(), verifyConcurrentFlagUsage())
	return cmd
}

// verifyConcurrentFlagUsage returns the user-facing verifier concurrency description.
func verifyConcurrentFlagUsage() string {
	return fmt.Sprintf("Maximum number of verification hashing operations to run concurrently; defaults to %d", verifysvc.DefaultWorkerCount())
}

// readVerifyWorkers reads the verification worker count from the command.
func readVerifyWorkers(cmd *cobra.Command) int {
	workers, _ := cmd.Flags().GetInt("concurrent")
	return workers
}

// printVerifyJSON writes the structured verification result to stdout.
func printVerifyJSON(cmd *cobra.Command, result verifysvc.Result) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// printVerifySummary writes a human-readable verification summary.
func printVerifySummary(cmd *cobra.Command, result verifysvc.Result) {
	rows := []detailRow{
		{Label: "source", Value: result.SourceRoot},
		{Label: "destination", Value: result.DestinationRoot},
		{Label: "metadata", Value: fmt.Sprintf("%t", result.Metadata)},
		{Label: "source entries", Value: fmt.Sprintf("%d", result.TotalSourceEntries)},
		{Label: "destination entries", Value: fmt.Sprintf("%d", result.TotalDestinationEntries)},
		{Label: "matched files", Value: fmt.Sprintf("%d", result.MatchedFiles)},
		{Label: "matched dirs", Value: fmt.Sprintf("%d", result.MatchedDirectories)},
		{Label: "compared files", Value: fmt.Sprintf("%d", result.ComparedFiles)},
		{Label: "compared bytes", Value: fmt.Sprintf("%s (%d bytes)", bytefmt.Format(result.ComparedBytes), result.ComparedBytes)},
	}

	if result.Matched {
		printSuccessBlock(cmd, "Verification matched", rows...)
		return
	}

	rows = append(rows, detailRow{Label: "differences", Value: fmt.Sprintf("%d", len(result.Differences))})
	printDangerBlock(cmd, "Verification found differences", rows...)
	printVerifyDifferences(cmd, result.Differences)
}

// printVerifyDifferences renders each mismatch in a compact human-readable form.
func printVerifyDifferences(cmd *cobra.Command, differences []verifysvc.Difference) {
	for _, difference := range differences {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", difference.Path, difference.Message)
	}
}
