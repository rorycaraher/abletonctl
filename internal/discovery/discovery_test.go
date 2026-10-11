package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustTouch(t *testing.T, path string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverProjects_DetectsViaTopLevelAls(t *testing.T) {
	prod := t.TempDir()

	// Real project: has a top-level .als.
	mustTouch(t, filepath.Join(prod, "Track One", "Track One.als"))
	// Non-project folder: no .als anywhere at the top level.
	mustMkdir(t, filepath.Join(prod, "Reference Material"))
	mustTouch(t, filepath.Join(prod, "Reference Material", "notes.txt"))
	// .als exists but only nested in a Backup folder, not top-level -> not a project.
	mustTouch(t, filepath.Join(prod, "Half Baked", "Backup", "Half Baked [2024-01-01 120000].als"))

	projects, err := DiscoverProjects(prod)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("got %d projects, want 1: %+v", len(projects), projects)
	}
	if projects[0].Name != "Track One" {
		t.Fatalf("got project %q, want Track One", projects[0].Name)
	}
	if len(projects[0].AlsFiles) != 1 {
		t.Fatalf("got %d als files, want 1", len(projects[0].AlsFiles))
	}
}

func TestDiscoverProjects_IgnoresBackupFolderAls(t *testing.T) {
	prod := t.TempDir()
	mustTouch(t, filepath.Join(prod, "Song", "Song.als"))
	mustTouch(t, filepath.Join(prod, "Song", "Backup", "Song [old].als"))

	projects, err := DiscoverProjects(prod)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || len(projects[0].AlsFiles) != 1 {
		t.Fatalf("expected exactly one top-level als, got %+v", projects)
	}
}

func TestProjectNameAndFind(t *testing.T) {
	if got := ProjectName("installations Project"); got != "installations" {
		t.Errorf("ProjectName = %q", got)
	}
	if got := ProjectName("Track One"); got != "Track One" {
		t.Errorf("ProjectName = %q", got)
	}
	ps := []Project{
		{Name: "installations", Path: "/p/installations Project"},
		{Name: "dup", Path: "/p/dup"},
		{Name: "dup", Path: "/p/dup Project"},
	}
	for _, in := range []string{"installations", "installations Project"} {
		p, err := FindProject(ps, in)
		if err != nil || p.Path != "/p/installations Project" {
			t.Errorf("FindProject(%q) = %v, %v", in, p, err)
		}
	}
	if _, err := FindProject(ps, "missing"); err == nil {
		t.Error("want error for missing project")
	}
	if _, err := FindProject(ps, "dup"); err == nil {
		t.Error("want error for ambiguous project")
	}
}
