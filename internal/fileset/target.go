package fileset

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cirrusdata/datasim/internal/manifest"
	"github.com/cirrusdata/datasim/internal/storage"
	"github.com/cirrusdata/datasim/pkg/bytefmt"
)

// datasetTarget materializes a fileset plan against one backing target.
type datasetTarget interface {
	Root() string
	RequiresExplicitSize() bool
	EnsureRoot(context.Context) error
	ResolveGeneration(InitOptions) (int64, manifest.Generation, error)
	LoadManifest(context.Context) (*manifest.Manifest, error)
	SaveManifest(context.Context, *manifest.Manifest) error
	DeleteManifest(context.Context) error
	WriteSpec(context.Context, FileSpec, func(int64)) (manifest.FileRecord, error)
	DeleteFile(context.Context, string) error
	MutateSpec(context.Context, manifest.FileRecord, Mutation, func(int64)) (manifest.FileRecord, error)
	Cleanup(context.Context) error
}

// localTarget stores fileset data in a mounted filesystem path.
type localTarget struct {
	root  string
	store manifestStore
}

// manifestStore is the local manifest store surface used by localTarget.
type manifestStore interface {
	Load(string) (*manifest.Manifest, error)
	Save(string, *manifest.Manifest) error
	Delete(string) error
}

// newDatasetTarget constructs the backing target for a fileset root.
func newDatasetTarget(root string, store manifestStore, manifestFileName string) (datasetTarget, error) {
	if strings.HasPrefix(root, "s3://") || strings.HasPrefix(root, "s3+http://") || strings.HasPrefix(root, "s3+https://") {
		return newS3Target(root, manifestFileName)
	}

	return &localTarget{root: root, store: store}, nil
}

// Root returns the local dataset root.
func (t *localTarget) Root() string {
	return t.root
}

// RequiresExplicitSize reports whether init must receive --size.
func (t *localTarget) RequiresExplicitSize() bool {
	return false
}

// EnsureRoot creates the local dataset root when needed.
func (t *localTarget) EnsureRoot(context.Context) error {
	return os.MkdirAll(t.root, 0o755)
}

// ResolveGeneration determines the local initialization size target.
func (t *localTarget) ResolveGeneration(opts InitOptions) (int64, manifest.Generation, error) {
	if opts.TotalSize != "" {
		return parseExplicitSize(opts.TotalSize)
	}

	stats, err := storage.Stat(t.root)
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

// LoadManifest reads the local manifest.
func (t *localTarget) LoadManifest(context.Context) (*manifest.Manifest, error) {
	return t.store.Load(t.root)
}

// SaveManifest writes the local manifest.
func (t *localTarget) SaveManifest(_ context.Context, doc *manifest.Manifest) error {
	return t.store.Save(t.root, doc)
}

// DeleteManifest removes the local manifest.
func (t *localTarget) DeleteManifest(context.Context) error {
	return t.store.Delete(t.root)
}

// WriteSpec materializes a planned file on disk and returns its manifest record.
func (t *localTarget) WriteSpec(_ context.Context, spec FileSpec, progress func(int64)) (manifest.FileRecord, error) {
	return writeSpec(t.root, spec, progress)
}

// DeleteFile removes one local file.
func (t *localTarget) DeleteFile(_ context.Context, rel string) error {
	if err := os.Remove(filepath.Join(t.root, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// MutateSpec applies a single local file mutation.
func (t *localTarget) MutateSpec(_ context.Context, record manifest.FileRecord, mutation Mutation, progress func(int64)) (manifest.FileRecord, error) {
	return mutateSpec(t.root, record, mutation, progress)
}

// Cleanup removes empty local directories after destroy.
func (t *localTarget) Cleanup(context.Context) error {
	for _, dir := range collectEmptyDirectories(t.root) {
		_ = os.Remove(dir)
	}

	return nil
}

// parseExplicitSize parses a user-specified target size.
func parseExplicitSize(size string) (int64, manifest.Generation, error) {
	targetBytes, err := bytefmt.Parse(size)
	if err != nil {
		return 0, manifest.Generation{}, err
	}

	return targetBytes, manifest.Generation{
		TargetBytes: targetBytes,
	}, nil
}

// patternReader streams deterministic patterned content.
type patternReader struct {
	block     []byte
	offset    int
	remaining int64
}

// newPatternReader constructs a deterministic content reader.
func newPatternReader(size int64, seed int64) io.Reader {
	block := make([]byte, 32*1024)
	for i := range block {
		block[i] = byte((int64(i) + seed) % 255)
	}

	return &patternReader{
		block:     block,
		remaining: size,
	}
}

// Read fills p with deterministic content until the configured size is exhausted.
func (r *patternReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}

	if int64(len(p)) > r.remaining {
		p = p[:int(r.remaining)]
	}

	written := 0
	for written < len(p) {
		n := copy(p[written:], r.block[r.offset:])
		written += n
		r.offset = (r.offset + n) % len(r.block)
	}

	r.remaining -= int64(written)
	return written, nil
}

// progressReader reports bytes read from an underlying stream.
type progressReader struct {
	reader   io.Reader
	progress func(int64)
}

// Read reads from the underlying stream and reports successful read sizes.
func (r progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.progress != nil {
		r.progress(int64(n))
	}

	return n, err
}

// errExplicitSizeRequired returns the common object target size error.
func errExplicitSizeRequired(root string) error {
	return fmt.Errorf("--size is required when --fs uses an object-store target: %s", root)
}
