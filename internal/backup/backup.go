// Package backup builds and runs rclone copy jobs: one for each Directory
// and each of its Remotes.
package backup

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/rorycaraher/abletonctl/internal/config"
)

// Job is one local-directory-to-remote rclone copy. RemoteDir is the exact
// destination — rclone lands LocalDir's contents there directly, with no
// implicit subfolder.
type Job struct {
	Dir       string // Directory name
	LocalDir  string
	RemoteDir string
}

// BuildJobs returns a Job for every Remote of every Directory, in config
// file order. With names, only those Directories are included, in that
// order. A Directory without Remotes is skipped, or an error when it was
// named explicitly; an unknown name is an error.
func BuildJobs(cfg *config.Config, names []string) ([]Job, error) {
	explicit := len(names) > 0
	if !explicit {
		names = cfg.Names()
	}
	var jobs []Job
	for _, name := range names {
		d, ok := cfg.Dir(name)
		if !ok {
			return nil, fmt.Errorf("no [dirs.%s] in config", name)
		}
		if len(d.Remotes) == 0 {
			if explicit {
				return nil, fmt.Errorf("dirs.%s has no remotes", name)
			}
			continue
		}
		for _, r := range d.Remotes {
			jobs = append(jobs, Job{Dir: name, LocalDir: d.Path, RemoteDir: r})
		}
	}
	return jobs, nil
}

// CopyArgs builds the rclone argv for copying a job. copy (not sync) is used
// deliberately: it never deletes files on the remote, so a local mistake
// can't take out the only backup copy too. Unless quiet, -v makes rclone log
// each file it transfers. Finder's .DS_Store files are excluded at any depth.
func CopyArgs(j Job, dryRun, quiet bool) []string {
	args := []string{"copy", j.LocalDir, j.RemoteDir, "--exclude", ".DS_Store"}
	if !quiet {
		args = append(args, "-v")
	}
	if dryRun {
		args = append(args, "--dry-run")
	}
	return args
}

// Run executes a single job via the rclone binary, streaming its output.
func Run(j Job, dryRun, quiet bool, stdout, stderr io.Writer) error {
	cmd := exec.Command("rclone", CopyArgs(j, dryRun, quiet)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone copy: %w", err)
	}
	return nil
}

// ListArgs builds the rclone argv for listing every file under a remote,
// recursively, as paths relative to it. The timeouts keep an unreachable
// remote from hanging the command for minutes.
func ListArgs(remote string) []string {
	return []string{"lsf", "--recursive", "--files-only", "--contimeout", "20s", "--timeout", "60s", remote}
}

// ListRemote lists the files under remote via the rclone binary and returns
// their paths relative to it, slash-separated. It only reads.
func ListRemote(remote string) ([]string, error) {
	cmd := exec.Command("rclone", ListArgs(remote)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("rclone lsf %s: %w", remote, err)
		}
		return nil, fmt.Errorf("rclone lsf %s: %w: %s", remote, err, msg)
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}
