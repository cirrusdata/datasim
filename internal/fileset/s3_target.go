package fileset

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/cirrusdata/datasim/internal/manifest"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// s3Target stores fileset data as S3-compatible objects.
type s3Target struct {
	root             string
	endpoint         string
	bucket           string
	prefix           string
	manifestFileName string
	client           *minio.Client
}

// newS3Target constructs an S3-compatible object-store target.
func newS3Target(root string, manifestFileName string) (*s3Target, error) {
	parsed, err := parseS3Root(root)
	if err != nil {
		return nil, err
	}

	client, err := minio.New(parsed.endpoint, &minio.Options{
		Creds:  credentials.NewEnvAWS(),
		Secure: parsed.secure,
	})
	if err != nil {
		return nil, err
	}

	return &s3Target{
		root:             root,
		endpoint:         parsed.endpoint,
		bucket:           parsed.bucket,
		prefix:           parsed.prefix,
		manifestFileName: manifestFileName,
		client:           client,
	}, nil
}

// Root returns the object-store dataset root.
func (t *s3Target) Root() string {
	return t.root
}

// RequiresExplicitSize reports whether init must receive --size.
func (t *s3Target) RequiresExplicitSize() bool {
	return true
}

// EnsureRoot validates that the bucket is reachable.
func (t *s3Target) EnsureRoot(ctx context.Context) error {
	exists, err := t.client.BucketExists(ctx, t.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("s3 bucket %q does not exist", t.bucket)
	}

	return nil
}

// ResolveGeneration determines the object-store initialization size target.
func (t *s3Target) ResolveGeneration(opts InitOptions) (int64, manifest.Generation, error) {
	if opts.TotalSize == "" {
		return 0, manifest.Generation{}, errExplicitSizeRequired(t.root)
	}

	return parseExplicitSize(opts.TotalSize)
}

// LoadManifest reads the object-store manifest.
func (t *s3Target) LoadManifest(ctx context.Context) (*manifest.Manifest, error) {
	object, err := t.client.GetObject(ctx, t.bucket, t.manifestKey(), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, err
	}

	var doc manifest.Manifest
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

// SaveManifest writes the object-store manifest.
func (t *s3Target) SaveManifest(ctx context.Context, doc *manifest.Manifest) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	_, err = t.client.PutObject(
		ctx,
		t.bucket,
		t.manifestKey(),
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/json"},
	)
	return err
}

// DeleteManifest removes the object-store manifest.
func (t *s3Target) DeleteManifest(ctx context.Context) error {
	return t.client.RemoveObject(ctx, t.bucket, t.manifestKey(), minio.RemoveObjectOptions{})
}

// WriteSpec materializes a planned object and returns its manifest record.
func (t *s3Target) WriteSpec(ctx context.Context, spec FileSpec, progress func(int64)) (manifest.FileRecord, error) {
	checksum, err := t.putPatternObject(ctx, spec.RelativePath, spec.Size, spec.Seed, spec.Mode.String(), spec.ModifiedAt.Format(timeFormatRFC3339Nano), spec.Labels, progress)
	if err != nil {
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

// DeleteFile removes one object-store file.
func (t *s3Target) DeleteFile(ctx context.Context, rel string) error {
	return t.client.RemoveObject(ctx, t.bucket, t.objectKey(rel), minio.RemoveObjectOptions{})
}

// MutateSpec applies a single object-store mutation.
func (t *s3Target) MutateSpec(ctx context.Context, record manifest.FileRecord, mutation Mutation, progress func(int64)) (manifest.FileRecord, error) {
	reader, size, err := t.mutationReader(ctx, record, mutation)
	if err != nil {
		return manifest.FileRecord{}, err
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}

	checksum, err := t.putObject(ctx, mutation.RelativePath, size, reader, record.Mode, mutation.ModifiedAt.Format(timeFormatRFC3339Nano), record.Labels, progress)
	if err != nil {
		return manifest.FileRecord{}, err
	}

	record.Size = mutation.NewSize
	record.ChecksumMD5 = checksum
	record.ModifiedAt = mutation.ModifiedAt
	return record, nil
}

// Cleanup is a no-op for object-store prefixes.
func (t *s3Target) Cleanup(context.Context) error {
	return nil
}

// putPatternObject writes deterministic patterned object content.
func (t *s3Target) putPatternObject(ctx context.Context, rel string, size int64, seed int64, mode string, modifiedAt string, labels map[string]string, progress func(int64)) (string, error) {
	return t.putObject(ctx, rel, size, newPatternReader(size, seed), mode, modifiedAt, labels, progress)
}

// putObject writes one object and returns the uploaded content checksum.
func (t *s3Target) putObject(ctx context.Context, rel string, size int64, reader io.Reader, mode string, modifiedAt string, labels map[string]string, progress func(int64)) (string, error) {
	hasher := md5.New()
	var stream io.Reader = io.TeeReader(reader, hasher)
	if progress != nil {
		stream = progressReader{reader: stream, progress: progress}
	}

	_, err := t.client.PutObject(ctx, t.bucket, t.objectKey(rel), stream, size, minio.PutObjectOptions{
		ContentType:  "application/octet-stream",
		UserMetadata: objectMetadata(mode, modifiedAt, labels),
	})
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// mutationReader returns the source stream for one object-store mutation.
func (t *s3Target) mutationReader(ctx context.Context, record manifest.FileRecord, mutation Mutation) (io.Reader, int64, error) {
	switch mutation.Action {
	case MutationRewrite:
		return newPatternReader(mutation.NewSize, mutation.Seed), mutation.NewSize, nil
	case MutationAppend:
		object, err := t.client.GetObject(ctx, t.bucket, t.objectKey(record.Path), minio.GetObjectOptions{})
		if err != nil {
			return nil, 0, err
		}
		return struct {
			io.Reader
			io.Closer
		}{
			Reader: io.MultiReader(object, newPatternReader(mutation.NewSize-record.Size, mutation.Seed)),
			Closer: object,
		}, mutation.NewSize, nil
	case MutationTruncate:
		object, err := t.client.GetObject(ctx, t.bucket, t.objectKey(record.Path), minio.GetObjectOptions{})
		if err != nil {
			return nil, 0, err
		}
		return struct {
			io.Reader
			io.Closer
		}{
			Reader: io.LimitReader(object, mutation.NewSize),
			Closer: object,
		}, mutation.NewSize, nil
	default:
		return nil, 0, fmt.Errorf("unknown mutation action %q", mutation.Action)
	}
}

// manifestKey returns the object key for the manifest.
func (t *s3Target) manifestKey() string {
	return t.objectKey(t.manifestFileName)
}

// objectKey returns the object key for a fileset relative path.
func (t *s3Target) objectKey(rel string) string {
	rel = strings.TrimLeft(path.Clean(strings.ReplaceAll(rel, "\\", "/")), "/")
	if t.prefix == "" {
		return rel
	}

	return path.Join(t.prefix, rel)
}

// s3Root describes a parsed s3://host/bucket/prefix root.
type s3Root struct {
	endpoint string
	bucket   string
	prefix   string
	secure   bool
}

// parseS3Root parses datasim's S3-compatible fileset root convention.
func parseS3Root(raw string) (s3Root, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return s3Root{}, err
	}
	secure := true
	switch parsed.Scheme {
	case "s3", "s3+https":
	case "s3+http":
		secure = false
	default:
		return s3Root{}, fmt.Errorf("expected s3 URL, got %q", raw)
	}
	if parsed.User != nil {
		return s3Root{}, fmt.Errorf("credentials in S3 URLs are not supported; use AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables")
	}
	if parsed.Host == "" {
		return s3Root{}, fmt.Errorf("s3 URL must include an endpoint host")
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return s3Root{}, fmt.Errorf("s3 URL must include a bucket path")
	}

	prefix := ""
	if len(parts) > 1 {
		prefix = path.Join(parts[1:]...)
	}

	return s3Root{
		endpoint: parsed.Host,
		bucket:   parts[0],
		prefix:   prefix,
		secure:   secure,
	}, nil
}

// objectMetadata builds object metadata for filesystem-ish manifest fields.
func objectMetadata(mode string, modifiedAt string, labels map[string]string) map[string]string {
	metadata := map[string]string{
		"datasim-mode":        mode,
		"datasim-modified-at": modifiedAt,
	}
	for key, value := range labels {
		metadata["datasim-label-"+key] = value
	}

	return metadata
}

const timeFormatRFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"
