// Package backup builds and runs rclone copy jobs for registered artist
// roles (e.g. production, demos).
package backup

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rorycaraher/ableton-framework/internal/config"
	"github.com/rorycaraher/ableton-framework/internal/discovery"
)

// Job is one local-directory-to-remote rclone copy.
type Job struct {
	Artist    string
	Role      string
	LocalDir  string
	RemoteDir string
}

// BuildJobs expands the registry into concrete copy jobs, filtered by
// artist/role when non-empty. Each role's glob may match multiple local
// directories (e.g. several PRODUCTION-YYYY years); each becomes its own job,
// copied to a like-named subfolder of the role's remote.
func BuildJobs(reg *config.Registry, artistFilter, roleFilter string) ([]Job, error) {
	var jobs []Job

	artists := reg.ArtistNames()
	if artistFilter != "" {
		artists = []string{artistFilter}
		if _, ok := reg.Artists[artistFilter]; !ok {
			return nil, fmt.Errorf("artist %q is not registered", artistFilter)
		}
	}

	for _, artist := range artists {
		root, ok := reg.Artists[artist]
		if !ok {
			return nil, fmt.Errorf("artist %q is not registered", artist)
		}

		artistCfg, err := config.LoadArtistConfig(root)
		if err != nil {
			return nil, fmt.Errorf("artist %q: %w", artist, err)
		}

		roles := artistCfg.RoleNames()
		if roleFilter != "" {
			if _, ok := artistCfg.Roles[roleFilter]; !ok {
				return nil, fmt.Errorf("artist %q has no role %q", artist, roleFilter)
			}
			roles = []string{roleFilter}
		}

		for _, roleName := range roles {
			role := artistCfg.Roles[roleName]
			dirs, err := discovery.MatchRoleDirs(root, role.Glob)
			if err != nil {
				return nil, fmt.Errorf("artist %q role %q: %w", artist, roleName, err)
			}
			for _, dir := range dirs {
				jobs = append(jobs, Job{
					Artist:    artist,
					Role:      roleName,
					LocalDir:  dir,
					RemoteDir: joinRemote(role.Remote, filepath.Base(dir)),
				})
			}
		}
	}

	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Artist != jobs[j].Artist {
			return jobs[i].Artist < jobs[j].Artist
		}
		if jobs[i].Role != jobs[j].Role {
			return jobs[i].Role < jobs[j].Role
		}
		return jobs[i].LocalDir < jobs[j].LocalDir
	})

	return jobs, nil
}

func joinRemote(remote, name string) string {
	return strings.TrimRight(remote, "/") + "/" + name
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
