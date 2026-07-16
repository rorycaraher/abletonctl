package describe

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/rorycaraher/abletonctl/internal/config"
	"github.com/rorycaraher/abletonctl/internal/discovery"
)

const minimalAls = `<?xml version="1.0" encoding="UTF-8"?><Ableton><LiveSet></LiveSet></Ableton>`

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeMinimalAls(t *testing.T, path string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	if _, err := gz.Write([]byte(minimalAls)); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path string, contents string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupArtist builds an artist root with one Project ("Song") under
// PRODUCTION-2026, and returns (artistRoot, project, registry).
func setupArtist(t *testing.T) (string, discovery.Project, *config.Registry) {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, "PRODUCTION-2026", "Song")
	alsPath := filepath.Join(projectPath, "Song.als")
	writeMinimalAls(t, alsPath)

	project := discovery.Project{
		Name:     "Song",
		Path:     projectPath,
		AlsFiles: []string{alsPath},
	}
	reg := &config.Registry{Artists: map[string]string{"artist-name": root}}
	return root, project, reg
}

func TestBuild_ArtistUnresolvedWithoutRegistry(t *testing.T) {
	_, project, _ := setupArtist(t)

	r, err := Build(project, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Artist != "" {
		t.Errorf("got artist %q, want unresolved", r.Artist)
	}
	if r.HasCatalog || r.DemosRoleConfigured {
		t.Errorf("expected catalog/demos to be skipped without a registry, got %+v", r)
	}
}

func TestBuild_ArtistUnresolvedWhenNotRegistered(t *testing.T) {
	_, project, _ := setupArtist(t)
	reg := &config.Registry{Artists: map[string]string{"someone-else": t.TempDir()}}

	r, err := Build(project, reg)
	if err != nil {
		t.Fatal(err)
	}
	if r.Artist != "" {
		t.Errorf("got artist %q, want unresolved", r.Artist)
	}
}

func TestBuild_ResolvesArtistAndSkipsMissingCatalogAndDemosRole(t *testing.T) {
	root, project, reg := setupArtist(t)
	// No .abletonctl.toml, no .abletonctl-tracks.csv at all.
	_ = root

	r, err := Build(project, reg)
	if err != nil {
		t.Fatal(err)
	}
	if r.Artist != "artist-name" {
		t.Errorf("got artist %q, want artist-name", r.Artist)
	}
	if r.HasCatalog {
		t.Error("expected no catalog when none exists on disk")
	}
	if r.DemosRoleConfigured {
		t.Error("expected demos role to be unconfigured when no .abletonctl.toml exists")
	}
}

func TestBuild_TrackCatalogMatchAndMismatch(t *testing.T) {
	root, project, reg := setupArtist(t)
	mustWriteFile(t, filepath.Join(root, ".abletonctl-tracks.csv"),
		"Track,Status,Priority\nSong,Mixdown,A\nOther Song,Idea,B\n")

	r, err := Build(project, reg)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasCatalog {
		t.Fatal("expected catalog to be found")
	}
	if !r.TrackFound {
		t.Fatal("expected a matching row for Song")
	}
	if r.Track["Status"] != "Mixdown" || r.Track["Priority"] != "A" {
		t.Errorf("unexpected row: %+v", r.Track)
	}
}

func TestBuild_TrackCatalogNoMatchingRow(t *testing.T) {
	root, project, reg := setupArtist(t)
	mustWriteFile(t, filepath.Join(root, ".abletonctl-tracks.csv"),
		"Track,Status\nSome Other Track,Idea\n")

	r, err := Build(project, reg)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasCatalog {
		t.Fatal("expected catalog to be found")
	}
	if r.TrackFound {
		t.Errorf("expected no matching row, got %+v", r.Track)
	}
}

func TestBuild_DemoMatching(t *testing.T) {
	root, project, reg := setupArtist(t)
	mustWriteFile(t, filepath.Join(root, ".abletonctl.toml"),
		"[roles.demos]\nglob = \"demos\"\nremote = \"gdrive:artist-name\"\n")

	demosDir := filepath.Join(root, "demos")
	mustWriteFile(t, filepath.Join(demosDir, "Song.mp3"), "x")
	mustWriteFile(t, filepath.Join(demosDir, "Song - alt.mp3"), "x")
	// Should NOT match: shares a prefix but isn't a name-boundary match.
	mustWriteFile(t, filepath.Join(demosDir, "Songbird.mp3"), "x")
	// Unrelated track, should not match.
	mustWriteFile(t, filepath.Join(demosDir, "Other Song.mp3"), "x")

	r, err := Build(project, reg)
	if err != nil {
		t.Fatal(err)
	}
	if !r.DemosRoleConfigured {
		t.Fatal("expected demos role to be configured")
	}
	want := []string{
		filepath.Join("demos", "Song - alt.mp3"),
		filepath.Join("demos", "Song.mp3"),
	}
	if len(r.DemoMatches) != len(want) {
		t.Fatalf("got matches %v, want %v", r.DemoMatches, want)
	}
	for i := range want {
		if r.DemoMatches[i] != want[i] {
			t.Errorf("got matches %v, want %v", r.DemoMatches, want)
			break
		}
	}
}

func TestMatchesProjectName(t *testing.T) {
	cases := []struct {
		filename, project string
		want               bool
	}{
		{"Song.mp3", "Song", true},
		{"Song - alt.mp3", "Song", true},
		{"Song (v2).aiff", "Song", true},
		{"Song_bounce.wav", "Song", true},
		{"Songbird.mp3", "Song", false},
		{"Other Song.mp3", "Song", false},
	}
	for _, c := range cases {
		if got := MatchesProjectName(c.filename, c.project); got != c.want {
			t.Errorf("MatchesProjectName(%q, %q) = %v, want %v", c.filename, c.project, got, c.want)
		}
	}
}
