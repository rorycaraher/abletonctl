package config

import (
	"os"
	"path/filepath"
	"reflect"
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
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, `
[dirs.projects]
path = "/path/to/projects"
remotes = ["r2:bucket/projects", "nas:music/projects"]

[dirs.demos]
path = "/path/to/demos"

[dirs.user-library]
path = "/path/to/User Library"
remotes = ["r2:bucket/user-library"]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.Names(), []string{"projects", "demos", "user-library"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want file order %v", got, want)
	}
	p, ok := cfg.Dir(Projects)
	want := Directory{Name: "projects", Path: "/path/to/projects", Remotes: []string{"r2:bucket/projects", "nas:music/projects"}}
	if !ok || !reflect.DeepEqual(p, want) {
		t.Fatalf("projects = %+v, %v", p, ok)
	}
	if d, ok := cfg.Dir(Demos); !ok || d.Path != "/path/to/demos" || len(d.Remotes) != 0 {
		t.Fatalf("demos = %+v, %v", d, ok)
	}
	if _, ok := cfg.Dir("nope"); ok {
		t.Fatal("unconfigured Directory found")
	}
}

func TestLoadExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, `
[dirs.demos]
path = "~/nltl/demos"
[dirs.home]
path = "~"
[dirs.other]
path = "~other/x"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"demos": filepath.Join(home, "nltl", "demos"),
		"home":  home,
		"other": "~other/x", // only the current user's home is expanded
	} {
		if d, _ := cfg.Dir(name); d.Path != want {
			t.Errorf("%s: path = %q, want %q", name, d.Path, want)
		}
	}
}

func TestLoadCheckedReportsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, `
projects_dir = "/old/flat/key"
[dirs.demos]
path = "/d"
remote = "r2:typo"
`)
	_, unknown, err := LoadChecked(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"dirs.demos.remote", "projects_dir"}; !reflect.DeepEqual(unknown, want) {
		t.Fatalf("unknown = %v, want %v", unknown, want)
	}
}

func TestNamesWithoutFileOrderAreSorted(t *testing.T) {
	cfg := &Config{Dirs: map[string]Directory{"b": {}, "a": {}, "c": {}}}
	if got := cfg.Names(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("got %v", got)
	}
}

func TestLoad_MissingFileErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.toml")); err == nil {
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
	want := filepath.Join(home, ".config", "studioctl", "config.toml")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
