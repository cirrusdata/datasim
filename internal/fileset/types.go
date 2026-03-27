package fileset

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cirrusdata/datasim/internal/manifest"
)

const (
	// StrategyBalanced uses the default profile-shaped file distribution.
	StrategyBalanced = "balanced"
	// StrategyRandom increases variability in generated and rotated content.
	StrategyRandom = "random"
	// historicalModifiedAtWindow defines how far back synthetic file mtimes may be randomized.
	historicalModifiedAtWindow = 5 * 365 * 24 * time.Hour
)

// Profile describes a built-in fileset flavor.
type Profile struct {
	Name            string
	Description     string
	Directories     []string
	Nouns           []string
	Extensions      map[string][]string
	Prefixes        []string
	CategoryWeights map[string]float64
	DirectoryRules  []DirectoryRule
	SizeTiers       []SizeTier
}

// DirectoryRule maps directory keywords to a content category.
type DirectoryRule struct {
	Keywords []string
	Category string
}

// SizeTier defines one weighted size bucket for file generation.
type SizeTier struct {
	Name     string
	Weight   float64
	MinBytes int64
	MaxBytes int64
}

// InitRequest describes a fileset initialization plan request.
type InitRequest struct {
	Root           string
	TargetBytes    int64
	PreferredFiles int
	Seed           int64
	Strategy       string
}

// RotateRequest describes a fileset rotation plan request.
type RotateRequest struct {
	Manifest  *manifest.Manifest
	CreatePct float64
	DeletePct float64
	ModifyPct float64
	Seed      int64
	Strategy  string
}

// InitPlan is the planned file inventory for initialization.
type InitPlan struct {
	Files []FileSpec
}

// RotatePlan is the planned mutation set for a rotation.
type RotatePlan struct {
	Creates   []FileSpec
	Deletes   []string
	Mutations []Mutation
}

// FileSpec describes one generated file.
type FileSpec struct {
	RelativePath string
	Size         int64
	Seed         int64
	Mode         os.FileMode
	ModifiedAt   time.Time
	Labels       map[string]string
}

// Mutation describes a single file change during rotation.
type Mutation struct {
	RelativePath string
	Action       MutationAction
	NewSize      int64
	Seed         int64
	ModifiedAt   time.Time
}

// MutationAction identifies the type of file mutation to apply.
type MutationAction string

const (
	// MutationRewrite rewrites a file in place with new content.
	MutationRewrite MutationAction = "rewrite"
	// MutationAppend appends data to an existing file.
	MutationAppend MutationAction = "append"
	// MutationTruncate shrinks an existing file.
	MutationTruncate MutationAction = "truncate"
)

// SupportedStrategies returns the valid fileset planning strategies.
func SupportedStrategies() []string {
	return []string{StrategyBalanced, StrategyRandom}
}

// DescribeStrategy returns a short user-facing description for a strategy.
func DescribeStrategy(name string) string {
	switch name {
	case StrategyBalanced:
		return "default profile-shaped distribution with steady file counts and churn"
	case StrategyRandom:
		return "higher-variance distribution with more irregular file counts, sizes, and churn"
	default:
		return ""
	}
}

// ValidateStrategy validates that a fileset strategy name is supported.
func ValidateStrategy(name string) error {
	for _, strategy := range SupportedStrategies() {
		if name == strategy {
			return nil
		}
	}

	return fmt.Errorf("unknown fileset strategy %q", name)
}

// PlanInit builds a fileset initialization plan for a profile.
func (p Profile) PlanInit(_ context.Context, req InitRequest) (InitPlan, error) {
	if req.TargetBytes <= 0 {
		return InitPlan{}, fmt.Errorf("target bytes must be positive")
	}
	if err := ValidateStrategy(req.Strategy); err != nil {
		return InitPlan{}, err
	}

	rng := rand.New(rand.NewSource(req.Seed))
	fileCount := req.PreferredFiles
	if fileCount <= 0 {
		fileCount = p.estimateFileCount(req.TargetBytes)
	}
	if req.Strategy == StrategyRandom {
		factor := 0.5 + rng.Float64()*1.5
		fileCount = max(1, int(math.Round(float64(fileCount)*factor)))
	}

	sizes := allocateSizes(req.TargetBytes, fileCount, rng, p.SizeTiers)
	now := time.Now().UTC()
	files := make([]FileSpec, 0, len(sizes))

	for i, size := range sizes {
		dir := p.Directories[rng.Intn(len(p.Directories))]
		category := p.categoryForDirectory(dir, rng)
		ext := pickExtension(rng, p.Extensions, category)
		base := p.fileBaseName(rng, category, i)
		rel := filepath.ToSlash(filepath.Join(dir, base+ext))

		files = append(files, FileSpec{
			RelativePath: rel,
			Size:         size,
			Seed:         req.Seed + int64(i+1),
			Mode:         pickMode(rng),
			ModifiedAt:   randomHistoricalModifiedAt(rng, now),
			Labels: map[string]string{
				"workload": "fileset",
				"profile":  p.Name,
				"category": category,
			},
		})
	}

	return InitPlan{Files: dedupeFiles(files)}, nil
}

// PlanRotate builds a fileset rotation plan for a profile.
func (p Profile) PlanRotate(_ context.Context, req RotateRequest) (RotatePlan, error) {
	if req.Manifest == nil {
		return RotatePlan{}, fmt.Errorf("manifest is required")
	}
	if err := ValidateStrategy(req.Strategy); err != nil {
		return RotatePlan{}, err
	}

	rng := rand.New(rand.NewSource(req.Seed))
	current := append([]manifest.FileRecord(nil), req.Manifest.Files...)

	deleteCount := percentCount(len(current), req.DeletePct)
	modifyCount := percentCount(len(current), req.ModifyPct)
	createCount := percentCount(len(current), req.CreatePct)

	rng.Shuffle(len(current), func(i, j int) {
		current[i], current[j] = current[j], current[i]
	})

	deletes := make([]string, 0, deleteCount)
	deletedBytes := int64(0)
	for _, record := range current[:min(deleteCount, len(current))] {
		deletes = append(deletes, record.Path)
		deletedBytes += record.Size
	}

	remaining := current[min(deleteCount, len(current)):]
	mutations := make([]Mutation, 0, modifyCount)
	rotationNow := time.Now().UTC()
	for idx, record := range remaining[:min(modifyCount, len(remaining))] {
		action := []MutationAction{MutationRewrite, MutationAppend, MutationTruncate}[rng.Intn(3)]
		newSize := record.Size

		switch action {
		case MutationRewrite:
		case MutationAppend:
			newSize = record.Size + max(4*1024, int64(float64(record.Size)*0.25))
		case MutationTruncate:
			newSize = max(512, int64(float64(record.Size)*0.6))
		}

		mutations = append(mutations, Mutation{
			RelativePath: record.Path,
			Action:       action,
			NewSize:      newSize,
			Seed:         req.Seed + int64(idx+1),
			ModifiedAt:   randomMutationModifiedAt(rng, record.ModifiedAt, rotationNow),
		})
	}

	targetBytes := deletedBytes
	if targetBytes == 0 {
		targetBytes = max(int64(float64(totalBytes(current))*0.05), 4*1024)
	}
	if req.Strategy == StrategyRandom {
		targetBytes = int64(float64(targetBytes) * 1.2)
	}
	if createCount == 0 && deleteCount > 0 {
		createCount = deleteCount
	}
	if createCount == 0 {
		createCount = max(1, len(current)/20)
	}

	initPlan, err := p.PlanInit(context.Background(), InitRequest{
		Root:           req.Manifest.Filesystem.Root,
		TargetBytes:    targetBytes,
		PreferredFiles: createCount,
		Seed:           req.Seed + 10_000,
		Strategy:       req.Strategy,
	})
	if err != nil {
		return RotatePlan{}, err
	}

	existing := make(map[string]struct{}, len(req.Manifest.Files))
	for _, file := range req.Manifest.Files {
		existing[file.Path] = struct{}{}
	}

	creates := make([]FileSpec, 0, len(initPlan.Files))
	for _, spec := range initPlan.Files {
		if _, ok := existing[spec.RelativePath]; ok {
			spec.RelativePath = filepath.ToSlash(filepath.Join(filepath.Dir(spec.RelativePath), fmt.Sprintf("rotated-%d-%s", req.Seed, filepath.Base(spec.RelativePath))))
		}
		creates = append(creates, spec)
	}

	return RotatePlan{
		Creates:   creates,
		Deletes:   deletes,
		Mutations: mutations,
	}, nil
}

// estimateFileCount derives a profile-appropriate file count from a target size.
func (p Profile) estimateFileCount(targetBytes int64) int {
	tiers := normalizedTiers(p.SizeTiers)
	avg := averageTierSize(tiers)
	if avg <= 0 {
		avg = 160 * 1024
	}

	count := int(math.Round(float64(targetBytes) / float64(avg)))
	return max(1, count)
}

// fileBaseName returns a realistic filename stem for a profile category.
func (p Profile) fileBaseName(rng *rand.Rand, category string, index int) string {
	prefix := p.Prefixes[rng.Intn(len(p.Prefixes))]
	noun := p.Nouns[rng.Intn(len(p.Nouns))]
	when := time.Now().UTC().Add(-time.Duration(rng.Intn(3000)) * time.Hour)
	stamp := when.Format("20060102")
	year := when.Format("2006")
	month := strings.ToLower(when.Format("Jan"))

	switch category {
	case "img":
		return fmt.Sprintf("IMG_%s_%04d", stamp, rng.Intn(9999))
	case "vid":
		return fmt.Sprintf("VID_%s_%04d", stamp, rng.Intn(9999))
	case "doc":
		return fmt.Sprintf("%s-%s-%d", prefix, noun, index+1)
	case "archive":
		return fmt.Sprintf("bundle-%s-%s", noun, stamp)
	case "code":
		return fmt.Sprintf("%s-%s", noun, prefix)
	case "sheet":
		return fmt.Sprintf("budget-%s-%s-Q%d", noun, year, rng.Intn(4)+1)
	case "log":
		return fmt.Sprintf("server-%s-%s", noun, stamp)
	case "db":
		return fmt.Sprintf("dump-%s-%s", noun, stamp)
	case "pres":
		return fmt.Sprintf("review-%s-%s", noun, year)
	case "audio":
		return fmt.Sprintf("recording-%s-%s", noun, stamp)
	case "misc":
		return fmt.Sprintf("export-%s-%s", noun, month)
	default:
		return fmt.Sprintf("%s-%s-%d", prefix, noun, rng.Intn(10000))
	}
}

// categoryForDirectory returns the category implied by a directory path.
func (p Profile) categoryForDirectory(dir string, rng *rand.Rand) string {
	for _, rule := range p.DirectoryRules {
		for _, keyword := range rule.Keywords {
			if strings.Contains(dir, keyword) {
				return rule.Category
			}
		}
	}

	if len(p.CategoryWeights) == 0 {
		keys := make([]string, 0, len(p.Extensions))
		for key := range p.Extensions {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		return keys[rng.Intn(len(keys))]
	}

	target := rng.Float64()
	accum := 0.0
	keys := make([]string, 0, len(p.CategoryWeights))
	for key := range p.CategoryWeights {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		accum += p.CategoryWeights[key]
		if target <= accum {
			return key
		}
	}

	return keys[len(keys)-1]
}

// pickMode selects a plausible file mode for generated files.
func pickMode(rng *rand.Rand) os.FileMode {
	modes := []os.FileMode{0o644, 0o640, 0o600, 0o664, 0o755, 0o700, 0o444}
	return modes[rng.Intn(len(modes))]
}

// allocateSizes spreads a byte target across a generated file set.
func allocateSizes(total int64, fileCount int, rng *rand.Rand, tiers []SizeTier) []int64 {
	normalized := normalizedTiers(tiers)
	minTier := minimumTierBytes(normalized)
	maxFiles := int(max(int64(1), total/max(int64(1), minTier)))
	if fileCount > maxFiles {
		fileCount = maxFiles
	}
	if fileCount <= 1 {
		return []int64{max(512, total)}
	}

	result := make([]int64, fileCount)
	remaining := total
	for i := 0; i < fileCount; i++ {
		left := fileCount - i
		if left == 1 {
			result[i] = max(512, remaining)
			break
		}

		tier := sampleTier(rng, normalized)
		minRemaining := int64(left-1) * minTier
		maxAllowed := max(minTier, remaining-minRemaining)
		size := tier.MinBytes
		if tier.MaxBytes > tier.MinBytes {
			size += int64(rng.Int63n(tier.MaxBytes - tier.MinBytes + 1))
		}
		size = min(size, maxAllowed)
		size = max(minTier, size)
		result[i] = size
		remaining -= size
	}

	if remaining != 0 {
		result[len(result)-1] += remaining
	}

	reconcileSizes(result, total, minTier)
	return result
}

// reconcileSizes adjusts a size allocation so it does not exceed the target total.
func reconcileSizes(sizes []int64, target int64, minimum int64) {
	var sum int64
	for _, size := range sizes {
		sum += size
	}

	if sum == target {
		return
	}

	if sum < target {
		sizes[len(sizes)-1] += target - sum
		return
	}

	over := sum - target
	for i := len(sizes) - 1; i >= 0 && over > 0; i-- {
		headroom := sizes[i] - minimum
		if headroom <= 0 {
			continue
		}
		delta := min(headroom, over)
		sizes[i] -= delta
		over -= delta
	}
}

// dedupeFiles ensures generated relative paths remain unique.
func dedupeFiles(files []FileSpec) []FileSpec {
	seen := make(map[string]int, len(files))
	for idx := range files {
		original := files[idx].RelativePath
		if count, ok := seen[original]; ok {
			ext := filepath.Ext(original)
			base := strings.TrimSuffix(filepath.Base(original), ext)
			dir := filepath.Dir(original)
			files[idx].RelativePath = filepath.ToSlash(filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, count+1, ext)))
			seen[original] = count + 1
			continue
		}

		seen[original] = 1
	}

	return files
}

// pickExtension chooses an extension for a category.
func pickExtension(rng *rand.Rand, extensions map[string][]string, category string) string {
	exts, ok := extensions[category]
	if !ok || len(exts) == 0 {
		return ".dat"
	}

	return exts[rng.Intn(len(exts))]
}

// normalizedTiers returns the profile tiers or a generic fallback set.
func normalizedTiers(tiers []SizeTier) []SizeTier {
	if len(tiers) > 0 {
		return tiers
	}

	return []SizeTier{
		{Name: "tiny", Weight: 0.30, MinBytes: 64, MaxBytes: 4 * 1024},
		{Name: "small", Weight: 0.25, MinBytes: 4 * 1024, MaxBytes: 64 * 1024},
		{Name: "medium", Weight: 0.20, MinBytes: 64 * 1024, MaxBytes: 512 * 1024},
		{Name: "large", Weight: 0.15, MinBytes: 128 * 1024, MaxBytes: 512 * 1024},
		{Name: "xxl", Weight: 0.10, MinBytes: 512 * 1024, MaxBytes: 2 * 1024 * 1024},
	}
}

// averageTierSize computes the weighted average tier size.
func averageTierSize(tiers []SizeTier) int64 {
	if len(tiers) == 0 {
		return 0
	}

	totalWeight := 0.0
	average := 0.0
	for _, tier := range tiers {
		weight := tier.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
		average += weight * float64(tier.MinBytes+tier.MaxBytes) / 2
	}

	if totalWeight == 0 {
		return 0
	}

	return int64(average / totalWeight)
}

// minimumTierBytes returns the smallest minimum file size among all tiers.
func minimumTierBytes(tiers []SizeTier) int64 {
	minimum := tiers[0].MinBytes
	for _, tier := range tiers[1:] {
		if tier.MinBytes < minimum {
			minimum = tier.MinBytes
		}
	}
	return minimum
}

// sampleTier picks a size tier using the configured weights.
func sampleTier(rng *rand.Rand, tiers []SizeTier) SizeTier {
	target := rng.Float64()
	accum := 0.0
	totalWeight := 0.0
	for _, tier := range tiers {
		totalWeight += max(0.0001, tier.Weight)
	}

	for _, tier := range tiers {
		accum += max(0.0001, tier.Weight) / totalWeight
		if target <= accum {
			return tier
		}
	}

	return tiers[len(tiers)-1]
}

// percentCount converts a percentage into a file count.
func percentCount(total int, pct float64) int {
	return int(math.Round(float64(total) * pct / 100.0))
}

// totalBytes sums the size of a manifest file set.
func totalBytes(files []manifest.FileRecord) int64 {
	var sum int64
	for _, file := range files {
		sum += file.Size
	}
	return sum
}

// randomHistoricalModifiedAt returns a randomized historical mtime for a synthetic file.
func randomHistoricalModifiedAt(rng *rand.Rand, now time.Time) time.Time {
	return randomTimeBetween(rng, now.Add(-historicalModifiedAtWindow), now)
}

// randomMutationModifiedAt returns a randomized mutation time that still advances the file clock.
func randomMutationModifiedAt(rng *rand.Rand, previous time.Time, now time.Time) time.Time {
	if previous.IsZero() || !previous.Before(now) {
		return now.UTC()
	}

	return randomTimeBetween(rng, previous.Add(time.Second), now)
}

// randomTimeBetween returns a random UTC timestamp between the provided bounds.
func randomTimeBetween(rng *rand.Rand, start time.Time, end time.Time) time.Time {
	start = start.UTC().Truncate(time.Second)
	end = end.UTC().Truncate(time.Second)
	if !start.Before(end) {
		return end
	}

	spanSeconds := int64(end.Sub(start) / time.Second)
	if spanSeconds <= 0 {
		return end
	}

	return start.Add(time.Duration(rng.Int63n(spanSeconds+1)) * time.Second)
}
