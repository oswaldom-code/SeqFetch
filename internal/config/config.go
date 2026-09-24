// Package config persists user preferences in a YAML file under the
// OS-specific user config directory (e.g. ~/.config/seqfetch/config.yaml).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	appDir   = "seqfetch"
	fileName = "config.yaml"
)

// Config holds the persisted settings.
type Config struct {
	DownloadDir string `yaml:"download_dir"`
}

// DefaultPath returns the config file location for the current user.
func DefaultPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(base, appDir, fileName), nil
}

// Load reads the config at path. A missing file yields a zero Config.
func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// Save writes cfg to path, creating parent directories as needed.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ResolveDownloadDir picks the download directory with precedence:
// explicit flag > persisted config > current working directory.
func ResolveDownloadDir(flag string, cfg Config) (string, error) {
	switch {
	case flag != "":
		return filepath.Abs(flag)
	case cfg.DownloadDir != "":
		return filepath.Abs(cfg.DownloadDir)
	default:
		return os.Getwd()
	}
}
