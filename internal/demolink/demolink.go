// Package demolink relates Demos (rendered audio in the demos directory) to
// Versions (top-level .als files) using a filename convention: a Demo's
// name is either a version slug exactly, or "<version-slug>__<descriptor>".
// The link is derived from names on every run; nothing is stored, so
// renaming a Version dangles its Demos by design.
package demolink

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rorycaraher/abletonctl/internal/discovery"
)

// Separator divides the version slug from the human descriptor in a Demo name.
const Separator = "__"

// Status is how a Demo relates to the Versions on disk.
type Status string

const (
	// Linked: the slug matches exactly one Version.
	Linked Status = "linked"
	// Ambiguous: the slug matches Versions in more than one Project; never guessed.
	Ambiguous Status = "ambiguous"
	// Dangling: the name uses the separator but its slug matches no Version.
	Dangling Status = "dangling"
	// Unlinked: no separator and no exact match, so intent can't be told.
	Unlinked Status = "unlinked"
)

// Version is a top-level .als file inside a Project.
type Version struct {
	Slug        string // file name without .als
	Path        string // absolute path of the .als
	ProjectName string // Project folder name
	ProjectPath string
}

// Location is where a Demo is found. It is decided by name only, never by
// comparing content, so Both does not mean the two files are identical.
type Location string

const (
	Local  Location = "local"
	Remote Location = "remote"
	Both   Location = "both"
)

// Demo is one rendered piece of audio, grouped by directory and name so a
// .wav and its .mp3 are a single Demo.
type Demo struct {
	Name     string   // file name without extension
	Dir      string   // directory relative to the demos root, slash-separated; "" at the root
	Formats  []string // lowercase extensions without the dot, sorted
	Location Location
}

// Link is the result of resolving one Demo.
type Link struct {
	Demo       Demo
	Status     Status
	Version    *Version  // set when Linked
	Candidates []Version // set when Ambiguous
}

var audioExts = map[string]bool{".wav": true, ".aiff": true, ".aif": true, ".mp3": true}

// Parse splits a Demo name into its version slug and descriptor, splitting
// on the first separator. hasSep reports whether the separator was present;
// without it the whole name is the slug.
func Parse(name string) (slug, descriptor string, hasSep bool) {
	return strings.Cut(name, Separator)
}

// GroupPaths groups file paths (relative to a demos root, slash-separated)
// into Demos by directory and name, tagging each with loc. Non-audio files
// (e.g. Ableton's .asd analysis files) are ignored. Results are sorted by
// directory, then name.
func GroupPaths(paths []string, loc Location) []Demo {
	byKey := map[string]*Demo{}
	for _, p := range paths {
		ext := strings.ToLower(path.Ext(p))
		if !audioExts[ext] {
			continue
		}
		dir, base := path.Split(p)
		dir = strings.TrimSuffix(dir, "/")
		name := strings.TrimSuffix(base, path.Ext(base))
		key := dir + "\x00" + name
		d, ok := byKey[key]
		if !ok {
			d = &Demo{Name: name, Dir: dir, Location: loc}
			byKey[key] = d
		}
		d.Formats = append(d.Formats, strings.TrimPrefix(ext, "."))
	}
	return sortedDemos(byKey)
}

func sortedDemos(byKey map[string]*Demo) []Demo {
	demos := make([]Demo, 0, len(byKey))
	for _, d := range byKey {
		sort.Strings(d.Formats)
		demos = append(demos, *d)
	}
	sort.Slice(demos, func(i, j int) bool {
		if demos[i].Dir != demos[j].Dir {
			return demos[i].Dir < demos[j].Dir
		}
		return demos[i].Name < demos[j].Name
	})
	return demos
}

// ListDemos finds audio files under dir recursively and groups them into
// local Demos.
func ListDemos(dir string) ([]Demo, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", dir, err)
	}
	return GroupPaths(paths, Local), nil
}

// Merge combines local and remote Demos into one list. A Demo is the same
// on both sides when its relative directory and name match (extensions may
// differ: a local .wav and a remote .mp3 are one Demo). Formats are unioned.
func Merge(local, remote []Demo) []Demo {
	byKey := map[string]*Demo{}
	add := func(src []Demo, loc Location) {
		for _, d := range src {
			key := d.Dir + "\x00" + d.Name
			m, ok := byKey[key]
			if !ok {
				c := d
				c.Formats = append([]string(nil), d.Formats...)
				c.Location = loc
				byKey[key] = &c
				continue
			}
			m.Location = Both
			for _, f := range d.Formats {
				if !contains(m.Formats, f) {
					m.Formats = append(m.Formats, f)
				}
			}
		}
	}
	add(local, Local)
	add(remote, Remote)
	return sortedDemos(byKey)
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// Resolve links each Demo to the Versions in projects. Matching is exact
// and case-sensitive; there is no prefix guessing. Output order follows demos.
func Resolve(demos []Demo, projects []discovery.Project) []Link {
	index := map[string][]Version{}
	for _, p := range projects {
		for _, als := range p.AlsFiles {
			slug := strings.TrimSuffix(filepath.Base(als), filepath.Ext(als))
			index[slug] = append(index[slug], Version{
				Slug: slug, Path: als, ProjectName: p.Name, ProjectPath: p.Path,
			})
		}
	}

	links := make([]Link, 0, len(demos))
	for _, d := range demos {
		slug, _, hasSep := Parse(d.Name)
		l := Link{Demo: d}
		matches := index[slug]
		switch {
		case slug == "":
			l.Status = Unlinked
		case len(matches) == 1:
			l.Status = Linked
			v := matches[0]
			l.Version = &v
		case len(matches) > 1:
			l.Status = Ambiguous
			l.Candidates = matches
		case hasSep:
			l.Status = Dangling
		default:
			l.Status = Unlinked
		}
		links = append(links, l)
	}
	return links
}

// ForProject keeps the links that point at projectPath: Linked ones whose
// Version is in it, and Ambiguous ones where it is a candidate.
func ForProject(links []Link, projectPath string) []Link {
	var out []Link
	for _, l := range links {
		switch {
		case l.Version != nil && l.Version.ProjectPath == projectPath:
			out = append(out, l)
		case l.Status == Ambiguous:
			for _, c := range l.Candidates {
				if c.ProjectPath == projectPath {
					out = append(out, l)
					break
				}
			}
		}
	}
	return out
}

// CountByProject counts Linked Demos per Project path. Ambiguous, dangling
// and unlinked Demos are not counted.
func CountByProject(links []Link) map[string]int {
	counts := map[string]int{}
	for _, l := range links {
		if l.Status == Linked {
			counts[l.Version.ProjectPath]++
		}
	}
	return counts
}
