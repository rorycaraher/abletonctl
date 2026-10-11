package export

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultOutput(t *testing.T) {
	got := DefaultOutput("/demos", "/p/Bell Project/bell-04.als")
	if want := "/demos/bell-04.wav"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestRunPathsWithSpaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My Project")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	als := filepath.Join(dir, "my set-01.als")
	if err := os.WriteFile(als, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "my set-01.wav")

	var gotArgs []string
	fake := func(_ string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}
	if err := Run(fake, als, out); err != nil {
		t.Fatal(err)
	}
	want := []string{als, dir + "/", "my set-01"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("runner args = %q, want %q", gotArgs, want)
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	als := filepath.Join(dir, "song.als")
	if err := os.WriteFile(als, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "song.wav")

	var gotArgs []string
	fake := func(_ string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}
	if err := Run(fake, als, out); err != nil {
		t.Fatal(err)
	}
	want := []string{als, dir + "/", "song"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("runner args = %q, want %q", gotArgs, want)
	}

	existing := filepath.Join(dir, "taken.wav")
	if err := os.WriteFile(existing, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct{ als, out, want string }{
		"not als":      {filepath.Join(dir, "x.wav"), out, "not an .als"},
		"missing als":  {filepath.Join(dir, "gone.als"), out, "no such file"},
		"relative out": {als, "song.wav", "absolute"},
		"not wav":      {als, filepath.Join(dir, "song.mp3"), ".wav"},
		"exists":       {als, existing, "not overwriting"},
		"no out dir":   {als, filepath.Join(dir, "nope", "a.wav"), "does not exist"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := Run(fake, c.als, c.out)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want containing %q", err, c.want)
			}
		})
	}
}
