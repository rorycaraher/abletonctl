package samples

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/rorycaraher/ableton-framework/internal/discovery"
)

// Status describes how confidently a Samples/ file is used by a project.
type Status int

const (
	// Used means the file's project-relative path exactly matches a
	// RelativePath reference in the project's .als file(s).
	Used Status = iota
	// Uncertain means the file wasn't matched by relative path, but its
	// filename appears somewhere in the .als references (e.g. the sample
	// was moved within Samples/ after being added to the Live set). Never
	// auto-quarantined.
	Uncertain
	// Orphan means no reference at all was found, by path or filename.
	Orphan
)

// FileResult is one file found under a project's Samples/ directory.
type FileResult struct {
	// RelPath is relative to the project root, e.g. "Samples/Imported/foo.wav".
	RelPath string
	AbsPath string
	Size    int64
	Status  Status
}

// Scan reads every top-level .als file in the project, merges their sample
// references, and classifies every file found under Samples/.
func Scan(project discovery.Project) ([]FileResult, error) {
	refs := newSampleRefs()
	for _, als := range project.AlsFiles {
		fileRefs, err := ExtractSampleRefs(als)
		if err != nil {
			return nil, err
		}
		refs.Merge(fileRefs)
	}

	samplesDir := project.SamplesDir()
	if _, err := os.Stat(samplesDir); os.IsNotExist(err) {
		return nil, nil
	}

	var results []FileResult
	err := filepath.Walk(samplesDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(project.Path, p)
		if err != nil {
			return err
		}
		relPath = normalizeRelPath(relPath)

		status := Orphan
		if refs.RelPaths[relPath] {
			status = Used
		} else if refs.Filenames[filepath.Base(p)] {
			status = Uncertain
		}

		results = append(results, FileResult{
			RelPath: relPath,
			AbsPath: p,
			Size:    info.Size(),
			Status:  status,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", samplesDir, err)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Size > results[j].Size })
	return results, nil
}

// Quarantine moves every Orphan-status result into a project-local
// _unreferenced/ folder, preserving its path relative to Samples/. It never
// touches Uncertain or Used files, and never deletes anything.
func Quarantine(project discovery.Project, results []FileResult) ([]FileResult, error) {
	var moved []FileResult
	for _, r := range results {
		if r.Status != Orphan {
			continue
		}
		relToSamples, err := filepath.Rel(project.SamplesDir(), r.AbsPath)
		if err != nil {
			return moved, err
		}
		dest := filepath.Join(project.Path, "_unreferenced", relToSamples)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return moved, fmt.Errorf("creating quarantine dir: %w", err)
		}
		if err := os.Rename(r.AbsPath, dest); err != nil {
			return moved, fmt.Errorf("moving %s: %w", r.AbsPath, err)
		}
		r.AbsPath = dest
		moved = append(moved, r)
	}
	return moved, nil
}
