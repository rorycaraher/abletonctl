// Package backup builds and runs rclone copy jobs for the two fixed backup
// targets: the projects directory and the demos directory.
package backup

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/rorycaraher/abletonctl/internal/config"
)

// Target identifies which of the two directories a Job backs up.
type Target string

const (
	Projects Target = "projects"
	Demos    Target = "demos"
)

// Job is one local-directory-to-remote rclone copy. RemoteDir is the exact
// destination — rclone lands LocalDir's contents there directly, with no
// implicit subfolder.
type Job struct {
	Target    Target
	LocalDir  string
	RemoteDir string
}

// BuildJobs returns the backup jobs configured in cfg, filtered to
// targetFilter when non-empty ("projects" or "demos"). A target with an
// empty local dir or remote is treated as unconfigured: silently omitted
// when no filter is given, or an error when explicitly requested.
func BuildJobs(cfg *config.Config, targetFilter string) ([]Job, error) {
	if targetFilter != "" && targetFilter != string(Projects) && targetFilter != string(Demos) {
		return nil, fmt.Errorf("unknown target %q (want %q or %q)", targetFilter, Projects, Demos)
	}

	candidates := []Job{
		{Target: Projects, LocalDir: cfg.ProjectsDir, RemoteDir: cfg.ProjectsRemote},
		{Target: Demos, LocalDir: cfg.DemosDir, RemoteDir: cfg.DemosRemote},
	}

	var jobs []Job
	for _, j := range candidates {
		if targetFilter != "" && string(j.Target) != targetFilter {
			continue
		}
		if j.LocalDir == "" || j.RemoteDir == "" {
			if targetFilter == string(j.Target) {
				return nil, fmt.Errorf("%s target is not fully configured (need both a directory and a remote)", j.Target)
			}
			continue
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// CopyArgs builds the rclone argv for copying a job. copy (not sync) is used
// deliberately: it never deletes files on the remote, so a local mistake
// can't take out the only backup copy too.
func CopyArgs(j Job, dryRun bool) []string {
	args := []string{"copy", j.LocalDir, j.RemoteDir}
	if dryRun {
		args = append(args, "--dry-run")
	}
	return args
}

// Run executes a single job via the rclone binary, streaming its output.
func Run(j Job, dryRun bool, stdout, stderr io.Writer) error {
	cmd := exec.Command("rclone", CopyArgs(j, dryRun)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone copy %s -> %s: %w", j.LocalDir, j.RemoteDir, err)
	}
	return nil
}
