package fileset

import (
	"context"
	"strings"
	"testing"

	"github.com/cirrusdata/datasim/internal/manifest"
)

// TestParseS3Root verifies datasim's S3-compatible root convention.
func TestParseS3Root(t *testing.T) {
	t.Parallel()

	root, err := parseS3Root("s3://minio.example.com:9000/test-bucket/team/a")
	if err != nil {
		t.Fatalf("parseS3Root returned error: %v", err)
	}

	if root.endpoint != "minio.example.com:9000" {
		t.Fatalf("expected endpoint minio.example.com:9000, got %q", root.endpoint)
	}
	if root.bucket != "test-bucket" {
		t.Fatalf("expected bucket test-bucket, got %q", root.bucket)
	}
	if root.prefix != "team/a" {
		t.Fatalf("expected prefix team/a, got %q", root.prefix)
	}
	if !root.secure {
		t.Fatal("expected s3 URL to default to secure transport")
	}
}

// TestParseS3RootSupportsExplicitHTTP verifies local MinIO can opt out of TLS.
func TestParseS3RootSupportsExplicitHTTP(t *testing.T) {
	t.Parallel()

	root, err := parseS3Root("s3+http://localhost:9000/test-bucket/team/a")
	if err != nil {
		t.Fatalf("parseS3Root returned error: %v", err)
	}

	if root.secure {
		t.Fatal("expected s3+http URL to disable secure transport")
	}
}

// TestParseS3RootRejectsURLCredentials verifies credentials are kept out of root URLs.
func TestParseS3RootRejectsURLCredentials(t *testing.T) {
	t.Parallel()

	_, err := parseS3Root("s3://access:secret@minio.example.com:9000/test-bucket")
	if err == nil {
		t.Fatal("expected credential-bearing S3 URL to be rejected")
	}
	if !strings.Contains(err.Error(), "environment variables") {
		t.Fatalf("expected env-var credential guidance, got %v", err)
	}
}

// TestS3InitRequiresExplicitSize verifies object-store init never defaults from filesystem capacity.
func TestS3InitRequiresExplicitSize(t *testing.T) {
	t.Parallel()

	service := NewService(NewCatalog(), manifest.NewStore(".cirrusdata-datasim"))
	_, err := service.Init(context.Background(), InitOptions{
		Profile:  "corporate",
		Root:     "s3://minio.example.com:9000/test-bucket/team/a",
		Seed:     99,
		Strategy: StrategyBalanced,
	})
	if err == nil {
		t.Fatal("expected S3 init without size to fail")
	}
	if !strings.Contains(err.Error(), "--size is required") {
		t.Fatalf("expected explicit size error, got %v", err)
	}
}
