// Package discovery finds role directories (e.g. PRODUCTION-*) and Ableton
// project folders on disk using structural conventions rather than naming
// exclude-lists.
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProductionDirGlob is the naming convention for a year's production
// directory. All directories matching it are treated as equally active —
// there is no year-based tiering.
const ProductionDirGlob = "PRODUCTION-*"

// DiscoverProductionDirs returns every PRODUCTION-* directory directly under
// an artist root, sorted.
func DiscoverProductionDirs(artistRoot string) ([]string, error) {
	return MatchRoleDirs(artistRoot, ProductionDirGlob)
}

// MatchRoleDirs returns absolute paths of direct child directories of root
// whose basename matches glob (a filepath.Match-style pattern), sorted.
func MatchRoleDirs(root, glob string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", root, err)
	}
	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ok, err := filepath.Match(glob, e.Name())
		if err != nil {
			return nil, fmt.Errorf("bad glob %q: %w", glob, err)
		}
		if ok {
			matches = append(matches, filepath.Join(root, e.Name()))
		}
	}
	sort.Strings(matches)
	return matches, nil
}

// Project is an Ableton project folder: a directory with one or more
// top-level .als files.
type Project struct {
	Name string
	Path string
	// AlsFiles are absolute paths to the top-level .als files in this
	// project (Ableton's auto-generated Backup/ subfolder is not scanned).
	AlsFiles []string
}

// SamplesDir returns the project's Samples directory path, which may not exist.
func (p Project) SamplesDir() string {
	return filepath.Join(p.Path, "Samples")
}

// DiscoverProjects finds project folders directly under dir: any child
// directory containing a top-level .als file. Children without one (e.g.
// reference material or sample-pack folders) are skipped.
func DiscoverProjects(dir string) ([]Project, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	var projects []Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		projectPath := filepath.Join(dir, e.Name())
		alsFiles, err := topLevelAlsFiles(projectPath)
		if err != nil {
			return nil, err
		}
		if len(alsFiles) == 0 {
			continue
		}
		projects = append(projects, Project{
			Name:     e.Name(),
			Path:     projectPath,
			AlsFiles: alsFiles,
		})
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, nil
}

func topLevelAlsFiles(projectPath string) ([]string, error) {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", projectPath, err)
	}
	var als []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".als") {
			als = append(als, filepath.Join(projectPath, e.Name()))
		}
	}
	sort.Strings(als)
	return als, nil
}
