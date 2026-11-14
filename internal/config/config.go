// Package config loads the config file: a set of named Directories, each a
// local path and the rclone Remotes it is copied to. Two names are
// reserved, "projects" and "demos"; every other Directory is only backed up.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Reserved Directory names, looked up by the commands that need them.
const (
	Projects = "projects"
	Demos    = "demos"
)

// Directory is one [dirs.<name>] entry.
type Directory struct {
	Name    string   `toml:"-"`
	Path    string   `toml:"path"`
	Remotes []string `toml:"remotes"`
}

// Config is the single configuration file at ~/.config/studioctl/config.toml.
type Config struct {
	Dirs map[string]Directory `toml:"dirs"`

	order []string // Directory names in file order
}

// Names returns the Directory names in the order the file lists them. A
// Config built in code rather than loaded has no file order, so its names
// are sorted.
func (c *Config) Names() []string {
	if len(c.order) == len(c.Dirs) {
		return append([]string(nil), c.order...)
	}
	names := make([]string, 0, len(c.Dirs))
	for n := range c.Dirs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Dir returns the Directory called name, if configured.
func (c *Config) Dir(name string) (Directory, bool) {
	d, ok := c.Dirs[name]
	if ok {
		d.Name = name
	}
	return d, ok
}

// DefaultPath returns ~/.config/studioctl/config.toml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "studioctl", "config.toml"), nil
}

// Load reads the config file from path. A leading "~/" in a Directory's
// path is expanded to the home directory.
func Load(path string) (*Config, error) {
	cfg, _, err := LoadChecked(path)
	return cfg, err
}

// LoadChecked is Load, also returning the keys the file sets that the
// config doesn't know (typos), sorted.
func LoadChecked(path string) (*Config, []string, error) {
	var cfg Config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config %s: %w", path, err)
	}

	for _, k := range md.Keys() {
		if len(k) == 2 && k[0] == "dirs" {
			cfg.order = append(cfg.order, k[1])
		}
	}

	var home string
	for name, d := range cfg.Dirs {
		if d.Path == "~" || strings.HasPrefix(d.Path, "~/") {
			if home == "" {
				if home, err = os.UserHomeDir(); err != nil {
					return nil, nil, fmt.Errorf("expanding ~ in dirs.%s.path: %w", name, err)
				}
			}
			d.Path = ExpandHome(d.Path, home)
		}
		d.Name = name
		cfg.Dirs[name] = d
	}

	var unknown []string
	for _, k := range md.Undecoded() {
		unknown = append(unknown, k.String())
	}
	sort.Strings(unknown)
	return &cfg, unknown, nil
}

// ExpandHome replaces a leading "~" or "~/" in p with home. Nothing else is
// expanded: no "~user", no environment variables.
func ExpandHome(p, home string) string {
	switch {
	case p == "~":
		return home
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(home, p[2:])
	}
	return p
}
