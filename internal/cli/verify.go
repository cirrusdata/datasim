package cli

import (
	"github.com/cirrusdata/datasim/internal/app"
	"github.com/spf13/cobra"
)

// newVerifyCmd builds the top-level verification command group.
func newVerifyCmd(bootstrap *app.Bootstrap) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "verify",
		Short:   "Compare source and destination data for integrity validation",
		GroupID: "aux",
	}

	cmd.AddCommand(newVerifyDirCmd(bootstrap))
	return cmd
}
