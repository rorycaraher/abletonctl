// Package config loads the flat abletonctl config file: where the projects
// directory and demos directory live, and which rclone remote each backs up
// to.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the single configuration file at ~/.config/abletonctl/config.toml.
type Config struct {
	ProjectsDir    string `toml:"projects_dir"`
	DemosDir       string `toml:"demos_dir"`
	ProjectsRemote string `toml:"projects_remote"`
	DemosRemote    string `toml:"demos_remote"`
}

// DefaultPath returns ~/.config/abletonctl/config.toml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "abletonctl", "config.toml"), nil
}

// Load reads the config file from path.
func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("loading config %s: %w", path, err)
	}
	return &cfg, nil
}
