package backup

import (
	"reflect"
	"testing"

	"github.com/rorycaraher/abletonctl/internal/config"
)

func fullConfig() *config.Config {
	return &config.Config{
		ProjectsDir:    "/local/projects",
		DemosDir:       "/local/demos",
		ProjectsRemote: "r2:my-bucket/projects",
		DemosRemote:    "gdrive:my-demos",
	}
}

func TestBuildJobs_BothTargetsByDefault(t *testing.T) {
	jobs, err := BuildJobs(fullConfig(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2: %+v", len(jobs), jobs)
	}
	want := []Job{
		{Target: Projects, LocalDir: "/local/projects", RemoteDir: "r2:my-bucket/projects"},
		{Target: Demos, LocalDir: "/local/demos", RemoteDir: "gdrive:my-demos"},
	}
	if !reflect.DeepEqual(jobs, want) {
		t.Fatalf("got %+v, want %+v", jobs, want)
	}
}

func TestBuildJobs_FiltersByTarget(t *testing.T) {
	jobs, err := BuildJobs(fullConfig(), "demos")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Target != Demos {
		t.Fatalf("got %+v", jobs)
	}
}

func TestBuildJobs_UnknownTargetErrors(t *testing.T) {
	if _, err := BuildJobs(fullConfig(), "nope"); err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestBuildJobs_SkipsUnconfiguredTargetWhenUnfiltered(t *testing.T) {
	cfg := &config.Config{ProjectsDir: "/local/projects", ProjectsRemote: "r2:my-bucket/projects"}
	jobs, err := BuildJobs(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Target != Projects {
		t.Fatalf("got %+v", jobs)
	}
}

func TestBuildJobs_UnconfiguredTargetExplicitlyRequestedErrors(t *testing.T) {
	cfg := &config.Config{ProjectsDir: "/local/projects", ProjectsRemote: "r2:my-bucket/projects"}
	if _, err := BuildJobs(cfg, "demos"); err == nil {
		t.Fatal("expected error requesting an unconfigured target")
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
