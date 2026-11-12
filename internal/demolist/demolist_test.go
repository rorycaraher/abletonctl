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
		{Name: "song-01__a", Dir: "", Formats: []string{"mp3", "wav"}, Location: Local},
		{Name: "other", Dir: "sub", Formats: []string{"aif"}, Location: Local},
		{Name: "song-01__a", Dir: "sub", Formats: []string{"wav"}, Location: Local},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestGroupPaths(t *testing.T) {
	got := GroupPaths([]string{
		"a.mp3", "a.wav", "a.wav.asd", "notes.txt",
		"sub/a.mp3", "sub/deep/b.AIFF", "README",
	}, Remote)
	want := []Demo{
		{Name: "a", Dir: "", Formats: []string{"mp3", "wav"}, Location: Remote},
		{Name: "a", Dir: "sub", Formats: []string{"mp3"}, Location: Remote},
		{Name: "b", Dir: "sub/deep", Formats: []string{"aiff"}, Location: Remote},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestMerge(t *testing.T) {
	local := GroupPaths([]string{"both.wav", "only-local.wav", "sub/x.wav", "x.wav"}, Local)
	remote := GroupPaths([]string{"both.mp3", "only-remote.mp3", "sub/x.mp3", "sub/y.mp3"}, Remote)
	got := Merge(local, remote)
	want := []Demo{
		{Name: "both", Formats: []string{"wav", "mp3"}, Location: Both},
		{Name: "only-local", Formats: []string{"wav"}, Location: Local},
		{Name: "only-remote", Formats: []string{"mp3"}, Location: Remote},
		{Name: "x", Formats: []string{"wav"}, Location: Local}, // root x, distinct from sub/x
		{Name: "x", Dir: "sub", Formats: []string{"wav", "mp3"}, Location: Both},
		{Name: "y", Dir: "sub", Formats: []string{"mp3"}, Location: Remote},
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
}

func TestListDemosMissingDir(t *testing.T) {
	if _, err := ListDemos(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("want error")
	}
}
