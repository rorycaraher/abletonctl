package versions

import (
	"strings"
	"testing"
)

func TestLatest(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  string
		err   string
	}{
		{"simple", []string{"/p/song-01.als", "/p/song-10.als", "/p/song-02.als"}, "/p/song-10.als", ""},
		{"numeric not lexical", []string{"/p/song-9.als", "/p/song-10.als"}, "/p/song-10.als", ""},
		{"descriptor ignored", []string{"/p/song-03__alt.als", "/p/song-02.als"}, "/p/song-03__alt.als", ""},
		{"descriptor digits ignored", []string{"/p/song-02__take-99.als", "/p/song-03.als"}, "/p/song-03.als", ""},
		{"nested slugs", []string{"/p/bell-04.als", "/p/bell-04-2026.als", "/p/bell-04-2026-02.als", "/p/bell-04-2026-01.als"}, "/p/bell-04-2026-02.als", ""},
		{"no number ignored", []string{"/p/Untitled.als", "/p/song-01.als"}, "/p/song-01.als", ""},
		{"none numbered", []string{"/p/Untitled.als"}, "", "no .als file with a version number"},
		{"tie", []string{"/p/song-03.als", "/p/song-03__alt.als", "/p/song-02.als"}, "", "ambiguous"},
		{"leading zeros equal", []string{"/p/song-04.als", "/p/song-4.als"}, "", "ambiguous"},
		{"case of mtime irrelevant", []string{"/p/a-1.als"}, "/p/a-1.als", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Latest(c.files)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("err = %v, want containing %q", err, c.err)
				}
				return
			}
			if err != nil || got != c.want {
				t.Errorf("got %q, %v; want %q", got, err, c.want)
			}
		})
	}
}
