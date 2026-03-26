package app

import (
	"github.com/cirrusdata/datasim/internal/config"
	"github.com/cirrusdata/datasim/internal/fileset"
	"github.com/cirrusdata/datasim/internal/filesystem"
	"github.com/cirrusdata/datasim/internal/manifest"
	"github.com/cirrusdata/datasim/internal/update"
)

// BuildInfo describes the build-time metadata embedded in the binary.
type BuildInfo struct {
	Version    string
	Commit     string
	Date       string
	Repository string
}

// Bootstrap holds the application dependencies used by the command layer.
type Bootstrap struct {
	Config      config.Config
	Build       BuildInfo
	Fileset     *fileset.Service
	BlockDevice *filesystem.Manager
	Updater     *update.Service
}

// New loads default configuration and constructs the application bootstrap.
func New(build BuildInfo) (*Bootstrap, error) {
	cfg, err := config.Load("")
	if err != nil {
		return nil, err
	}

	return NewWithConfig(build, cfg)
}

// NewWithConfig constructs the application bootstrap from an explicit config.
func NewWithConfig(build BuildInfo, cfg config.Config) (*Bootstrap, error) {
	stateStore, err := filesystem.NewStateStore(cfg)
	if err != nil {
		return nil, err
	}

	manifestStore := manifest.NewStore(cfg.MetadataFileName)
	worker := fileset.NewService(fileset.NewCatalog(), manifestStore)
	blockDevice := filesystem.NewManager(cfg, stateStore, filesystem.ExecRunner{})
	updater, err := update.NewService(update.Config{
		CurrentVersion: build.Version,
		Repository:     build.Repository,
	})
	if err != nil {
		return nil, err
	}

	return &Bootstrap{
		Config:      cfg,
		Build:       build,
		Fileset:     worker,
		BlockDevice: blockDevice,
		Updater:     updater,
	}, nil
}

// Reload refreshes the bootstrap dependencies using a new config file path.
func (b *Bootstrap) Reload(configFile string) error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}

	next, err := NewWithConfig(b.Build, cfg)
	if err != nil {
		return err
	}

	b.Config = next.Config
	b.Fileset = next.Fileset
	b.BlockDevice = next.BlockDevice
	b.Updater = next.Updater

	return nil
}
