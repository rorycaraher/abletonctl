package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRegistry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFile(t, path, `
[artists]
artist-name = "/Users/rca/Music/artist-name"
other-artist = "/Volumes/External/other-artist"
`)

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reg.Artists["artist-name"]; got != "/Users/rca/Music/artist-name" {
		t.Fatalf("got %q", got)
	}
	names := reg.ArtistNames()
	if len(names) != 2 || names[0] != "artist-name" || names[1] != "other-artist" {
		t.Fatalf("got %v", names)
	}
}

func TestLoadArtistConfig(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ArtistConfigPath(root), `
[roles.production]
glob = "PRODUCTION-*"
remote = "r2:my-bucket/artist-name"

[roles.demos]
glob = "demos"
remote = "gdrive:artist-name/demos"
`)

	cfg, err := LoadArtistConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roles) != 2 {
		t.Fatalf("got %d roles", len(cfg.Roles))
	}
	if cfg.Roles["production"].Glob != "PRODUCTION-*" {
		t.Fatalf("got %+v", cfg.Roles["production"])
	}
	names := cfg.RoleNames()
	if len(names) != 2 || names[0] != "demos" || names[1] != "production" {
		t.Fatalf("got %v", names)
	}
}
