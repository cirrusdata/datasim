package fileset

import (
	"time"

	"github.com/cirrusdata/datasim/internal/manifest"
)

const (
	currentGeneratorVersion = 1
	deterministicEpochUnix  = 946684800
	deterministicClockSpan  = 10 * 365 * 24 * time.Hour
	rotationClockSpan       = 45 * 24 * time.Hour
)

// generatorVersionForManifest returns the planner version that should drive deterministic replay.
func generatorVersionForManifest(doc *manifest.Manifest) int {
	if doc != nil && doc.GeneratorVersion > 0 {
		return doc.GeneratorVersion
	}

	return currentGeneratorVersion
}

// deterministicPlanningClock returns a stable planning timestamp derived from the request seed.
func deterministicPlanningClock(seed int64, version int) time.Time {
	offset := durationFromSeed(seed, int64(version), deterministicClockSpan)
	return time.Unix(deterministicEpochUnix, 0).UTC().Add(offset).Truncate(time.Second)
}

// deterministicRotationClock returns a stable rotation timestamp that always advances beyond the latest tracked mtime.
func deterministicRotationClock(doc *manifest.Manifest, seed int64) time.Time {
	latest := latestModifiedAt(doc.Files)
	if latest.IsZero() {
		latest = deterministicPlanningClock(seed, generatorVersionForManifest(doc))
	}

	offset := durationFromSeed(seed, int64(len(doc.History)+1), rotationClockSpan)
	return latest.UTC().Truncate(time.Second).Add(time.Second + offset)
}

// latestModifiedAt returns the latest tracked modified time in the manifest inventory.
func latestModifiedAt(files []manifest.FileRecord) time.Time {
	var latest time.Time
	for _, file := range files {
		modifiedAt := file.ModifiedAt.UTC().Truncate(time.Second)
		if modifiedAt.After(latest) {
			latest = modifiedAt
		}
	}

	return latest
}

// durationFromSeed returns a deterministic duration bounded by the provided span.
func durationFromSeed(first int64, second int64, span time.Duration) time.Duration {
	if span <= 0 {
		return 0
	}

	seconds := int64(span / time.Second)
	if seconds <= 0 {
		return 0
	}

	return time.Duration(mixSeed(first, second)%uint64(seconds+1)) * time.Second
}

// mixSeed combines deterministic inputs into a stable positive hash.
func mixSeed(values ...int64) uint64 {
	const (
		offset = 1469598103934665603
		prime  = 1099511628211
		mask   = (uint64(1) << 63) - 1
	)

	hash := uint64(offset)
	for _, value := range values {
		hash ^= uint64(value)
		hash *= prime
	}

	return hash & mask
}
