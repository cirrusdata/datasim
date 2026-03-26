package fileset

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cirrusdata/datasim/internal/manifest"
	"github.com/cirrusdata/datasim/internal/storage"
	"github.com/cirrusdata/datasim/pkg/bytefmt"
)

// Service orchestrates fileset initialization, rotation, status, and destroy operations.
type Service struct {
	catalog *Catalog
	store   *manifest.Store
}

// InitOptions describes a fileset initialization request.
type InitOptions struct {
	Profile   string
	Root      string
	TotalSize string
	Seed      int64
	Strategy  string
	Progress  ProgressFunc
}

// RotateOptions describes a fileset rotation request.
type RotateOptions struct {
	Root      string
	CreatePct float64
	DeletePct float64
	ModifyPct float64
	Seed      int64
	Strategy  string
	Progress  ProgressFunc
}

// DestroyOptions describes a fileset destroy request.
type DestroyOptions struct {
	Root     string
	Progress ProgressFunc
}

// Progress describes progress for long-running fileset operations.
type Progress struct {
	Operation      string
	Phase          string
	CurrentPath    string
	CurrentAction  string
	CompletedItems int
	TotalItems     int
	CompletedBytes int64
	TotalBytes     int64
}

// ProgressFunc receives progress updates for fileset operations.
type ProgressFunc func(Progress)

// NewService constructs a fileset service from a profile catalog and manifest store.
func NewService(catalog *Catalog, store *manifest.Store) *Service {
	return &Service{catalog: catalog, store: store}
}

// Catalog returns the fileset profile catalog.
func (s *Service) Catalog() *Catalog {
	return s.catalog
}

// Init initializes a fileset and persists its manifest.
func (s *Service) Init(ctx context.Context, opts InitOptions) (*manifest.Manifest, error) {
	if opts.Profile == "" {
		opts.Profile = s.catalog.DefaultProfileName()
	}
	if opts.Strategy == "" {
		opts.Strategy = StrategyBalanced
	}
	if err := ValidateStrategy(opts.Strategy); err != nil {
		return nil, err
	}

	profile, err := s.catalog.Get(opts.Profile)
	if err != nil {
		return nil, err
	}

	seed := opts.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	if err := os.MkdirAll(opts.Root, 0o755); err != nil {
		return nil, err
	}

	targetBytes, generation, err := s.resolveGeneration(opts)
	if err != nil {
		return nil, err
	}

	plan, err := profile.PlanInit(ctx, InitRequest{
		Root:           opts.Root,
		TargetBytes:    targetBytes,
		PreferredFiles: 0,
		Seed:           seed,
		Strategy:       opts.Strategy,
	})
	if err != nil {
		return nil, err
	}

	totalBytes := int64(0)
	for _, spec := range plan.Files {
		totalBytes += spec.Size
	}
	reportProgress(opts.Progress, Progress{
		Operation:     "init",
		Phase:         "write",
		TotalItems:    len(plan.Files),
		TotalBytes:    totalBytes,
		CurrentAction: "create",
	})

	now := time.Now().UTC()
	doc := &manifest.Manifest{
		Version:    1,
		Workload:   "fileset",
		Profile:    profile.Name,
		Strategy:   opts.Strategy,
		Seed:       seed,
		CreatedAt:  now,
		UpdatedAt:  now,
		Generation: generation,
		Filesystem: manifest.Filesystem{
			Root: opts.Root,
		},
	}

	completedFiles := 0
	completedBytes := int64(0)
	for _, spec := range plan.Files {
		record, err := writeSpec(opts.Root, spec, func(written int64) {
			completedBytes += written
			reportProgress(opts.Progress, Progress{
				Operation:      "init",
				Phase:          "write",
				CurrentPath:    spec.RelativePath,
				CurrentAction:  "create",
				CompletedItems: completedFiles,
				TotalItems:     len(plan.Files),
				CompletedBytes: completedBytes,
				TotalBytes:     totalBytes,
			})
		})
		if err != nil {
			return nil, err
		}
		doc.Files = append(doc.Files, record)
		completedFiles++
		reportProgress(opts.Progress, Progress{
			Operation:      "init",
			Phase:          "write",
			CurrentPath:    spec.RelativePath,
			CurrentAction:  "create",
			CompletedItems: completedFiles,
			TotalItems:     len(plan.Files),
			CompletedBytes: completedBytes,
			TotalBytes:     totalBytes,
		})
	}

	sortFiles(doc.Files)
	manifest.RefreshStatus(doc, "init", now, len(doc.Files), 0, 0)

	reportProgress(opts.Progress, Progress{
		Operation:      "init",
		Phase:          "save",
		CurrentAction:  "save-manifest",
		CompletedItems: 0,
		TotalItems:     1,
	})
	if err := s.store.Save(opts.Root, doc); err != nil {
		return nil, err
	}
	reportProgress(opts.Progress, Progress{
		Operation:      "init",
		Phase:          "save",
		CurrentAction:  "save-manifest",
		CompletedItems: 1,
		TotalItems:     1,
	})

	return doc, nil
}

// Rotate mutates an existing fileset and appends a rotation history record.
func (s *Service) Rotate(ctx context.Context, opts RotateOptions) (*manifest.Manifest, error) {
	if opts.Strategy == "" {
		opts.Strategy = StrategyBalanced
	}
	if err := ValidateStrategy(opts.Strategy); err != nil {
		return nil, err
	}

	doc, err := s.store.Load(opts.Root)
	if err != nil {
		return nil, err
	}

	if doc.Workload != "" && doc.Workload != "fileset" {
		return nil, fmt.Errorf("manifest at %s is for workload %q, not fileset", opts.Root, doc.Workload)
	}

	profileName := doc.Profile
	if profileName == "" {
		profileName = s.catalog.DefaultProfileName()
	}

	profile, err := s.catalog.Get(profileName)
	if err != nil {
		return nil, err
	}

	seed := opts.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	plan, err := profile.PlanRotate(ctx, RotateRequest{
		Manifest:  doc,
		CreatePct: opts.CreatePct,
		DeletePct: opts.DeletePct,
		ModifyPct: opts.ModifyPct,
		Seed:      seed,
		Strategy:  opts.Strategy,
	})
	if err != nil {
		return nil, err
	}

	records := make(map[string]manifest.FileRecord, len(doc.Files))
	for _, file := range doc.Files {
		records[file.Path] = file
	}

	reportProgress(opts.Progress, Progress{
		Operation:     "rotate",
		Phase:         "delete",
		CurrentAction: "delete",
		TotalItems:    len(plan.Deletes),
	})
	deleted := 0
	for _, rel := range plan.Deletes {
		if err := os.Remove(filepath.Join(opts.Root, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		delete(records, rel)
		deleted++
		reportProgress(opts.Progress, Progress{
			Operation:      "rotate",
			Phase:          "delete",
			CurrentPath:    rel,
			CurrentAction:  "delete",
			CompletedItems: deleted,
			TotalItems:     len(plan.Deletes),
		})
	}

	totalMutationBytes := int64(0)
	for _, mutation := range plan.Mutations {
		record, ok := records[mutation.RelativePath]
		if !ok {
			continue
		}
		totalMutationBytes += mutationWorkBytes(record, mutation)
	}
	reportProgress(opts.Progress, Progress{
		Operation:     "rotate",
		Phase:         "mutate",
		CurrentAction: "mutate",
		TotalItems:    len(plan.Mutations),
		TotalBytes:    totalMutationBytes,
	})
	mutated := 0
	mutatedBytes := int64(0)
	for _, mutation := range plan.Mutations {
		record, ok := records[mutation.RelativePath]
		if !ok {
			continue
		}

		updated, err := mutateSpec(opts.Root, record, mutation, func(written int64) {
			mutatedBytes += written
			reportProgress(opts.Progress, Progress{
				Operation:      "rotate",
				Phase:          "mutate",
				CurrentPath:    mutation.RelativePath,
				CurrentAction:  string(mutation.Action),
				CompletedItems: mutated,
				TotalItems:     len(plan.Mutations),
				CompletedBytes: mutatedBytes,
				TotalBytes:     totalMutationBytes,
			})
		})
		if err != nil {
			return nil, err
		}
		records[mutation.RelativePath] = updated
		mutated++
		reportProgress(opts.Progress, Progress{
			Operation:      "rotate",
			Phase:          "mutate",
			CurrentPath:    mutation.RelativePath,
			CurrentAction:  string(mutation.Action),
			CompletedItems: mutated,
			TotalItems:     len(plan.Mutations),
			CompletedBytes: mutatedBytes,
			TotalBytes:     totalMutationBytes,
		})
	}

	totalCreateBytes := int64(0)
	for _, spec := range plan.Creates {
		totalCreateBytes += spec.Size
	}
	reportProgress(opts.Progress, Progress{
		Operation:     "rotate",
		Phase:         "create",
		CurrentAction: "create",
		TotalItems:    len(plan.Creates),
		TotalBytes:    totalCreateBytes,
	})
	created := 0
	createdBytes := int64(0)
	for _, spec := range plan.Creates {
		record, err := writeSpec(opts.Root, spec, func(written int64) {
			createdBytes += written
			reportProgress(opts.Progress, Progress{
				Operation:      "rotate",
				Phase:          "create",
				CurrentPath:    spec.RelativePath,
				CurrentAction:  "create",
				CompletedItems: created,
				TotalItems:     len(plan.Creates),
				CompletedBytes: createdBytes,
				TotalBytes:     totalCreateBytes,
			})
		})
		if err != nil {
			return nil, err
		}
		records[record.Path] = record
		created++
		reportProgress(opts.Progress, Progress{
			Operation:      "rotate",
			Phase:          "create",
			CurrentPath:    spec.RelativePath,
			CurrentAction:  "create",
			CompletedItems: created,
			TotalItems:     len(plan.Creates),
			CompletedBytes: createdBytes,
			TotalBytes:     totalCreateBytes,
		})
	}

	doc.Files = doc.Files[:0]
	for _, record := range records {
		doc.Files = append(doc.Files, record)
	}

	sortFiles(doc.Files)
	doc.Workload = "fileset"
	doc.Profile = profile.Name
	doc.UpdatedAt = time.Now().UTC()
	doc.History = append(doc.History, manifest.RotationHistory{
		At:        doc.UpdatedAt,
		Seed:      seed,
		CreatePct: opts.CreatePct,
		DeletePct: opts.DeletePct,
		ModifyPct: opts.ModifyPct,
		Created:   len(plan.Creates),
		Deleted:   len(plan.Deletes),
		Modified:  len(plan.Mutations),
		Strategy:  opts.Strategy,
	})
	manifest.RefreshStatus(doc, "rotate", doc.UpdatedAt, len(plan.Creates), len(plan.Deletes), len(plan.Mutations))

	reportProgress(opts.Progress, Progress{
		Operation:      "rotate",
		Phase:          "save",
		CurrentAction:  "save-manifest",
		CompletedItems: 0,
		TotalItems:     1,
	})
	if err := s.store.Save(opts.Root, doc); err != nil {
		return nil, err
	}
	reportProgress(opts.Progress, Progress{
		Operation:      "rotate",
		Phase:          "save",
		CurrentAction:  "save-manifest",
		CompletedItems: 1,
		TotalItems:     1,
	})

	return doc, nil
}

// Destroy removes the files tracked by a fileset manifest and deletes the manifest itself.
func (s *Service) Destroy(opts DestroyOptions) error {
	doc, err := s.store.Load(opts.Root)
	if err != nil {
		return err
	}

	reportProgress(opts.Progress, Progress{
		Operation:     "destroy",
		Phase:         "delete",
		CurrentAction: "delete",
		TotalItems:    len(doc.Files),
	})
	deleted := 0
	for _, file := range doc.Files {
		if err := os.Remove(filepath.Join(opts.Root, filepath.FromSlash(file.Path))); err != nil && !os.IsNotExist(err) {
			return err
		}
		deleted++
		reportProgress(opts.Progress, Progress{
			Operation:      "destroy",
			Phase:          "delete",
			CurrentPath:    file.Path,
			CurrentAction:  "delete",
			CompletedItems: deleted,
			TotalItems:     len(doc.Files),
		})
	}

	dirs := collectEmptyDirectories(opts.Root)
	reportProgress(opts.Progress, Progress{
		Operation:     "destroy",
		Phase:         "cleanup",
		CurrentAction: "remove-empty-directories",
		TotalItems:    len(dirs),
	})
	for idx, dir := range dirs {
		_ = os.Remove(dir)
		reportProgress(opts.Progress, Progress{
			Operation:      "destroy",
			Phase:          "cleanup",
			CurrentPath:    dir,
			CurrentAction:  "remove-empty-directories",
			CompletedItems: idx + 1,
			TotalItems:     len(dirs),
		})
	}

	reportProgress(opts.Progress, Progress{
		Operation:      "destroy",
		Phase:          "save",
		CurrentAction:  "delete-manifest",
		CompletedItems: 0,
		TotalItems:     1,
	})
	if err := s.store.Delete(opts.Root); err != nil {
		return err
	}
	reportProgress(opts.Progress, Progress{
		Operation:      "destroy",
		Phase:          "save",
		CurrentAction:  "delete-manifest",
		CompletedItems: 1,
		TotalItems:     1,
	})

	return nil
}

// Status loads the manifest-backed state for a fileset.
func (s *Service) Status(root string) (*manifest.Manifest, error) {
	doc, err := s.store.Load(root)
	if err != nil {
		return nil, err
	}

	if doc.Status.State == "" {
		manifest.RefreshStatus(doc, "status", doc.UpdatedAt, 0, 0, 0)
	}

	return doc, nil
}

// resolveGeneration determines the initialization size target.
func (s *Service) resolveGeneration(opts InitOptions) (int64, manifest.Generation, error) {
	if opts.TotalSize != "" {
		targetBytes, err := bytefmt.Parse(opts.TotalSize)
		if err != nil {
			return 0, manifest.Generation{}, err
		}

		return targetBytes, manifest.Generation{
			TargetBytes: targetBytes,
		}, nil
	}

	stats, err := storage.Stat(opts.Root)
	if err != nil {
		return 0, manifest.Generation{}, err
	}

	targetBytes := int64(stats.CapacityBytes * 80 / 100)
	return targetBytes, manifest.Generation{
		TargetBytes:           targetBytes,
		DefaultedFromCapacity: true,
		CapacityBytes:         stats.CapacityBytes,
		TargetUtilizationPct:  80,
	}, nil
}

// sortFiles keeps manifest file records in a stable order.
func sortFiles(files []manifest.FileRecord) {
	slices.SortFunc(files, func(a manifest.FileRecord, b manifest.FileRecord) int {
		return strings.Compare(a.Path, b.Path)
	})
}

// writeSpec materializes a planned file on disk and returns its manifest record.
func writeSpec(root string, spec FileSpec, progress func(int64)) (manifest.FileRecord, error) {
	fullPath := filepath.Join(root, filepath.FromSlash(spec.RelativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return manifest.FileRecord{}, err
	}

	checksum, err := writePatternFile(fullPath, spec.Size, spec.Seed, false, progress)
	if err != nil {
		return manifest.FileRecord{}, err
	}

	if err := os.Chmod(fullPath, spec.Mode); err != nil {
		return manifest.FileRecord{}, err
	}
	if err := os.Chtimes(fullPath, spec.ModifiedAt, spec.ModifiedAt); err != nil {
		return manifest.FileRecord{}, err
	}

	return manifest.FileRecord{
		Path:        spec.RelativePath,
		Size:        spec.Size,
		ChecksumMD5: checksum,
		Mode:        spec.Mode.String(),
		ModifiedAt:  spec.ModifiedAt,
		Labels:      spec.Labels,
	}, nil
}

// mutateSpec applies a single file mutation and returns the updated manifest record.
func mutateSpec(root string, record manifest.FileRecord, mutation Mutation, progress func(int64)) (manifest.FileRecord, error) {
	fullPath := filepath.Join(root, filepath.FromSlash(record.Path))
	originalMode, restoreMode, err := makeWritableForMutation(fullPath)
	if err != nil {
		return manifest.FileRecord{}, err
	}
	if restoreMode {
		defer func() {
			_ = os.Chmod(fullPath, originalMode)
		}()
	}

	switch mutation.Action {
	case MutationRewrite:
		checksum, err := writePatternFile(fullPath, mutation.NewSize, mutation.Seed, false, progress)
		if err != nil {
			return manifest.FileRecord{}, err
		}
		record.ChecksumMD5 = checksum
	case MutationAppend:
		if _, err := writePatternFile(fullPath, mutation.NewSize-record.Size, mutation.Seed, true, progress); err != nil {
			return manifest.FileRecord{}, err
		}
		checksum, err := checksumFile(fullPath, progress)
		if err != nil {
			return manifest.FileRecord{}, err
		}
		record.ChecksumMD5 = checksum
	case MutationTruncate:
		if err := os.Truncate(fullPath, mutation.NewSize); err != nil {
			return manifest.FileRecord{}, err
		}
		checksum, err := checksumFile(fullPath, progress)
		if err != nil {
			return manifest.FileRecord{}, err
		}
		record.ChecksumMD5 = checksum
	default:
		return manifest.FileRecord{}, fmt.Errorf("unknown mutation action %q", mutation.Action)
	}

	record.Size = mutation.NewSize
	record.ModifiedAt = mutation.ModifiedAt
	if err := os.Chtimes(fullPath, mutation.ModifiedAt, mutation.ModifiedAt); err != nil {
		return manifest.FileRecord{}, err
	}

	return record, nil
}

// makeWritableForMutation temporarily adds owner-write permission when needed.
func makeWritableForMutation(path string) (os.FileMode, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false, err
	}

	mode := info.Mode().Perm()
	if mode&0o200 != 0 {
		return mode, false, nil
	}

	if err := os.Chmod(path, mode|0o200); err != nil {
		return 0, false, err
	}

	return mode, true, nil
}

// writePatternFile writes deterministic patterned content and returns its MD5 checksum.
func writePatternFile(path string, size int64, seed int64, appendMode bool, progress func(int64)) (string, error) {
	flag := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	file, err := os.OpenFile(path, flag, 0o644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := md5.New()
	writer := io.MultiWriter(file, hasher)
	if err := writePattern(writer, size, seed, progress); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// writePattern streams deterministic content into a writer.
func writePattern(w io.Writer, size int64, seed int64, progress func(int64)) error {
	if size <= 0 {
		return nil
	}

	block := make([]byte, 32*1024)
	for i := range block {
		block[i] = byte((int64(i) + seed) % math.MaxUint8)
	}

	remaining := size
	for remaining > 0 {
		chunk := min(remaining, int64(len(block)))
		if _, err := w.Write(block[:chunk]); err != nil {
			return err
		}
		if progress != nil {
			progress(chunk)
		}
		remaining -= chunk
	}

	return nil
}

// checksumFile computes the MD5 checksum for an existing file.
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

// Write reports written bytes through a progress callback.
func (w progressWriter) Write(p []byte) (int, error) {
	w(int64(len(p)))
	return len(p), nil
}

// mutationWorkBytes estimates the I/O work needed for a mutation.
func mutationWorkBytes(record manifest.FileRecord, mutation Mutation) int64 {
	switch mutation.Action {
	case MutationRewrite:
		return mutation.NewSize
	case MutationAppend:
		return max(0, mutation.NewSize-record.Size) + mutation.NewSize
	case MutationTruncate:
		return mutation.NewSize
	default:
		return 0
	}
}

// collectEmptyDirectories returns existing directories in removal order.
func collectEmptyDirectories(root string) []string {
	ordered := make([]string, 0)
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == root || !entry.IsDir() {
			return nil
		}

		ordered = append(ordered, path)
		return nil
	})

	slices.SortFunc(ordered, func(a string, b string) int {
		if len(a) > len(b) {
			return -1
		}
		if len(a) < len(b) {
			return 1
		}
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})

	return ordered
}

// reportProgress emits a progress update when a callback is configured.
func reportProgress(progress ProgressFunc, update Progress) {
	if progress == nil {
		return
	}

	progress(update)
}
