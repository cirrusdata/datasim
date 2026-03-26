package cli

import (
	"context"

	"github.com/cirrusdata/datasim/internal/app"
	"github.com/cirrusdata/datasim/internal/filesystem"
	"github.com/spf13/cobra"
)

// newBlockDeviceFormatCmd builds the block-device format command.
func newBlockDeviceFormatCmd(bootstrap *app.Bootstrap) *cobra.Command {
	opts := &blockDeviceFormatOptions{}

	cmd := &cobra.Command{
		Use:   "format <block-device> <mount-point>",
		Short: "Format and mount a block device for datasim use",
		Long: `Format a block device with the platform default filesystem and mount it at
the provided mount point. Linux defaults to XFS and Windows defaults to NTFS.
On Windows, the mount point may be a drive letter such as X:\ or a directory
path such as C:\datasim-mount.

By default, this command refuses to reuse an already-mounted target or block
device. Use --force when you want datasim to unmount the existing filesystem
before recreating it.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			record, err := bootstrap.BlockDevice.Format(context.Background(), args[0], args[1], filesystem.FormatOptions{
				FSType: opts.fsType,
				Force:  opts.force,
			})
			if err != nil {
				return err
			}

			printSuccessBlock(
				cmd,
				"Prepared block device",
				detailRow{Label: "device", Value: record.BlockDevice},
				detailRow{Label: "mount", Value: record.MountPoint},
				detailRow{Label: "fstype", Value: record.FSType},
			)
			return nil
		},
	}

	bindBlockDeviceFormatFlags(cmd, bootstrap, opts)
	return cmd
}

// blockDeviceFormatOptions stores CLI flags for block-device format commands.
type blockDeviceFormatOptions struct {
	fsType string
	force  bool
}

// bindBlockDeviceFormatFlags registers the shared block-device format flags.
func bindBlockDeviceFormatFlags(cmd *cobra.Command, bootstrap *app.Bootstrap, opts *blockDeviceFormatOptions) {
	cmd.Flags().StringVar(&opts.fsType, "fstype", bootstrap.Config.DefaultFSType(), "Filesystem type to create")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Unmount and recreate an already-mounted target or block device")
}
