// Package config loads the top-level artist registry and per-artist role
// configuration used to drive discovery and backups.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// Registry maps artist names to their root directory on disk.
type Registry struct {
	Artists map[string]string `toml:"artists"`
	// Library optionally configures backup of the local Ableton User
	// Library. Unlike artists, there's exactly one per machine, so it's a
	// single optional section rather than a map.
	Library *Library `toml:"library"`
}

// Library configures backup of the Ableton User Library: presets, samples,
// M4L devices, and templates shared across every project, not tied to any
// one artist namespace.
type Library struct {
	// Path is the User Library directory. Defaults to
	// ~/Music/Ableton/User Library (Ableton's own default location) when empty.
	Path string `toml:"path"`
	// Remote is the rclone remote the library is copied to, following the
	// same "parent directory" convention as a Role's remote: the library
	// folder itself lands in a like-named subfolder there.
	Remote string `toml:"remote"`
}

// DefaultUserLibraryPath returns Ableton's default User Library location,
// ~/Music/Ableton/User Library.
func DefaultUserLibraryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Music", "Ableton", "User Library"), nil
}

// ResolvedPath returns l.Path, falling back to DefaultUserLibraryPath if unset.
func (l *Library) ResolvedPath() (string, error) {
	if l.Path != "" {
		return l.Path, nil
	}
	return DefaultUserLibraryPath()
}

// DefaultRegistryPath returns ~/.config/abletonctl/config.toml.
func DefaultRegistryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "abletonctl", "config.toml"), nil
}

// LoadRegistry reads the artist registry from path.
func LoadRegistry(path string) (*Registry, error) {
	var reg Registry
	if _, err := toml.DecodeFile(path, &reg); err != nil {
		return nil, fmt.Errorf("loading registry %s: %w", path, err)
	}
	return &reg, nil
}

// ArtistNames returns registered artist names, sorted.
func (r *Registry) ArtistNames() []string {
	names := make([]string, 0, len(r.Artists))
	for name := range r.Artists {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Role describes one backup role for an artist: which directories under the
// artist root it covers (by glob pattern) and which rclone remote they copy to.
type Role struct {
	Glob   string `toml:"glob"`
	Remote string `toml:"remote"`
}

// ArtistConfig is the per-artist config file living at <root>/.abletonctl.toml.
type ArtistConfig struct {
	Roles map[string]Role `toml:"roles"`
}

// ArtistConfigPath returns the per-artist config path for a given artist root.
func ArtistConfigPath(artistRoot string) string {
	return filepath.Join(artistRoot, ".abletonctl.toml")
}

// LoadArtistConfig reads the per-artist role config for the artist rooted at artistRoot.
func LoadArtistConfig(artistRoot string) (*ArtistConfig, error) {
	path := ArtistConfigPath(artistRoot)
	var cfg ArtistConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("loading artist config %s: %w", path, err)
	}
	return &cfg, nil
}

// RoleNames returns configured role names, sorted.
func (c *ArtistConfig) RoleNames() []string {
	names := make([]string, 0, len(c.Roles))
	for name := range c.Roles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
