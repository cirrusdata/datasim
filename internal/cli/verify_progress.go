package cli

import (
	"github.com/cirrusdata/datasim/internal/fileset"
	verifysvc "github.com/cirrusdata/datasim/internal/verify"
)

// newVerifyProgressFunc adapts verifier progress events into the shared CLI renderer format.
func newVerifyProgressFunc(reporter *progressRenderer) verifysvc.ProgressFunc {
	return func(progress verifysvc.Progress) {
		reporter.Update(fileset.Progress{
			Operation:      "verify",
			Phase:          progress.Phase,
			CurrentPath:    progress.CurrentPath,
			CurrentAction:  progress.CurrentAction,
			CompletedItems: progress.CompletedItems,
			TotalItems:     progress.TotalItems,
			CompletedBytes: progress.CompletedBytes,
			TotalBytes:     progress.TotalBytes,
		})
	}
}
