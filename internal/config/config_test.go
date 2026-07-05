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

func TestLoadRegistry_Library(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFile(t, path, `
[artists]
artist-name = "/Users/rca/Music/artist-name"

[library]
path = "/Users/rca/Music/Ableton/User Library"
remote = "r2:my-bucket/user-library"
`)

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if reg.Library == nil {
		t.Fatal("expected Library to be set")
	}
	if got, err := reg.Library.ResolvedPath(); err != nil || got != "/Users/rca/Music/Ableton/User Library" {
		t.Fatalf("got %q, %v", got, err)
	}
	if reg.Library.Remote != "r2:my-bucket/user-library" {
		t.Fatalf("got %q", reg.Library.Remote)
	}
}

func TestLoadRegistry_NoLibrary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFile(t, path, `
[artists]
artist-name = "/Users/rca/Music/artist-name"
`)

	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if reg.Library != nil {
		t.Fatalf("expected no Library, got %+v", reg.Library)
	}
}

func TestLibrary_ResolvedPath_DefaultsWhenEmpty(t *testing.T) {
	lib := &Library{Remote: "r2:my-bucket/user-library"}
	got, err := lib.ResolvedPath()
	if err != nil {
		t.Fatal(err)
	}
	want, err := DefaultUserLibraryPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
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
