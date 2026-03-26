package cli

import (
	"encoding/json"
	"fmt"

	"github.com/cirrusdata/datasim/internal/app"
	"github.com/cirrusdata/datasim/pkg/bytefmt"
	"github.com/spf13/cobra"
)

// newFilesetStatusCmd builds the fileset status command.
func newFilesetStatusCmd(bootstrap *app.Bootstrap) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status <path>",
		Short: "Show manifest-backed fileset status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, err := bootstrap.Fileset.Status(args[0])
			if err != nil {
				return err
			}

			if jsonOutput {
				data, err := json.MarshalIndent(doc, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			printInfoBlock(
				cmd,
				"Fileset status",
				detailRow{Label: "workload", Value: doc.Workload},
				detailRow{Label: "profile", Value: doc.Profile},
				detailRow{Label: "root", Value: doc.Filesystem.Root},
				detailRow{Label: "state", Value: doc.Status.State},
				detailRow{Label: "files", Value: fmt.Sprintf("%d", doc.Status.FileCount)},
				detailRow{Label: "size", Value: fmt.Sprintf("%s (%d bytes)", bytefmt.Format(doc.Status.TotalBytes), doc.Status.TotalBytes)},
				detailRow{Label: "target", Value: fmt.Sprintf("%s (%d bytes)", bytefmt.Format(doc.Generation.TargetBytes), doc.Generation.TargetBytes)},
				detailRow{Label: "rotations", Value: fmt.Sprintf("%d", doc.Status.RotationCount)},
				detailRow{Label: "last action", Value: fmt.Sprintf("%s at %s", doc.Status.LastAction, doc.Status.LastActionAt.Format("2006-01-02 15:04:05 MST"))},
			)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print the full manifest as JSON")
	return cmd
}
