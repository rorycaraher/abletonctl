// Package demolist finds Demos (rendered audio in the Demos directory) and
// where each one is found: locally and on which Remotes. A Demo is
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

// Local is the place name for the Demos directory itself; a Remote's place
// name is its rclone destination.
const Local = "local"

// Demo is one rendered piece of audio, grouped by directory and name so a
// .wav and its .mp3 are a single Demo.
type Demo struct {
	Name    string   // file name without extension
	Dir     string   // directory relative to the demos root, slash-separated; "" at the root
	Formats []string // lowercase extensions without the dot, sorted
	// Location is the places the Demo was found, in the order they were
	// merged. Decided by name only, never by comparing content, so being
	// found in two places does not mean the files are identical.
	Location []string
}

// In reports whether the Demo was found at place.
func (d Demo) In(place string) bool {
	return contains(d.Location, place)
}

var audioExts = map[string]bool{".wav": true, ".aiff": true, ".aif": true, ".mp3": true}

// GroupPaths groups file paths (relative to a demos root, slash-separated)
// into Demos by directory and name, found at place. Non-audio files
// (e.g. Ableton's .asd analysis files) are ignored. Results are sorted by
// directory, then name.
func GroupPaths(paths []string, place string) []Demo {
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
			d = &Demo{Name: name, Dir: dir, Location: []string{place}}
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

// Merge combines Demo lists from several places into one. A Demo is the
// same in two places when its relative directory and name match
// (extensions may differ: a local .wav and a remote .mp3 are one Demo).
// Formats and Locations are unioned, Locations in argument order.
func Merge(lists ...[]Demo) []Demo {
	byKey := map[string]*Demo{}
	for _, list := range lists {
		for _, d := range list {
			key := d.Dir + "\x00" + d.Name
			m, ok := byKey[key]
			if !ok {
				c := d
				c.Formats = append([]string(nil), d.Formats...)
				c.Location = append([]string(nil), d.Location...)
				byKey[key] = &c
				continue
			}
			for _, f := range d.Formats {
				if !contains(m.Formats, f) {
					m.Formats = append(m.Formats, f)
				}
			}
			for _, l := range d.Location {
				if !contains(m.Location, l) {
					m.Location = append(m.Location, l)
				}
			}
		}
	}
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
