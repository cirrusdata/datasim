package cli

import (
	"path"

	"github.com/cirrusdata/datasim/internal/manifest"
)

// countFilesetDirectories returns the number of unique directories implied by file paths.
func countFilesetDirectories(files []manifest.FileRecord) int {
	dirs := make(map[string]struct{})
	for _, file := range files {
		dir := path.Dir(file.Path)
		for dir != "." && dir != "/" {
			dirs[dir] = struct{}{}
			next := path.Dir(dir)
			if next == dir {
				break
			}
			dir = next
		}
	}

	return len(dirs)
}
