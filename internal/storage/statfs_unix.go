//go:build !windows

package storage

import "syscall"

type Stats struct {
	CapacityBytes uint64
	FreeBytes     uint64
}

// Stat returns filesystem capacity and free-space statistics for a path.
func Stat(path string) (Stats, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return Stats{}, err
	}

	return Stats{
		CapacityBytes: fs.Blocks * uint64(fs.Bsize),
		FreeBytes:     fs.Bavail * uint64(fs.Bsize),
	}, nil
}
