package cli

import (
	"fmt"

	"github.com/cirrusdata/datasim/internal/app"
	"github.com/spf13/cobra"
)

// newVersionCmd builds the version command.
func newVersionCmd(bootstrap *app.Bootstrap) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Show build and configuration details",
		GroupID: "aux",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"datasim %s\nrepository: %s\ncommit: %s\nbuilt: %s\nconfig: %s\n",
				bootstrap.Build.Version,
				bootstrap.Build.Repository,
				bootstrap.Build.Commit,
				bootstrap.Build.Date,
				bootstrap.Config.ConfigFile,
			)
		},
	}

	return cmd
}
