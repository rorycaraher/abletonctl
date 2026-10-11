// Package discovery finds Ableton project folders on disk using a
// structural convention (a top-level .als file) rather than a naming
// exclude-list.
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Project is an Ableton project folder: a directory with one or more
// top-level .als files.
type Project struct {
	// Name is the folder name without the " Project" suffix Live appends
	// to project folders.
	Name string
	Path string
	// AlsFiles are absolute paths to the top-level .als files in this
	// project (Ableton's auto-generated Backup/ subfolder is not scanned).
	AlsFiles []string
}

const folderSuffix = " Project"

// ProjectName strips the " Project" suffix Live appends to project folder
// names. A folder named exactly " Project" is left as is.
func ProjectName(folder string) string {
	if n := strings.TrimSuffix(folder, folderSuffix); n != "" {
		return n
	}
	return folder
}

// FindProject returns the project called name. The " Project" suffix is
// optional in name. Two folders that strip to the same name are an error.
func FindProject(projects []Project, name string) (Project, error) {
	want := ProjectName(name)
	var found []Project
	for _, p := range projects {
		if p.Name == want {
			found = append(found, p)
		}
	}
	switch len(found) {
	case 0:
		return Project{}, fmt.Errorf("no project named %q", want)
	case 1:
		return found[0], nil
	}
	paths := make([]string, len(found))
	for i, p := range found {
		paths[i] = p.Path
	}
	return Project{}, fmt.Errorf("project name %q is ambiguous: %s", want, strings.Join(paths, ", "))
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
			Name:     ProjectName(e.Name()),
			Path:     projectPath,
			AlsFiles: alsFiles,
		})
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Name != projects[j].Name {
			return projects[i].Name < projects[j].Name
		}
		return projects[i].Path < projects[j].Path
	})
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
