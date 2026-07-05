package samples

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rorycaraher/ableton-framework/internal/discovery"
)

func writeSample(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupProject(t *testing.T) discovery.Project {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, "Song")

	writeFixtureAls(t, filepath.Join(projectPath, "Song.als"))
	// Used: exact relative-path match against the fixture's RelativePath.
	writeSample(t, filepath.Join(projectPath, "Samples", "Imported", "kick.wav"), 300)
	// Uncertain: filename matches a reference, but it lives in a different
	// subfolder than the fixture's RelativePath says.
	writeSample(t, filepath.Join(projectPath, "Samples", "Processed", "snare.wav"), 200)
	// Orphan: no reference by path or filename.
	writeSample(t, filepath.Join(projectPath, "Samples", "Imported", "unused_loop.wav"), 100)

	return discovery.Project{
		Name:     "Song",
		Path:     projectPath,
		AlsFiles: []string{filepath.Join(projectPath, "Song.als")},
	}
}

func TestScan_ClassifiesByRelativePathThenFilenameFallback(t *testing.T) {
	project := setupProject(t)

	results, err := Scan(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3: %+v", len(results), results)
	}

	byRel := map[string]Status{}
	for _, r := range results {
		byRel[r.RelPath] = r.Status
	}

	if byRel["Samples/Imported/kick.wav"] != Used {
		t.Errorf("kick.wav: got %v, want Used", byRel["Samples/Imported/kick.wav"])
	}
	if byRel["Samples/Processed/snare.wav"] != Uncertain {
		t.Errorf("snare.wav: got %v, want Uncertain", byRel["Samples/Processed/snare.wav"])
	}
	if byRel["Samples/Imported/unused_loop.wav"] != Orphan {
		t.Errorf("unused_loop.wav: got %v, want Orphan", byRel["Samples/Imported/unused_loop.wav"])
	}
}

func TestScan_SortsBySizeDescending(t *testing.T) {
	project := setupProject(t)
	results, err := Scan(project)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(results); i++ {
		if results[i-1].Size < results[i].Size {
			t.Fatalf("results not sorted by size descending: %+v", results)
		}
	}
}

func TestQuarantine_OnlyMovesOrphans(t *testing.T) {
	project := setupProject(t)
	results, err := Scan(project)
	if err != nil {
		t.Fatal(err)
	}

	moved, err := Quarantine(project, results)
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 1 || moved[0].RelPath != "Samples/Imported/unused_loop.wav" {
		t.Fatalf("expected only unused_loop.wav to be quarantined, got %+v", moved)
	}

	if _, err := os.Stat(filepath.Join(project.Path, "_unreferenced", "Imported", "unused_loop.wav")); err != nil {
		t.Errorf("expected quarantined file at new location: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project.Path, "Samples", "Imported", "unused_loop.wav")); !os.IsNotExist(err) {
		t.Errorf("expected original orphan location to be gone")
	}
	// Used and Uncertain files must be untouched.
	if _, err := os.Stat(filepath.Join(project.Path, "Samples", "Imported", "kick.wav")); err != nil {
		t.Errorf("kick.wav should not have been moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project.Path, "Samples", "Processed", "snare.wav")); err != nil {
		t.Errorf("snare.wav should not have been moved: %v", err)
	}
}
