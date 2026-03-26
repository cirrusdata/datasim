package cli

import (
	"github.com/cirrusdata/datasim/internal/app"
	"github.com/spf13/cobra"
)

// newUpdateCmd builds the top-level self-update command.
func newUpdateCmd(bootstrap *app.Bootstrap) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Update datasim to the latest stable release",
		GroupID: "aux",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := bootstrap.Updater.Update(cmd.Context())
			if err != nil {
				return err
			}

			if result.Updated {
				printSuccessBlock(
					cmd,
					"Updated datasim",
					detailRow{Label: "from", Value: result.CurrentVersion},
					detailRow{Label: "to", Value: result.LatestVersion},
					detailRow{Label: "release", Value: result.ReleaseURL},
				)
				return nil
			}

			printInfoBlock(
				cmd,
				"Datasim is already up to date",
				detailRow{Label: "version", Value: result.LatestVersion},
			)
			return nil
		},
	}

	return cmd
}
