//go:build windows

package storage

import (
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

type Stats struct {
	CapacityBytes uint64
	FreeBytes     uint64
}

// Stat returns filesystem capacity and free-space statistics for a path.
func Stat(path string) (Stats, error) {
	root := filepath.VolumeName(path) + `\`
	p, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return Stats{}, err
	}

	var freeBytes uint64
	var capacityBytes uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeBytes, &capacityBytes, nil); err != nil {
		return Stats{}, err
	}

	return Stats{
		CapacityBytes: capacityBytes,
		FreeBytes:     freeBytes,
	}, nil
}
