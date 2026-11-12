// Package demolist finds Demos (rendered audio in the Demos directory) and
// where each one is found: locally, on a Remote, or both. A Demo is
// identified by its relative directory and name without extension, so a
// .wav and its .mp3 are one Demo.
package demolist

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

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

var audioExts = map[string]bool{".wav": true, ".aiff": true, ".aif": true, ".mp3": true}

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
