package backup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rorycaraher/ableton-framework/internal/config"
)

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupArtist(t *testing.T) (root string, reg *config.Registry) {
	t.Helper()
	root = t.TempDir()
	mustMkdir(t, filepath.Join(root, "PRODUCTION-2025"))
	mustMkdir(t, filepath.Join(root, "PRODUCTION-2026"))
	mustMkdir(t, filepath.Join(root, "demos"))
	mustWrite(t, config.ArtistConfigPath(root), `
[roles.production]
glob = "PRODUCTION-*"
remote = "r2:my-bucket/artist-name"

[roles.demos]
glob = "demos"
remote = "gdrive:artist-name"
`)

	reg = &config.Registry{Artists: map[string]string{"artist-name": root}}
	return root, reg
}

func TestBuildJobs_ExpandsGlobPerRole(t *testing.T) {
	_, reg := setupArtist(t)

	jobs, err := BuildJobs(reg, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 {
		t.Fatalf("got %d jobs, want 3 (two production years + demos): %+v", len(jobs), jobs)
	}

	var remotes []string
	for _, j := range jobs {
		remotes = append(remotes, j.RemoteDir)
	}
	want := []string{
		"gdrive:artist-name/demos",
		"r2:my-bucket/artist-name/PRODUCTION-2025",
		"r2:my-bucket/artist-name/PRODUCTION-2026",
	}
	if !reflect.DeepEqual(remotes, want) {
		t.Fatalf("got remotes %v, want %v", remotes, want)
	}
}

func TestBuildJobs_FiltersByArtistAndRole(t *testing.T) {
	_, reg := setupArtist(t)

	jobs, err := BuildJobs(reg, "artist-name", "demos")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Role != "demos" {
		t.Fatalf("got %+v", jobs)
	}
}

func TestBuildJobs_UnknownArtistErrors(t *testing.T) {
	_, reg := setupArtist(t)
	if _, err := BuildJobs(reg, "nope", ""); err == nil {
		t.Fatal("expected error for unregistered artist")
	}
}

func TestBuildJobs_IncludesLibraryByDefault(t *testing.T) {
	_, reg := setupArtist(t)
	libRoot := t.TempDir()
	reg.Library = &config.Library{Path: libRoot, Remote: "r2:my-bucket/user-library"}

	jobs, err := BuildJobs(reg, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 4 {
		t.Fatalf("got %d jobs, want 4 (library + two production years + demos): %+v", len(jobs), jobs)
	}

	var lib *Job
	for i := range jobs {
		if jobs[i].Artist == LibraryArtist {
			lib = &jobs[i]
		}
	}
	if lib == nil {
		t.Fatal("expected a library job")
	}
	if lib.LocalDir != libRoot {
		t.Fatalf("got LocalDir %q, want %q", lib.LocalDir, libRoot)
	}
	wantRemote := "r2:my-bucket/user-library/" + filepath.Base(libRoot)
	if lib.RemoteDir != wantRemote {
		t.Fatalf("got RemoteDir %q, want %q", lib.RemoteDir, wantRemote)
	}
}

func TestBuildJobs_ArtistFilterExcludesLibrary(t *testing.T) {
	_, reg := setupArtist(t)
	reg.Library = &config.Library{Path: t.TempDir(), Remote: "r2:my-bucket/user-library"}

	jobs, err := BuildJobs(reg, "artist-name", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.Artist == LibraryArtist {
			t.Fatalf("expected library job to be excluded when filtering by artist, got %+v", jobs)
		}
	}
}

func TestBuildJobs_LibraryArtistSelectsOnlyLibrary(t *testing.T) {
	_, reg := setupArtist(t)
	libRoot := t.TempDir()
	reg.Library = &config.Library{Path: libRoot, Remote: "r2:my-bucket/user-library"}

	jobs, err := BuildJobs(reg, LibraryArtist, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Artist != LibraryArtist || jobs[0].LocalDir != libRoot {
		t.Fatalf("got %+v", jobs)
	}
}

func TestBuildJobs_LibraryArtistWithRoleErrors(t *testing.T) {
	_, reg := setupArtist(t)
	reg.Library = &config.Library{Path: t.TempDir(), Remote: "r2:my-bucket/user-library"}

	if _, err := BuildJobs(reg, LibraryArtist, "demos"); err == nil {
		t.Fatal("expected error when combining --artist user-library with --role")
	}
}

func TestBuildJobs_LibraryArtistWithoutConfigErrors(t *testing.T) {
	_, reg := setupArtist(t)
	if _, err := BuildJobs(reg, LibraryArtist, ""); err == nil {
		t.Fatal("expected error when [library] isn't configured")
	}
}

func TestBuildJobs_NoLibraryConfigured(t *testing.T) {
	_, reg := setupArtist(t)

	jobs, err := BuildJobs(reg, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.Artist == LibraryArtist {
			t.Fatalf("expected no library job when unconfigured, got %+v", jobs)
		}
	}
}

func TestCopyArgs_UsesCopyNotSyncAndNeverDeletes(t *testing.T) {
	j := Job{LocalDir: "/local/dir", RemoteDir: "r2:bucket/dir"}
	args := CopyArgs(j, false)
	want := []string{"copy", "/local/dir", "r2:bucket/dir"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("got %v, want %v", args, want)
	}

	dryRunArgs := CopyArgs(j, true)
	if dryRunArgs[len(dryRunArgs)-1] != "--dry-run" {
		t.Fatalf("expected --dry-run to be passed through: %v", dryRunArgs)
	}
}
