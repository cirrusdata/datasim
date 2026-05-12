package verify

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// MetadataCollector collects normalized metadata for one filesystem entry.
type MetadataCollector interface {
	Collect(path string, info fs.FileInfo) (Metadata, error)
}

// Service verifies directory trees for migration-style integrity checks.
type Service struct {
	metadata MetadataCollector
}

// NewService constructs a verifier service with the provided metadata collector.
func NewService(metadata MetadataCollector) *Service {
	if metadata == nil {
		metadata = basicMetadataCollector{}
	}

	return &Service{metadata: metadata}
}

// DefaultWorkerCount returns the default verification concurrency.
func DefaultWorkerCount() int {
	return max(8, runtime.GOMAXPROCS(0))
}

// VerifyDir compares a source directory tree against a destination directory tree.
func (s *Service) VerifyDir(ctx context.Context, opts DirOptions) (Result, error) {
	if err := validateOptions(opts); err != nil {
		return Result{}, err
	}

	excludes, err := newExcludeMatcher(opts.Excludes)
	if err != nil {
		return Result{}, err
	}

	sourceEntries, err := s.scanDirectory(ctx, opts.SourceRoot, excludes)
	if err != nil {
		return Result{}, err
	}
	reportProgress(opts.Progress, Progress{
		Phase:          "scan",
		CurrentAction:  "source",
		CompletedItems: len(sourceEntries),
		TotalItems:     len(sourceEntries),
	})

	destinationEntries, err := s.scanDirectory(ctx, opts.DestinationRoot, excludes)
	if err != nil {
		return Result{}, err
	}
	reportProgress(opts.Progress, Progress{
		Phase:          "scan",
		CurrentAction:  "destination",
		CompletedItems: len(destinationEntries),
		TotalItems:     len(destinationEntries),
	})

	result := Result{
		SourceRoot:              opts.SourceRoot,
		DestinationRoot:         opts.DestinationRoot,
		Metadata:                opts.Metadata,
		Excludes:                excludes.Patterns(),
		TotalSourceEntries:      len(sourceEntries),
		TotalDestinationEntries: len(destinationEntries),
	}

	paths := collectPaths(sourceEntries, destinationEntries)
	fileTasks := make([]fileTask, 0)

	for _, relPath := range paths {
		sourceEntry, sourceOK := sourceEntries[relPath]
		destinationEntry, destinationOK := destinationEntries[relPath]

		switch {
		case sourceOK && !destinationOK:
			result.Differences = append(result.Differences, Difference{
				Path:    relPath,
				Kind:    DifferenceMissingFromDestination,
				Message: "entry is missing from destination",
				Source:  sourceEntry.ResultEntry(),
			})
			continue
		case !sourceOK && destinationOK:
			result.Differences = append(result.Differences, Difference{
				Path:        relPath,
				Kind:        DifferenceExtraInDestination,
				Message:     "entry exists only in destination",
				Destination: destinationEntry.ResultEntry(),
			})
			continue
		}

		if sourceEntry.Type != destinationEntry.Type {
			result.Differences = append(result.Differences, Difference{
				Path:        relPath,
				Kind:        DifferenceTypeMismatch,
				Message:     fmt.Sprintf("entry type differs: source=%s destination=%s", sourceEntry.Type, destinationEntry.Type),
				Source:      sourceEntry.ResultEntry(),
				Destination: destinationEntry.ResultEntry(),
			})
			continue
		}

		if sourceEntry.Unsupported() || destinationEntry.Unsupported() {
			result.Differences = append(result.Differences, Difference{
				Path:        relPath,
				Kind:        DifferenceUnsupportedType,
				Message:     fmt.Sprintf("unsupported entry type %q", sourceEntry.Type),
				Source:      sourceEntry.ResultEntry(),
				Destination: destinationEntry.ResultEntry(),
			})
			continue
		}

		switch sourceEntry.Type {
		case EntryTypeDirectory:
			result.MatchedDirectories++
		case EntryTypeFile:
			differenceCount := len(result.Differences)
			if opts.Metadata {
				result.Differences = append(result.Differences, compareMetadata(relPath, sourceEntry, destinationEntry)...)
			}
			if sourceEntry.Size != destinationEntry.Size {
				result.Differences = append(result.Differences, Difference{
					Path:        relPath,
					Kind:        DifferenceSizeMismatch,
					Message:     fmt.Sprintf("file size differs: source=%d destination=%d", sourceEntry.Size, destinationEntry.Size),
					Source:      sourceEntry.ResultEntry(),
					Destination: destinationEntry.ResultEntry(),
				})
				continue
			}

			fileTasks = append(fileTasks, fileTask{
				Path:        relPath,
				Source:      sourceEntry,
				Destination: destinationEntry,
				BaseMatched: len(result.Differences) == differenceCount,
			})
		}
	}

	hashDifferences, matchedFiles, comparedFiles, comparedBytes, err := compareFiles(ctx, fileTasks, normalizeWorkerCount(opts.Workers, len(fileTasks)), opts.Progress)
	if err != nil {
		return Result{}, err
	}

	result.Differences = append(result.Differences, hashDifferences...)
	result.MatchedFiles = matchedFiles
	result.ComparedFiles = comparedFiles
	result.ComparedBytes = comparedBytes
	sortDifferences(result.Differences)
	result.Matched = len(result.Differences) == 0

	return result, nil
}

// basicMetadataCollector collects cross-platform mode and mtime metadata.
type basicMetadataCollector struct{}

// Collect normalizes the supported metadata fields for one filesystem entry.
func (basicMetadataCollector) Collect(_ string, info fs.FileInfo) (Metadata, error) {
	return Metadata{
		Mode:       normalizeMode(info.Mode()),
		ModifiedAt: normalizeModifiedTime(info.ModTime()),
	}, nil
}

type scannedEntry struct {
	RelativePath string
	FullPath     string
	Type         EntryType
	Size         int64
	Metadata     Metadata
}

// ResultEntry converts a scanned entry into the public result representation.
func (e scannedEntry) ResultEntry() *Entry {
	return &Entry{
		Path:     e.RelativePath,
		Type:     e.Type,
		Size:     e.Size,
		Metadata: e.Metadata,
	}
}

// Unsupported reports whether the scanned entry type is unsupported for verification.
func (e scannedEntry) Unsupported() bool {
	return e.Type != EntryTypeFile && e.Type != EntryTypeDirectory
}

type fileTask struct {
	Path        string
	Source      scannedEntry
	Destination scannedEntry
	BaseMatched bool
}

type fileResult struct {
	Difference    *Difference
	Matched       bool
	ComparedFile  bool
	ComparedBytes int64
}

type excludeMatcher struct {
	patterns []string
}

// Patterns returns the normalized exclude patterns configured for the matcher.
func (m excludeMatcher) Patterns() []string {
	return append([]string(nil), m.patterns...)
}

// Match reports whether the relative path should be excluded.
func (m excludeMatcher) Match(relPath string) bool {
	if relPath == "" {
		return false
	}

	base := path.Base(relPath)
	for _, pattern := range m.patterns {
		if relPath == pattern || base == pattern {
			return true
		}

		if ok, _ := path.Match(pattern, relPath); ok {
			return true
		}
		if !strings.Contains(pattern, "/") {
			if ok, _ := path.Match(pattern, base); ok {
				return true
			}
		}
		if strings.HasPrefix(relPath, pattern+"/") {
			return true
		}
	}

	return false
}

// validateOptions rejects invalid verification options.
func validateOptions(opts DirOptions) error {
	if strings.TrimSpace(opts.SourceRoot) == "" {
		return fmt.Errorf("source root is required")
	}
	if strings.TrimSpace(opts.DestinationRoot) == "" {
		return fmt.Errorf("destination root is required")
	}
	if err := validateWorkerCount(opts.Workers); err != nil {
		return err
	}
	if err := validateDirectory(opts.SourceRoot); err != nil {
		return fmt.Errorf("source root: %w", err)
	}
	if err := validateDirectory(opts.DestinationRoot); err != nil {
		return fmt.Errorf("destination root: %w", err)
	}

	return nil
}

// validateDirectory rejects paths that are missing or not directories.
func validateDirectory(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", root)
	}

	return nil
}

// newExcludeMatcher validates and normalizes exclude patterns.
func newExcludeMatcher(patterns []string) (excludeMatcher, error) {
	normalized := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if _, err := path.Match(pattern, "probe"); err != nil {
			return excludeMatcher{}, fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
		}
		normalized = append(normalized, strings.TrimPrefix(pattern, "./"))
	}

	return excludeMatcher{patterns: normalized}, nil
}

// scanDirectory walks one directory tree into a normalized relative-path inventory.
func (s *Service) scanDirectory(ctx context.Context, root string, excludes excludeMatcher) (map[string]scannedEntry, error) {
	entries := make(map[string]scannedEntry)

	if err := filepath.WalkDir(root, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if currentPath == root {
			return nil
		}

		relPath, err := filepath.Rel(root, currentPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if excludes.Match(relPath) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		metadata, err := s.metadata.Collect(currentPath, info)
		if err != nil {
			return err
		}

		entries[relPath] = scannedEntry{
			RelativePath: relPath,
			FullPath:     currentPath,
			Type:         classifyEntryType(info.Mode()),
			Size:         fileSize(info),
			Metadata:     metadata,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return entries, nil
}

// classifyEntryType converts a filesystem mode into the verifier entry type.
func classifyEntryType(mode fs.FileMode) EntryType {
	switch {
	case mode.IsRegular():
		return EntryTypeFile
	case mode.IsDir():
		return EntryTypeDirectory
	case mode&fs.ModeSymlink != 0:
		return EntryTypeSymlink
	default:
		return EntryTypeOther
	}
}

// fileSize returns a normalized size for the scanned entry.
func fileSize(info fs.FileInfo) int64 {
	if info.Mode().IsRegular() {
		return info.Size()
	}

	return 0
}

// collectPaths returns the sorted union of source and destination inventory keys.
func collectPaths(source map[string]scannedEntry, destination map[string]scannedEntry) []string {
	seen := make(map[string]struct{}, len(source)+len(destination))
	for relPath := range source {
		seen[relPath] = struct{}{}
	}
	for relPath := range destination {
		seen[relPath] = struct{}{}
	}

	paths := make([]string, 0, len(seen))
	for relPath := range seen {
		paths = append(paths, relPath)
	}
	slices.Sort(paths)

	return paths
}

// compareMetadata reports metadata differences for one matched entry pair.
func compareMetadata(relPath string, source scannedEntry, destination scannedEntry) []Difference {
	differences := make([]Difference, 0, 2)

	if source.Metadata.Mode != destination.Metadata.Mode {
		differences = append(differences, Difference{
			Path:        relPath,
			Kind:        DifferenceModeMismatch,
			Message:     fmt.Sprintf("mode differs: source=%s destination=%s", source.Metadata.Mode, destination.Metadata.Mode),
			Source:      source.ResultEntry(),
			Destination: destination.ResultEntry(),
		})
	}

	if !source.Metadata.ModifiedAt.Equal(destination.Metadata.ModifiedAt) {
		differences = append(differences, Difference{
			Path:        relPath,
			Kind:        DifferenceModifiedTimeMismatch,
			Message:     fmt.Sprintf("modified time differs: source=%s destination=%s", source.Metadata.ModifiedAt.Format(time.RFC3339), destination.Metadata.ModifiedAt.Format(time.RFC3339)),
			Source:      source.ResultEntry(),
			Destination: destination.ResultEntry(),
		})
	}

	return differences
}

// compareFiles hashes and compares each regular file pair.
func compareFiles(ctx context.Context, tasks []fileTask, workers int, progress ProgressFunc) ([]Difference, int, int, int64, error) {
	if len(tasks) == 0 {
		return nil, 0, 0, 0, nil
	}

	totalBytes := int64(0)
	for _, task := range tasks {
		totalBytes += task.Source.Size + task.Destination.Size
	}
	reportProgress(progress, Progress{
		Phase:         "hash",
		CurrentAction: "compare-content",
		TotalItems:    len(tasks),
		TotalBytes:    totalBytes,
	})

	results := make([]fileResult, len(tasks))
	var completedItems atomic.Int64
	var completedBytes atomic.Int64
	emitter := newProgressEmitter(progress)

	if err := runParallel(ctx, workers, len(tasks), func(index int) error {
		task := tasks[index]
		sourceHash, err := checksumFile(task.Source.FullPath, func(written int64) {
			bytes := completedBytes.Add(written)
			emitter.Report(Progress{
				Phase:          "hash",
				CurrentPath:    task.Path,
				CurrentAction:  "source",
				CompletedItems: int(completedItems.Load()),
				TotalItems:     len(tasks),
				CompletedBytes: bytes,
				TotalBytes:     totalBytes,
			})
		})
		if err != nil {
			return err
		}

		destinationHash, err := checksumFile(task.Destination.FullPath, func(written int64) {
			bytes := completedBytes.Add(written)
			emitter.Report(Progress{
				Phase:          "hash",
				CurrentPath:    task.Path,
				CurrentAction:  "destination",
				CompletedItems: int(completedItems.Load()),
				TotalItems:     len(tasks),
				CompletedBytes: bytes,
				TotalBytes:     totalBytes,
			})
		})
		if err != nil {
			return err
		}

		result := fileResult{
			Matched:       task.BaseMatched && sourceHash == destinationHash,
			ComparedFile:  true,
			ComparedBytes: task.Source.Size,
		}
		if sourceHash != destinationHash {
			sourceEntry := task.Source.ResultEntry()
			destinationEntry := task.Destination.ResultEntry()
			sourceEntry.HashMD5 = sourceHash
			destinationEntry.HashMD5 = destinationHash
			result.Difference = &Difference{
				Path:        task.Path,
				Kind:        DifferenceContentMismatch,
				Message:     fmt.Sprintf("content hash differs: source=%s destination=%s", sourceHash, destinationHash),
				Source:      sourceEntry,
				Destination: destinationEntry,
			}
		}

		results[index] = result
		items := completedItems.Add(1)
		emitter.Report(Progress{
			Phase:          "hash",
			CurrentPath:    task.Path,
			CurrentAction:  "compare-content",
			CompletedItems: int(items),
			TotalItems:     len(tasks),
			CompletedBytes: completedBytes.Load(),
			TotalBytes:     totalBytes,
		})
		return nil
	}); err != nil {
		return nil, 0, 0, 0, err
	}

	differences := make([]Difference, 0)
	matchedFiles := 0
	comparedFiles := 0
	comparedBytes := int64(0)
	for _, result := range results {
		if !result.ComparedFile {
			continue
		}
		comparedFiles++
		comparedBytes += result.ComparedBytes
		if result.Difference != nil {
			differences = append(differences, *result.Difference)
			continue
		}
		if result.Matched {
			matchedFiles++
		}
	}

	return differences, matchedFiles, comparedFiles, comparedBytes, nil
}

// checksumFile computes the MD5 checksum for the provided file path.
func checksumFile(path string, progress func(int64)) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := md5.New()
	reader := io.Reader(file)
	if progress != nil {
		reader = io.TeeReader(file, progressWriter(progress))
	}
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

type progressWriter func(int64)

// Write reports copied bytes through the supplied progress callback.
func (w progressWriter) Write(p []byte) (int, error) {
	w(int64(len(p)))
	return len(p), nil
}

// normalizeMode converts a file mode into the cross-platform verification representation.
func normalizeMode(mode fs.FileMode) string {
	return fmt.Sprintf("%#o", mode.Perm())
}

// normalizeModifiedTime reduces modified times to UTC second precision.
func normalizeModifiedTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Second)
}

// sortDifferences keeps mismatch output stable for humans and tests.
func sortDifferences(differences []Difference) {
	slices.SortFunc(differences, func(a Difference, b Difference) int {
		if cmp := strings.Compare(a.Path, b.Path); cmp != 0 {
			return cmp
		}
		return strings.Compare(string(a.Kind), string(b.Kind))
	})
}

// reportProgress emits a verifier progress update when configured.
func reportProgress(progress ProgressFunc, update Progress) {
	if progress == nil {
		return
	}

	progress(update)
}

// validateWorkerCount rejects negative worker counts.
func validateWorkerCount(workers int) error {
	if workers < 0 {
		return fmt.Errorf("workers must be zero or greater")
	}

	return nil
}

// normalizeWorkerCount returns a bounded worker count for a task list.
func normalizeWorkerCount(workers int, totalItems int) int {
	if totalItems <= 0 {
		return 1
	}
	if workers <= 0 {
		workers = DefaultWorkerCount()
	}
	if workers > totalItems {
		return totalItems
	}

	return workers
}

// progressEmitter serializes concurrent progress callbacks.
type progressEmitter struct {
	progress ProgressFunc
	mu       sync.Mutex
}

// newProgressEmitter constructs a serialized progress reporter.
func newProgressEmitter(progress ProgressFunc) *progressEmitter {
	return &progressEmitter{progress: progress}
}

// Report emits one serialized progress update.
func (p *progressEmitter) Report(update Progress) {
	if p.progress == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress(update)
}

// runParallel executes work items with bounded concurrency.
func runParallel(ctx context.Context, workers int, total int, fn func(index int) error) error {
	if total <= 0 {
		return nil
	}

	group, ctx := errgroup.WithContext(ctx)
	indexes := make(chan int)
	workers = normalizeWorkerCount(workers, total)

	for range workers {
		group.Go(func() error {
			for index := range indexes {
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := fn(index); err != nil {
					return err
				}
			}
			return nil
		})
	}

	for index := 0; index < total; index++ {
		select {
		case <-ctx.Done():
			close(indexes)
			return ctx.Err()
		case indexes <- index:
		}
	}
	close(indexes)

	return group.Wait()
}
