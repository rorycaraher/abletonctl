package demolink

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/rorycaraher/abletonctl/internal/discovery"
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

// project builds a Project folder with the given Version slugs and
// returns it via discovery, as the real command does.
func projects(t *testing.T, layout map[string][]string) []discovery.Project {
	t.Helper()
	root := t.TempDir()
	for name, slugs := range layout {
		for _, s := range slugs {
			touch(t, filepath.Join(root, name, s+".als"))
		}
	}
	ps, err := discovery.DiscoverProjects(root)
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		in, slug, desc string
		hasSep         bool
	}{
		{"bell-04", "bell-04", "", false},
		{"bell-04__soft-pad", "bell-04", "soft-pad", true},
		{"bell-04__", "bell-04", "", true},
		{"a__b__c", "a", "b__c", true},
		{"__x", "", "x", true},
		{"has-single_underscore", "has-single_underscore", "", false},
	} {
		slug, desc, sep := Parse(tc.in)
		if slug != tc.slug || desc != tc.desc || sep != tc.hasSep {
			t.Errorf("Parse(%q) = %q,%q,%v", tc.in, slug, desc, sep)
		}
	}
}

func demo(name string) Demo { return Demo{Name: name, Dir: "/d", Formats: []string{"wav"}} }

func statuses(links []Link) map[string]Status {
	m := map[string]Status{}
	for _, l := range links {
		m[l.Demo.Name] = l.Status
	}
	return m
}

func TestResolve(t *testing.T) {
	ps := projects(t, map[string][]string{
		"bell-04 Project":    {"bell-04", "bell-04-2026", "bell-04-2026-02"},
		"rumble Project":     {"rumble-pad-01", "Untitled"},
		"scratch Project":    {"Untitled"},
		"ambassador Project": {"briefcase-04"},
	})
	got := statuses(Resolve([]Demo{
		demo("bell-04-2026-02__soft-pad"), // exact slug, not a bell-04 prefix guess
		demo("bell-04-2026"),              // bare slug
		demo("bell-04"),
		demo("bell-04-2026-soft"),    // no sep, no exact match
		demo("Untitled__idea"),       // slug in two Projects
		demo("nonexistent__x"),       // sep but no Version
		demo("not-two-DEMO-MASTER"),  // plain human name
		demo("__x"),                  // empty slug
		demo("Bell-04"),              // case-sensitive
		demo("briefcase-04__glassy"), // Version in a differently named Project
	}, ps))
	want := map[string]Status{
		"bell-04-2026-02__soft-pad": Linked,
		"bell-04-2026":              Linked,
		"bell-04":                   Linked,
		"bell-04-2026-soft":         Unlinked,
		"Untitled__idea":            Ambiguous,
		"nonexistent__x":            Dangling,
		"not-two-DEMO-MASTER":       Unlinked,
		"__x":                       Unlinked,
		"Bell-04":                   Unlinked,
		"briefcase-04__glassy":      Linked,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestResolveLinkedVersionAndAmbiguousCandidates(t *testing.T) {
	ps := projects(t, map[string][]string{
		"a Project": {"Untitled", "song-01"},
		"b Project": {"Untitled"},
	})
	links := Resolve([]Demo{demo("song-01__x"), demo("Untitled")}, ps)
	if v := links[0].Version; v == nil || v.ProjectName != "a Project" || v.Slug != "song-01" {
		t.Fatalf("linked version = %+v", v)
	}
	if n := len(links[1].Candidates); links[1].Status != Ambiguous || n != 2 {
		t.Fatalf("ambiguous = %+v", links[1])
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

func TestMergeKeepsLocationInResolve(t *testing.T) {
	ps := projects(t, map[string][]string{"a Project": {"song-01"}})
	merged := Merge(
		GroupPaths([]string{"song-01__x.wav"}, Local),
		GroupPaths([]string{"song-01__x.mp3", "gone__y.mp3"}, Remote),
	)
	links := Resolve(merged, ps)
	got := map[string]Location{}
	for _, l := range links {
		got[l.Demo.Name] = l.Demo.Location
	}
	if got["song-01__x"] != Both || got["gone__y"] != Remote {
		t.Fatalf("locations = %v", got)
	}
	for _, l := range links {
		if l.Demo.Name == "gone__y" && l.Status != Dangling {
			t.Fatalf("gone__y status = %s, want dangling", l.Status)
		}
	}
}

func TestListDemosMissingDir(t *testing.T) {
	if _, err := ListDemos(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("want error")
	}
}

func TestForProjectAndCount(t *testing.T) {
	ps := projects(t, map[string][]string{
		"a Project": {"Untitled", "song-01"},
		"b Project": {"Untitled", "song-02"},
	})
	links := Resolve([]Demo{
		demo("song-01__x"), demo("song-01"), demo("song-02__y"),
		demo("Untitled"), demo("zzz__q"), demo("plain"),
	}, ps)

	var a, b string
	for _, p := range ps {
		if p.Name == "a Project" {
			a = p.Path
		} else {
			b = p.Path
		}
	}
	counts := CountByProject(links)
	if counts[a] != 2 || counts[b] != 1 {
		t.Fatalf("counts = %v", counts)
	}
	// a: its two song-01 Demos plus the ambiguous Untitled (candidate in a).
	if n := len(ForProject(links, a)); n != 3 {
		t.Fatalf("ForProject(a) = %d, want 3", n)
	}
	if n := len(ForProject(links, b)); n != 2 {
		t.Fatalf("ForProject(b) = %d, want 2", n)
	}
}
