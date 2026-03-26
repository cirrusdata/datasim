package cli

import (
	"github.com/cirrusdata/datasim/internal/app"
	"github.com/cirrusdata/datasim/internal/fileset"
	"github.com/spf13/cobra"
)

// newFilesetDestroyCmd builds the fileset destroy command.
func newFilesetDestroyCmd(bootstrap *app.Bootstrap) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy <path>",
		Short: "Remove a fileset dataset and delete its manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reporter := newProgressRenderer(cmd)
			defer reporter.Finish()

			if err := bootstrap.Fileset.Destroy(fileset.DestroyOptions{
				Root:     args[0],
				Progress: reporter.Update,
			}); err != nil {
				return err
			}
			reporter.Finish()

			printSuccessBlock(cmd, "Destroyed fileset", detailRow{Label: "root", Value: args[0]})
			return nil
		},
	}

	return cmd
}
