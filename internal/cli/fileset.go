package cli

import (
	"github.com/cirrusdata/datasim/internal/app"
	"github.com/spf13/cobra"
)

// newFilesetCmd builds the fileset command group.
func newFilesetCmd(bootstrap *app.Bootstrap) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fileset",
		Short:   "Manage synthetic file-tree datasets",
		GroupID: "core",
	}

	cmd.AddCommand(
		newFilesetInitCmd(bootstrap),
		newFilesetRotateCmd(bootstrap),
		newFilesetStatusCmd(bootstrap),
		newFilesetDestroyCmd(bootstrap),
	)

	return cmd
}
