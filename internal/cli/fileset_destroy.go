package cli

import (
	"strings"

	"github.com/cirrusdata/datasim/internal/app"
	"github.com/cirrusdata/datasim/internal/fileset"
	"github.com/spf13/cobra"
)

// newFilesetDestroyCmd builds the fileset destroy command.
func newFilesetDestroyCmd(bootstrap *app.Bootstrap) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy <path>",
		Short: "Remove a fileset dataset and delete its manifest",
		Long: `Remove a manifest-backed fileset dataset from a local filesystem root or
S3-compatible object-store prefix.

Targets:
  /path/to/root                 Local filesystem root.
  s3://host/bucket/prefix       S3-compatible object-store prefix over HTTPS.
  s3+http://host/bucket/prefix  S3-compatible object-store prefix over HTTP.

S3 credentials are read from AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, and
AWS_SESSION_TOKEN when present. Credentials in S3 URLs are rejected.`,
		Example: strings.Join([]string{
			"  datasim fileset destroy /mnt/datasim-source",
			"  datasim fileset destroy s3://object.example.com/test-bucket/demo",
		}, "\n"),
		Args: cobra.ExactArgs(1),
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
