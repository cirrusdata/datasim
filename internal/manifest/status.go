package manifest

import "time"

// RefreshStatus recalculates the manifest's current status summary.
func RefreshStatus(doc *Manifest, action string, at time.Time, created int, deleted int, modified int) {
	categoryCounts := make(map[string]int)
	var totalBytes int64

	for _, file := range doc.Files {
		totalBytes += file.Size
		if category, ok := file.Labels["category"]; ok && category != "" {
			categoryCounts[category]++
		}
	}

	doc.Status = Status{
		State:          "ready",
		FileCount:      len(doc.Files),
		TotalBytes:     totalBytes,
		CategoryCounts: categoryCounts,
		RotationCount:  len(doc.History),
		LastAction:     action,
		LastActionAt:   at,
		LastCreated:    created,
		LastDeleted:    deleted,
		LastModified:   modified,
	}
}
