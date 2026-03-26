package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/cirrusdata/datasim/internal/app"
	"github.com/spf13/cobra"
)

// newFilesetRotateLoopCmd builds the long-running fileset rotate loop command.
func newFilesetRotateLoopCmd(bootstrap *app.Bootstrap) *cobra.Command {
	var (
		interval   time.Duration
		iterations int
	)

	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Run fileset rotation repeatedly on a schedule",
		Long: `Run the same fileset rotation workflow repeatedly at a fixed interval.

Use --iterations 0 to continue until interrupted.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := readFilesetRotateOptions(cmd)
			ctx, stop := signal.NotifyContext(context.Background(), interruptSignals()...)
			defer stop()

			run := func(iteration int) error {
				reporter := newProgressRenderer(cmd)
				opts.Progress = reporter.Update
				defer reporter.Finish()

				doc, err := bootstrap.Fileset.Rotate(ctx, opts)
				if err != nil {
					return err
				}
				reporter.Finish()

				history := doc.History[len(doc.History)-1]
				printSuccessBlock(
					cmd,
					fmt.Sprintf("Completed rotation %d", iteration),
					detailRow{Label: "root", Value: doc.Filesystem.Root},
					detailRow{Label: "created", Value: fmt.Sprintf("%d", history.Created)},
					detailRow{Label: "deleted", Value: fmt.Sprintf("%d", history.Deleted)},
					detailRow{Label: "modified", Value: fmt.Sprintf("%d", history.Modified)},
				)
				return nil
			}

			for i := 1; iterations == 0 || i <= iterations; i++ {
				if err := run(i); err != nil {
					return err
				}
				if iterations > 0 && i == iterations {
					break
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
			}

			return nil
		},
	}

	bindFilesetRotateFlags(cmd)
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Minute, "Sleep interval between rotation runs")
	cmd.Flags().IntVar(&iterations, "iterations", 0, "Number of rotations to run; 0 means run until interrupted")
	return cmd
}

// interruptSignals returns the signals that should stop a loop command.
func interruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
