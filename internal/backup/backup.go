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

	"github.com/rorycaraher/abletonctl/internal/config"
	"github.com/rorycaraher/abletonctl/internal/discovery"
)

// Job is one local-directory-to-remote rclone copy.
type Job struct {
	Artist    string
	Role      string
	LocalDir  string
	RemoteDir string
}

// LibraryArtist is the pseudo artist name used to address the single
// machine-level User Library backup job via the existing --artist flag
// (e.g. `abletonctl backup --artist user-library`), rather than adding a
// separate flag for what is otherwise just another backup job.
const LibraryArtist = "user-library"

// libraryRole is the pseudo role name reported for the User Library job.
// The library has no configurable roles, so this is purely cosmetic output.
const libraryRole = "library"

// BuildJobs expands the registry into concrete copy jobs, filtered by
// artist/role when non-empty. Each role's glob may match multiple local
// directories (e.g. several PRODUCTION-YYYY years); each becomes its own job,
// copied to a like-named subfolder of the role's remote.
//
// If the registry has a [library] section, its backup job is included
// whenever neither filter narrows the run to a specific artist/role, or
// selected on its own via artistFilter == LibraryArtist.
func BuildJobs(reg *config.Registry, artistFilter, roleFilter string) ([]Job, error) {
	if artistFilter == LibraryArtist {
		if roleFilter != "" {
			return nil, fmt.Errorf("%q has no roles; omit --role", LibraryArtist)
		}
		job, err := libraryJob(reg)
		if err != nil {
			return nil, err
		}
		if job == nil {
			return nil, fmt.Errorf("no [library] configured in the registry")
		}
		return []Job{*job}, nil
	}

	var jobs []Job

	if artistFilter == "" && roleFilter == "" {
		job, err := libraryJob(reg)
		if err != nil {
			return nil, err
		}
		if job != nil {
			jobs = append(jobs, *job)
		}
	}

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

// libraryJob builds the User Library backup job from reg.Library, or returns
// nil if no [library] section is configured.
func libraryJob(reg *config.Registry) (*Job, error) {
	if reg.Library == nil {
		return nil, nil
	}
	if reg.Library.Remote == "" {
		return nil, fmt.Errorf("[library] is configured but has no remote")
	}
	path, err := reg.Library.ResolvedPath()
	if err != nil {
		return nil, fmt.Errorf("resolving user library path: %w", err)
	}
	return &Job{
		Artist:    LibraryArtist,
		Role:      libraryRole,
		LocalDir:  path,
		RemoteDir: joinRemote(reg.Library.Remote, filepath.Base(path)),
	}, nil
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
