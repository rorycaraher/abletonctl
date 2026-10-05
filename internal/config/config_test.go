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

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFile(t, path, `
projects_dir = "/path/to/projects"
demos_dir = "/path/to/demos"
projects_remote = "r2:my-bucket/projects"
demos_remote = "gdrive:my-demos"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProjectsDir != "/path/to/projects" {
		t.Fatalf("got %q", cfg.ProjectsDir)
	}
	if cfg.DemosDir != "/path/to/demos" {
		t.Fatalf("got %q", cfg.DemosDir)
	}
	if cfg.ProjectsRemote != "r2:my-bucket/projects" {
		t.Fatalf("got %q", cfg.ProjectsRemote)
	}
	if cfg.DemosRemote != "gdrive:my-demos" {
		t.Fatalf("got %q", cfg.DemosRemote)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	writeFile(t, path, `
projects_dir = "/path/to/projects"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DemosDir != "" {
		t.Fatalf("expected empty DemosDir, got %q", cfg.DemosDir)
	}
}

func TestLoad_MissingFileErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(filepath.Join(dir, "nope.toml")); err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestDefaultPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "abletonctl", "config.toml")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
