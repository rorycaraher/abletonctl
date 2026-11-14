package backup

import (
	"reflect"
	"testing"

	"github.com/rorycaraher/studioctl/internal/config"
)

func fullConfig() *config.Config {
	return &config.Config{Dirs: map[string]config.Directory{
		"projects":     {Path: "/local/projects", Remotes: []string{"r2:bucket/projects", "nas:music/projects"}},
		"demos":        {Path: "/local/demos", Remotes: []string{"gdrive:demos"}},
		"user-library": {Path: "/local/user-library", Remotes: []string{"r2:bucket/user-library"}},
		"scratch":      {Path: "/local/scratch"},
	}}
}

func TestBuildJobs_EveryRemoteOfEveryDirectory(t *testing.T) {
	jobs, err := BuildJobs(fullConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Built in code, so Directories come in name order; scratch has no Remotes.
	want := []Job{
		{Dir: "demos", LocalDir: "/local/demos", RemoteDir: "gdrive:demos"},
		{Dir: "projects", LocalDir: "/local/projects", RemoteDir: "r2:bucket/projects"},
		{Dir: "projects", LocalDir: "/local/projects", RemoteDir: "nas:music/projects"},
		{Dir: "user-library", LocalDir: "/local/user-library", RemoteDir: "r2:bucket/user-library"},
	}
	if !reflect.DeepEqual(jobs, want) {
		t.Fatalf("got  %+v\nwant %+v", jobs, want)
	}
}

func TestBuildJobs_NamedDirectoriesInGivenOrder(t *testing.T) {
	jobs, err := BuildJobs(fullConfig(), []string{"user-library", "demos"})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 || jobs[0].Dir != "user-library" || jobs[1].Dir != "demos" {
		t.Fatalf("got %+v", jobs)
	}
}

func TestBuildJobs_UnknownDirectoryErrors(t *testing.T) {
	if _, err := BuildJobs(fullConfig(), []string{"nope"}); err == nil {
		t.Fatal("expected error for unknown Directory")
	}
}

func TestBuildJobs_NamedDirectoryWithoutRemotesErrors(t *testing.T) {
	if _, err := BuildJobs(fullConfig(), []string{"scratch"}); err == nil {
		t.Fatal("expected error naming a Directory without Remotes")
	}
}

func TestCopyArgs_UsesCopyNotSyncAndNeverDeletes(t *testing.T) {
	j := Job{LocalDir: "/local/dir", RemoteDir: "r2:bucket/dir"}
	args := CopyArgs(j, false, true)
	want := []string{"copy", "/local/dir", "r2:bucket/dir", "--exclude", ".DS_Store"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("got %v, want %v", args, want)
	}

	verbose := CopyArgs(j, false, false)
	if !reflect.DeepEqual(verbose, append(want, "-v")) {
		t.Fatalf("expected -v unless quiet: %v", verbose)
	}

	dryRunArgs := CopyArgs(j, true, true)
	if dryRunArgs[len(dryRunArgs)-1] != "--dry-run" {
		t.Fatalf("expected --dry-run to be passed through: %v", dryRunArgs)
	}
}

func TestListArgs(t *testing.T) {
	got := ListArgs("gdrive:NLTL/demos-backup")
	want := []string{"lsf", "--recursive", "--files-only", "--contimeout", "20s", "--timeout", "60s", "gdrive:NLTL/demos-backup"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for _, a := range got {
		if a == "copy" || a == "sync" || a == "delete" || a == "purge" {
			t.Fatalf("listing must be read-only, got %q", a)
		}
	}
}
