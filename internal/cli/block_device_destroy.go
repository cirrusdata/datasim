package cli

import (
	"context"

	"github.com/cirrusdata/datasim/internal/app"
	"github.com/spf13/cobra"
)

// newBlockDeviceDestroyCmd builds the block-device destroy command.
func newBlockDeviceDestroyCmd(bootstrap *app.Bootstrap) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy <mount-point>",
		Short: "Unmount and remove a datasim block-device mount",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := bootstrap.BlockDevice.Destroy(context.Background(), args[0]); err != nil {
				return err
			}

			printSuccessBlock(cmd, "Destroyed block-device mount", detailRow{Label: "mount", Value: args[0]})
			return nil
		},
	}

	return cmd
}
