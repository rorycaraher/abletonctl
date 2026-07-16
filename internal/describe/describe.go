// Package describe aggregates everything abletonctl knows about a single
// Project -- samples, its Track Catalog row, and matching Demo files --
// into one report. Backup freshness is deliberately omitted: Backup Jobs
// operate at Production-directory granularity, so no per-project backup
// state exists anywhere in the domain (see CONTEXT.md).
package describe

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rorycaraher/abletonctl/internal/config"
	"github.com/rorycaraher/abletonctl/internal/discovery"
	"github.com/rorycaraher/abletonctl/internal/samples"
	"github.com/rorycaraher/abletonctl/internal/tracks"
)

// Report aggregates everything known about a single Project.
type Report struct {
	Project discovery.Project

	// Artist is the registered artist name owning this Project, or "" if
	// the Project isn't structurally under any registered artist root.
	// Track Catalog and Demo matching are both artist-scoped, so they're
	// skipped (not failed) when Artist is "".
	Artist     string
	ArtistRoot string

	Samples        []samples.FileResult
	UsedCount      int
	UncertainCount int
	OrphanCount    int
	OrphanBytes    int64

	HasCatalog  bool
	CatalogPath string
	TrackFound  bool
	Track       tracks.Row

	DemosRoleConfigured bool
	// DemoMatches are paths relative to the artist root of demo files whose
	// name matches the Project name (see MatchesProjectName).
	DemoMatches []string
}

// Build aggregates a full report for project. reg may be nil (no registry
// configured yet), in which case Artist stays "" and the Track/Demo
// sections are left unpopulated rather than erroring.
func Build(project discovery.Project, reg *config.Registry) (*Report, error) {
	r := &Report{Project: project}

	results, err := samples.Scan(project)
	if err != nil {
		return nil, err
	}
	r.Samples = results
	for _, res := range results {
		switch res.Status {
		case samples.Used:
			r.UsedCount++
		case samples.Uncertain:
			r.UncertainCount++
		case samples.Orphan:
			r.OrphanCount++
			r.OrphanBytes += res.Size
		}
	}

	if reg == nil {
		return r, nil
	}
	artist, root, ok := findOwningArtist(project, reg)
	if !ok {
		return r, nil
	}
	r.Artist = artist
	r.ArtistRoot = root

	if err := loadTrack(r, root, project.Name); err != nil {
		return nil, err
	}
	if err := loadDemos(r, root, project.Name); err != nil {
		return nil, err
	}
	return r, nil
}

// findOwningArtist returns the registered artist whose root is project's
// grandparent directory -- the structural relationship every Project has
// to its Artist root (Artist/Production/Project; see CONTEXT.md).
func findOwningArtist(project discovery.Project, reg *config.Registry) (name, root string, ok bool) {
	projectParent := filepath.Clean(filepath.Dir(filepath.Dir(project.Path)))
	for _, candidate := range reg.ArtistNames() {
		abs, err := filepath.Abs(reg.Artists[candidate])
		if err != nil {
			continue
		}
		if filepath.Clean(abs) == projectParent {
			return candidate, filepath.Clean(abs), true
		}
	}
	return "", "", false
}

func loadTrack(r *Report, artistRoot, projectName string) error {
	path := tracks.CatalogPath(artistRoot)
	r.CatalogPath = path
	cat, err := tracks.Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	r.HasCatalog = true
	if i := cat.Find(projectName); i != -1 {
		r.TrackFound = true
		r.Track = cat.Rows[i]
	}
	return nil
}

func loadDemos(r *Report, artistRoot, projectName string) error {
	cfg, err := config.LoadArtistConfig(artistRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	role, ok := cfg.Roles["demos"]
	if !ok {
		return nil
	}
	r.DemosRoleConfigured = true

	dirs, err := discovery.MatchRoleDirs(artistRoot, role.Glob)
	if err != nil {
		return err
	}

	var matches []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() || !MatchesProjectName(e.Name(), projectName) {
				continue
			}
			rel, err := filepath.Rel(artistRoot, filepath.Join(dir, e.Name()))
			if err != nil {
				return err
			}
			matches = append(matches, rel)
		}
	}
	sort.Strings(matches)
	r.DemoMatches = matches
	return nil
}

// MatchesProjectName reports whether a demo's filename matches projectName
// under the Retitle-guaranteed name-prefix convention (see CONTEXT.md /
// docs/adr/0001-project-names-are-immutable.md): the name before the
// extension must equal projectName exactly, or extend it starting at a
// non-alphanumeric separator. "Song - alt.mp3" matches "Song";
// "Songbird.mp3" does not.
func MatchesProjectName(filename, projectName string) bool {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	if base == projectName {
		return true
	}
	if !strings.HasPrefix(base, projectName) {
		return false
	}
	rest := base[len(projectName):]
	next, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLetter(next) && !unicode.IsDigit(next)
}
