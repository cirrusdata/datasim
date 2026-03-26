package update

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/Masterminds/semver/v3"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

const checksumFileName = "checksums.txt"

// ErrUnreleasedBuild indicates that the running binary does not have a SemVer release version.
var ErrUnreleasedBuild = errors.New("self-update is only available for released builds")

// Config describes the metadata required to update the running binary.
type Config struct {
	CurrentVersion string
	Repository     string
}

// Result describes the outcome of an update check or applied update.
type Result struct {
	CurrentVersion string
	LatestVersion  string
	AssetName      string
	ReleaseURL     string
	Updated        bool
}

// Service checks for and applies released binary updates.
type Service struct {
	currentVersion string
	repository     selfupdate.Repository
	client         client
}

// ReleaseInfo carries the subset of release metadata needed by the service.
type ReleaseInfo struct {
	Version    string
	AssetName  string
	ReleaseURL string
	raw        *selfupdate.Release
}

// client abstracts the self-update backend so the service can be unit tested.
type client interface {
	DetectLatest(ctx context.Context, repository selfupdate.Repository) (ReleaseInfo, bool, error)
	ExecutablePath() (string, error)
	UpdateTo(ctx context.Context, release ReleaseInfo, commandPath string) error
}

// NewService constructs a release updater using the default GitHub-backed client.
func NewService(cfg Config) (*Service, error) {
	updateClient, err := newDefaultClient()
	if err != nil {
		return nil, err
	}

	return NewServiceWithClient(cfg, updateClient)
}

// NewServiceWithClient constructs a release updater with an explicit backend.
func NewServiceWithClient(cfg Config, updateClient client) (*Service, error) {
	repository := selfupdate.ParseSlug(cfg.Repository)
	if _, _, err := repository.GetSlug(); err != nil {
		return nil, fmt.Errorf("invalid release repository %q: %w", cfg.Repository, err)
	}

	return &Service{
		currentVersion: cfg.CurrentVersion,
		repository:     repository,
		client:         updateClient,
	}, nil
}

// Update checks for the latest stable release and updates the running binary when needed.
func (s *Service) Update(ctx context.Context) (Result, error) {
	currentVersion, err := parseCurrentVersion(s.currentVersion)
	if err != nil {
		return Result{}, err
	}

	release, found, err := s.client.DetectLatest(ctx, s.repository)
	if err != nil {
		return Result{}, fmt.Errorf("detect latest release: %w", err)
	}
	if !found {
		return Result{}, fmt.Errorf(
			"no compatible release asset found for %s/%s",
			runtime.GOOS,
			runtime.GOARCH,
		)
	}

	latestVersion, err := semver.NewVersion(release.Version)
	if err != nil {
		return Result{}, fmt.Errorf("parse latest release version %q: %w", release.Version, err)
	}

	result := Result{
		CurrentVersion: currentVersion.String(),
		LatestVersion:  latestVersion.String(),
		AssetName:      release.AssetName,
		ReleaseURL:     release.ReleaseURL,
	}

	if !currentVersion.LessThan(latestVersion) {
		return result, nil
	}

	commandPath, err := s.client.ExecutablePath()
	if err != nil {
		return Result{}, fmt.Errorf("locate executable: %w", err)
	}
	if err := s.client.UpdateTo(ctx, release, commandPath); err != nil {
		return Result{}, fmt.Errorf("apply update: %w", err)
	}

	result.Updated = true
	return result, nil
}

// parseCurrentVersion validates that the running binary was built from a released SemVer tag.
func parseCurrentVersion(version string) (*semver.Version, error) {
	parsed, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("%w: current version %q is not SemVer", ErrUnreleasedBuild, version)
	}

	return parsed, nil
}

// selfUpdateClient adapts go-selfupdate to the local client interface.
type selfUpdateClient struct {
	updater *selfupdate.Updater
}

// newDefaultClient constructs the default GitHub-backed self-update client.
func newDefaultClient() (client, error) {
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: checksumFileName},
	})
	if err != nil {
		return nil, fmt.Errorf("create self-update client: %w", err)
	}

	return selfUpdateClient{updater: updater}, nil
}

// DetectLatest resolves the newest compatible stable release for the configured platform.
func (c selfUpdateClient) DetectLatest(ctx context.Context, repository selfupdate.Repository) (ReleaseInfo, bool, error) {
	release, found, err := c.updater.DetectLatest(ctx, repository)
	if err != nil {
		return ReleaseInfo{}, false, err
	}
	if !found || release == nil {
		return ReleaseInfo{}, false, nil
	}

	return ReleaseInfo{
		Version:    release.Version(),
		AssetName:  release.AssetName,
		ReleaseURL: release.URL,
		raw:        release,
	}, true, nil
}

// ExecutablePath returns the fully resolved path to the running executable.
func (c selfUpdateClient) ExecutablePath() (string, error) {
	return selfupdate.ExecutablePath()
}

// UpdateTo downloads and replaces the running binary with the provided release asset.
func (c selfUpdateClient) UpdateTo(ctx context.Context, release ReleaseInfo, commandPath string) error {
	if release.raw == nil {
		return errors.New("release payload is missing")
	}

	return c.updater.UpdateTo(ctx, release.raw, commandPath)
}
