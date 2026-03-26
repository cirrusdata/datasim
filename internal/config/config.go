package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

type Config struct {
	ConfigFile            string `mapstructure:"config_file"`
	MetadataFileName      string `mapstructure:"metadata_file_name"`
	DefaultLinuxMountRoot string `mapstructure:"default_linux_mount_root"`
	DefaultWindowsMount   string `mapstructure:"default_windows_mount_root"`
	DefaultLinuxFSType    string `mapstructure:"default_linux_fs_type"`
	DefaultWindowsFSType  string `mapstructure:"default_windows_fs_type"`
	StateFile             string `mapstructure:"state_file"`
}

// Load reads configuration from disk and environment variables.
func Load(configFile string) (Config, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return Config{}, err
	}

	appConfigDir := filepath.Join(cfgDir, "datasim")
	stateFile := filepath.Join(appConfigDir, "state.json")

	v := viper.New()
	v.SetConfigName("datasim")
	v.SetEnvPrefix("DATASIM")
	v.AutomaticEnv()

	v.SetDefault("metadata_file_name", ".cirrusdata-datasim")
	v.SetDefault("default_linux_mount_root", "/mnt")
	v.SetDefault("default_windows_mount_root", `C:\mnt`)
	v.SetDefault("default_linux_fs_type", "xfs")
	v.SetDefault("default_windows_fs_type", "ntfs")
	v.SetDefault("state_file", stateFile)

	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.AddConfigPath(appConfigDir)
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return Config{}, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}

	cfg.ConfigFile = v.ConfigFileUsed()

	return cfg, nil
}

// DefaultMountRoot returns the platform-specific default mount root.
func (c Config) DefaultMountRoot() string {
	if runtime.GOOS == "windows" {
		return c.DefaultWindowsMount
	}

	return c.DefaultLinuxMountRoot
}

// DefaultFSType returns the platform-specific default filesystem type.
func (c Config) DefaultFSType() string {
	if runtime.GOOS == "windows" {
		return c.DefaultWindowsFSType
	}

	return c.DefaultLinuxFSType
}
