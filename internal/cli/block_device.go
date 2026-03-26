package cli

import (
	"github.com/cirrusdata/datasim/internal/app"
	"github.com/spf13/cobra"
)

// newBlockDeviceCmd builds the block-device command group.
func newBlockDeviceCmd(bootstrap *app.Bootstrap) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "block-device",
		Short:   "Manage block devices used for datasim workloads",
		GroupID: "storage",
	}

	cmd.AddCommand(
		newBlockDeviceFormatCmd(bootstrap),
		newBlockDeviceDestroyCmd(bootstrap),
	)

	return cmd
}
