package demolist

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListDemos(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "song-01__a.wav"))
	touch(t, filepath.Join(dir, "song-01__a.mp3"))
	touch(t, filepath.Join(dir, "song-01__a.wav.asd")) // Ableton analysis file: ignored
	touch(t, filepath.Join(dir, "notes.txt"))          // not audio
	touch(t, filepath.Join(dir, "sub", "song-01__a.WAV"))
	touch(t, filepath.Join(dir, "sub", "other.aif"))

	got, err := ListDemos(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Demo{
		{Name: "song-01__a", Dir: "", Formats: []string{"mp3", "wav"}, Location: []string{Local}},
		{Name: "other", Dir: "sub", Formats: []string{"aif"}, Location: []string{Local}},
		{Name: "song-01__a", Dir: "sub", Formats: []string{"wav"}, Location: []string{Local}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestGroupPaths(t *testing.T) {
	got := GroupPaths([]string{
		"a.mp3", "a.wav", "a.wav.asd", "notes.txt",
		"sub/a.mp3", "sub/deep/b.AIFF", "README",
	}, "r2:demos")
	want := []Demo{
		{Name: "a", Dir: "", Formats: []string{"mp3", "wav"}, Location: []string{"r2:demos"}},
		{Name: "a", Dir: "sub", Formats: []string{"mp3"}, Location: []string{"r2:demos"}},
		{Name: "b", Dir: "sub/deep", Formats: []string{"aiff"}, Location: []string{"r2:demos"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestMerge(t *testing.T) {
	local := GroupPaths([]string{"everywhere.wav", "only-local.wav", "sub/x.wav", "x.wav"}, Local)
	r2 := GroupPaths([]string{"everywhere.mp3", "only-r2.mp3", "sub/x.mp3", "sub/y.mp3"}, "r2:demos")
	gdrive := GroupPaths([]string{"everywhere.mp3", "sub/y.mp3"}, "gdrive:demos")
	got := Merge(local, r2, gdrive)
	want := []Demo{
		{Name: "everywhere", Formats: []string{"wav", "mp3"}, Location: []string{Local, "r2:demos", "gdrive:demos"}},
		{Name: "only-local", Formats: []string{"wav"}, Location: []string{Local}},
		{Name: "only-r2", Formats: []string{"mp3"}, Location: []string{"r2:demos"}},
		{Name: "x", Formats: []string{"wav"}, Location: []string{Local}}, // root x, distinct from sub/x
		{Name: "x", Dir: "sub", Formats: []string{"wav", "mp3"}, Location: []string{Local, "r2:demos"}},
		{Name: "y", Dir: "sub", Formats: []string{"mp3"}, Location: []string{"r2:demos", "gdrive:demos"}},
	}
	// Order is by directory then name; compare ignoring format order.
	if len(got) != len(want) {
		t.Fatalf("got %d demos: %+v", len(got), got)
	}
	for i := range want {
		g, w := got[i], want[i]
		sort.Strings(g.Formats)
		sort.Strings(w.Formats)
		if !reflect.DeepEqual(g, w) {
			t.Errorf("[%d] got %+v want %+v", i, g, w)
		}
	}
	if !got[0].In("gdrive:demos") || got[1].In("r2:demos") {
		t.Errorf("In: %+v", got[:2])
	}
}

func TestListDemosMissingDir(t *testing.T) {
	if _, err := ListDemos(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("want error")
	}
}
