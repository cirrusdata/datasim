package update

import (
	"context"
	"errors"
	"testing"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// TestNewServiceWithClientRejectsInvalidRepository verifies repository validation at construction time.
func TestNewServiceWithClientRejectsInvalidRepository(t *testing.T) {
	t.Parallel()

	_, err := NewServiceWithClient(
		Config{CurrentVersion: "1.0.0", Repository: "cirrusdata"},
		&fakeClient{},
	)
	if err == nil {
		t.Fatal("expected invalid repository error")
	}
}

// TestUpdateRejectsUnreleasedBuild verifies that self-update is disabled for local development builds.
func TestUpdateRejectsUnreleasedBuild(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	service, err := NewServiceWithClient(
		Config{CurrentVersion: "dev", Repository: "cirrusdata/datasim"},
		client,
	)
	if err != nil {
		t.Fatalf("NewServiceWithClient returned error: %v", err)
	}

	_, err = service.Update(context.Background())
	if !errors.Is(err, ErrUnreleasedBuild) {
		t.Fatalf("expected ErrUnreleasedBuild, got %v", err)
	}
	if client.detectCalls != 0 {
		t.Fatalf("expected detect not to run, got %d calls", client.detectCalls)
	}
}

// TestUpdateReturnsNoopForEqualVersion verifies that equal versions do not trigger a replacement.
func TestUpdateReturnsNoopForEqualVersion(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		release: ReleaseInfo{
			Version:    "1.2.3",
			AssetName:  "datasim_1.2.3_linux_amd64.tar.gz",
			ReleaseURL: "https://github.com/cirrusdata/datasim/releases/tag/v1.2.3",
		},
		found: true,
	}
	service, err := NewServiceWithClient(
		Config{CurrentVersion: "1.2.3", Repository: "cirrusdata/datasim"},
		client,
	)
	if err != nil {
		t.Fatalf("NewServiceWithClient returned error: %v", err)
	}

	result, err := service.Update(context.Background())
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if result.Updated {
		t.Fatal("expected update result to be a no-op")
	}
	if client.updateCalls != 0 {
		t.Fatalf("expected UpdateTo not to run, got %d calls", client.updateCalls)
	}
	if result.CurrentVersion != "1.2.3" {
		t.Fatalf("expected current version to be normalized, got %q", result.CurrentVersion)
	}
	owner, repo, err := client.detectRepository.GetSlug()
	if err != nil {
		t.Fatalf("GetSlug returned error: %v", err)
	}
	if owner != "cirrusdata" || repo != "datasim" {
		t.Fatalf("expected repository cirrusdata/datasim, got %s/%s", owner, repo)
	}
}

// TestUpdateAppliesNewerRelease verifies that newer releases are applied to the running executable.
func TestUpdateAppliesNewerRelease(t *testing.T) {
	t.Parallel()

	client := &fakeClient{
		release: ReleaseInfo{
			Version:    "1.3.0",
			AssetName:  "datasim_1.3.0_linux_amd64.tar.gz",
			ReleaseURL: "https://github.com/cirrusdata/datasim/releases/tag/v1.3.0",
		},
		found:          true,
		executablePath: "/tmp/datasim",
	}
	service, err := NewServiceWithClient(
		Config{CurrentVersion: "1.2.3", Repository: "cirrusdata/datasim"},
		client,
	)
	if err != nil {
		t.Fatalf("NewServiceWithClient returned error: %v", err)
	}

	result, err := service.Update(context.Background())
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if !result.Updated {
		t.Fatal("expected update result to indicate a replacement")
	}
	if client.updateCalls != 1 {
		t.Fatalf("expected UpdateTo to run once, got %d calls", client.updateCalls)
	}
	if client.updatePath != "/tmp/datasim" {
		t.Fatalf("expected executable path to be passed through, got %q", client.updatePath)
	}
}

// TestUpdateReturnsCompatibleReleaseError verifies the unsupported-platform path when no matching asset is found.
func TestUpdateReturnsCompatibleReleaseError(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	service, err := NewServiceWithClient(
		Config{CurrentVersion: "1.2.3", Repository: "cirrusdata/datasim"},
		client,
	)
	if err != nil {
		t.Fatalf("NewServiceWithClient returned error: %v", err)
	}

	_, err = service.Update(context.Background())
	if err == nil {
		t.Fatal("expected no-compatible-release error")
	}
}

// fakeClient is a test double for the update backend.
type fakeClient struct {
	release           ReleaseInfo
	found             bool
	detectErr         error
	executablePath    string
	executablePathErr error
	updateErr         error
	detectCalls       int
	updateCalls       int
	detectRepository  selfupdate.Repository
	updatePath        string
}

// DetectLatest records the requested repository and returns the configured response.
func (f *fakeClient) DetectLatest(_ context.Context, repository selfupdate.Repository) (ReleaseInfo, bool, error) {
	f.detectCalls++
	f.detectRepository = repository
	return f.release, f.found, f.detectErr
}

// ExecutablePath returns the configured executable path.
func (f *fakeClient) ExecutablePath() (string, error) {
	return f.executablePath, f.executablePathErr
}

// UpdateTo records the update request.
func (f *fakeClient) UpdateTo(_ context.Context, release ReleaseInfo, commandPath string) error {
	f.updateCalls++
	f.updatePath = commandPath
	f.release = release
	return f.updateErr
}
