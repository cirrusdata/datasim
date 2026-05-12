package verify

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestVerifyDirMatchesIdenticalTrees verifies identical trees pass verification.
func TestVerifyDirMatchesIdenticalTrees(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	destination := t.TempDir()
	service := NewService(nil)
	modifiedAt := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)

	writeVerificationFile(t, source, "docs/report.txt", "hello world", 0o640, modifiedAt)
	writeVerificationFile(t, destination, "docs/report.txt", "hello world", 0o640, modifiedAt)

	result, err := service.VerifyDir(context.Background(), DirOptions{
		SourceRoot:      source,
		DestinationRoot: destination,
		Metadata:        true,
		Workers:         4,
	})
	if err != nil {
		t.Fatalf("VerifyDir returned error: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected trees to match, got differences: %+v", result.Differences)
	}
	if result.MatchedFiles != 1 {
		t.Fatalf("expected one matched file, got %d", result.MatchedFiles)
	}
	if result.MatchedDirectories != 1 {
		t.Fatalf("expected one matched directory, got %d", result.MatchedDirectories)
	}
}

// TestVerifyDirDetectsDifferences verifies the service reports inventory, content, and metadata mismatches.
func TestVerifyDirDetectsDifferences(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	destination := t.TempDir()
	service := NewService(nil)
	baseTime := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)

	writeVerificationFile(t, source, "missing.txt", "missing", 0o644, baseTime)
	writeVerificationFile(t, source, "content.txt", "source-data-01", 0o644, baseTime)
	writeVerificationFile(t, destination, "content.txt", "source-data-02", 0o644, baseTime)
	writeVerificationFile(t, source, "size.txt", "tiny", 0o644, baseTime)
	writeVerificationFile(t, destination, "size.txt", "much larger file", 0o644, baseTime)
	writeVerificationFile(t, source, "meta.txt", "same bytes", 0o644, baseTime)
	writeVerificationFile(t, destination, "meta.txt", "same bytes", metadataModeForPlatform(), baseTime.Add(2*time.Hour))
	writeVerificationFile(t, destination, "extra.txt", "extra", 0o644, baseTime)

	if err := os.MkdirAll(filepath.Join(source, "type.txt"), 0o755); err != nil {
		t.Fatalf("MkdirAll source type mismatch directory returned error: %v", err)
	}
	writeVerificationFile(t, destination, "type.txt", "file instead of directory", 0o644, baseTime)

	result, err := service.VerifyDir(context.Background(), DirOptions{
		SourceRoot:      source,
		DestinationRoot: destination,
		Metadata:        true,
		Workers:         4,
	})
	if err != nil {
		t.Fatalf("VerifyDir returned error: %v", err)
	}
	if result.Matched {
		t.Fatal("expected verification to report differences")
	}

	kinds := differenceKinds(result.Differences)
	expectDifferenceKind(t, kinds, DifferenceMissingFromDestination)
	expectDifferenceKind(t, kinds, DifferenceExtraInDestination)
	expectDifferenceKind(t, kinds, DifferenceContentMismatch)
	expectDifferenceKind(t, kinds, DifferenceSizeMismatch)
	expectDifferenceKind(t, kinds, DifferenceTypeMismatch)
	expectDifferenceKind(t, kinds, DifferenceModifiedTimeMismatch)
	if runtime.GOOS != "windows" {
		expectDifferenceKind(t, kinds, DifferenceModeMismatch)
	}
}

// TestVerifyDirMetadataDisabledIgnoresModeAndTime verifies metadata-only drift can be ignored.
func TestVerifyDirMetadataDisabledIgnoresModeAndTime(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	destination := t.TempDir()
	service := NewService(nil)
	baseTime := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)

	writeVerificationFile(t, source, "meta.txt", "same bytes", 0o644, baseTime)
	writeVerificationFile(t, destination, "meta.txt", "same bytes", metadataModeForPlatform(), baseTime.Add(2*time.Hour))

	result, err := service.VerifyDir(context.Background(), DirOptions{
		SourceRoot:      source,
		DestinationRoot: destination,
		Metadata:        false,
		Workers:         4,
	})
	if err != nil {
		t.Fatalf("VerifyDir returned error: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected metadata-disabled verification to match, got differences: %+v", result.Differences)
	}
}

// TestVerifyDirExcludesPatterns verifies exclude patterns remove intentional differences from the result.
func TestVerifyDirExcludesPatterns(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	destination := t.TempDir()
	service := NewService(nil)
	baseTime := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)

	writeVerificationFile(t, source, ".cirrusdata-datasim", "metadata", 0o644, baseTime)
	writeVerificationFile(t, source, "payload.txt", "payload", 0o644, baseTime)
	writeVerificationFile(t, destination, "payload.txt", "payload", 0o644, baseTime)

	result, err := service.VerifyDir(context.Background(), DirOptions{
		SourceRoot:      source,
		DestinationRoot: destination,
		Metadata:        true,
		Excludes:        []string{".cirrusdata-datasim"},
		Workers:         4,
	})
	if err != nil {
		t.Fatalf("VerifyDir returned error: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected excluded manifest difference to be ignored, got %+v", result.Differences)
	}
}

// TestVerifyDirReportsUnsupportedSymlink verifies unsupported entry types are surfaced explicitly.
func TestVerifyDirReportsUnsupportedSymlink(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	destination := t.TempDir()
	service := NewService(nil)
	baseTime := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)

	writeVerificationFile(t, source, "target.txt", "payload", 0o644, baseTime)
	writeVerificationFile(t, destination, "target.txt", "payload", 0o644, baseTime)

	if err := os.Symlink("target.txt", filepath.Join(source, "link.txt")); err != nil {
		t.Skipf("symlink creation is not available: %v", err)
	}
	if err := os.Symlink("target.txt", filepath.Join(destination, "link.txt")); err != nil {
		t.Skipf("symlink creation is not available: %v", err)
	}

	result, err := service.VerifyDir(context.Background(), DirOptions{
		SourceRoot:      source,
		DestinationRoot: destination,
		Metadata:        true,
		Workers:         4,
	})
	if err != nil {
		t.Fatalf("VerifyDir returned error: %v", err)
	}
	if result.Matched {
		t.Fatal("expected unsupported symlink entries to fail verification")
	}

	kinds := differenceKinds(result.Differences)
	expectDifferenceKind(t, kinds, DifferenceUnsupportedType)
}

// writeVerificationFile creates one test file with predictable content and metadata.
func writeVerificationFile(t *testing.T, root string, relativePath string, content string, mode os.FileMode, modifiedAt time.Time) {
	t.Helper()

	fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), mode); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.Chmod(fullPath, mode); err != nil && runtime.GOOS != "windows" {
		t.Fatalf("Chmod returned error: %v", err)
	}
	if err := os.Chtimes(fullPath, modifiedAt, modifiedAt); err != nil {
		t.Fatalf("Chtimes returned error: %v", err)
	}
}

// metadataModeForPlatform returns a metadata-only drift mode for the active platform.
func metadataModeForPlatform() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0o644
	}

	return 0o600
}

// differenceKinds collects the difference kinds present in the result.
func differenceKinds(differences []Difference) map[DifferenceKind]bool {
	kinds := make(map[DifferenceKind]bool, len(differences))
	for _, difference := range differences {
		kinds[difference.Kind] = true
	}

	return kinds
}

// expectDifferenceKind fails the test when the expected difference kind is absent.
func expectDifferenceKind(t *testing.T, kinds map[DifferenceKind]bool, kind DifferenceKind) {
	t.Helper()

	if !kinds[kind] {
		t.Fatalf("expected difference kind %q, got %v", kind, kinds)
	}
}
