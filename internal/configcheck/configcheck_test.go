package configcheck

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func fakeEnv(bins []string, remotes []string) Env {
	return Env{
		LookPath: func(name string) (string, error) {
			for _, b := range bins {
				if b == name {
					return "/bin/" + name, nil
				}
			}
			return "", errors.New("not found")
		},
		RcloneRemotes: func() ([]string, error) { return remotes, nil },
	}
}

func has(rs []Result, l Level, substr string) bool {
	for _, r := range rs {
		if r.Level == l && strings.Contains(r.Message, substr) {
			return true
		}
	}
	return false
}

// dir renders one [dirs.<name>] entry.
func dir(name, path string, remotes ...string) string {
	q := make([]string, len(remotes))
	for i, r := range remotes {
		q[i] = `"` + r + `"`
	}
	return "[dirs." + name + "]\npath = \"" + path + "\"\nremotes = [" + strings.Join(q, ", ") + "]\n"
}

func TestHealthyConfig(t *testing.T) {
	p := writeConfig(t,
		dir("projects", t.TempDir(), "r2:bucket/projects", "nas:music/projects")+
			dir("demos", t.TempDir(), "gdrive:demos")+
			dir("user-library", t.TempDir(), "r2:bucket/user-library"))
	rs := Check(p, fakeEnv([]string{"rclone", "ffmpeg"}, []string{"r2", "gdrive", "nas"}))
	if Failed(rs) {
		t.Fatalf("unexpected failure: %+v", rs)
	}
	if has(rs, Warn, "") {
		t.Fatalf("unexpected warning: %+v", rs)
	}
}

func TestMissingFile(t *testing.T) {
	rs := Check(filepath.Join(t.TempDir(), "nope.toml"), fakeEnv(nil, nil))
	if !Failed(rs) || !has(rs, Fail, "cannot load") {
		t.Fatalf("got %+v", rs)
	}
}

func TestUnknownKeysAtBothLevels(t *testing.T) {
	p := writeConfig(t, "projects_dir = \"/old\"\n"+dir("demos", t.TempDir())+"remote = \"r2:x\"\n")
	rs := Check(p, fakeEnv(nil, nil))
	if !has(rs, Fail, "projects_dir") || !has(rs, Fail, "dirs.demos.remote") {
		t.Fatalf("got %+v", rs)
	}
}

func TestNoDirectoriesFails(t *testing.T) {
	rs := Check(writeConfig(t, ""), fakeEnv(nil, nil))
	if !has(rs, Fail, "no Directories configured") {
		t.Fatalf("got %+v", rs)
	}
}

func TestReservedDirectoriesAreOptional(t *testing.T) {
	rs := Check(writeConfig(t, dir("samples", t.TempDir(), "b2:samples")), fakeEnv([]string{"rclone"}, []string{"b2"}))
	if Failed(rs) {
		t.Fatalf("unexpected failure: %+v", rs)
	}
	if !has(rs, Warn, "no [dirs.projects]") || !has(rs, Warn, "no [dirs.demos]: demos and convert-demos") {
		t.Fatalf("got %+v", rs)
	}
}

func TestDirectoryWithoutRemotes(t *testing.T) {
	p := writeConfig(t, dir("projects", t.TempDir())+dir("demos", t.TempDir())+dir("samples", t.TempDir()))
	rs := Check(p, fakeEnv([]string{"ffmpeg"}, nil))
	if !has(rs, Warn, "dirs.samples has no remotes") {
		t.Fatalf("got %+v", rs)
	}
	// projects and demos are used by commands even without Remotes.
	if has(rs, Warn, "dirs.projects has no remotes") || has(rs, Warn, "dirs.demos has no remotes") {
		t.Fatalf("got %+v", rs)
	}
}

func TestBadDirs(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir("projects", filepath.Join(t.TempDir(), "missing"))+
		dir("demos", file)+
		"[dirs.samples]\nremotes = [\"b2:s\"]\n")
	rs := Check(p, fakeEnv(nil, nil))
	if !has(rs, Fail, "dirs.projects: stat") || !has(rs, Fail, "not a directory") || !has(rs, Fail, "dirs.samples: path is not set") {
		t.Fatalf("got %+v", rs)
	}
}

func TestRemoteWithoutName(t *testing.T) {
	rs := Check(writeConfig(t, dir("projects", t.TempDir(), "nocolon")), fakeEnv([]string{"rclone"}, nil))
	if !has(rs, Fail, `has no "name:" prefix`) {
		t.Fatalf("got %+v", rs)
	}
}

func TestUnknownRemoteAndMissingBinaries(t *testing.T) {
	p := writeConfig(t, dir("projects", t.TempDir(), "r2:bucket")+dir("demos", t.TempDir(), "gdrive:demos"))
	rs := Check(p, fakeEnv([]string{"rclone"}, []string{"r2"}))
	if !has(rs, Fail, `"gdrive"`) || !has(rs, Warn, "ffmpeg") || has(rs, Fail, `"r2"`) {
		t.Fatalf("got %+v", rs)
	}
	rs = Check(p, fakeEnv(nil, nil))
	if !has(rs, Fail, "rclone not found") {
		t.Fatalf("got %+v", rs)
	}
}

func TestSharedRemoteFails(t *testing.T) {
	p := writeConfig(t, dir("demos", t.TempDir(), "r2:bucket/stuff")+dir("user-library", t.TempDir(), "r2:bucket/stuff/"))
	rs := Check(p, fakeEnv([]string{"rclone", "ffmpeg"}, []string{"r2"}))
	if !has(rs, Fail, "dirs.demos and dirs.user-library share the remote") {
		t.Fatalf("got %+v", rs)
	}
}

func TestNestedRemotesWarn(t *testing.T) {
	p := writeConfig(t,
		dir("projects", t.TempDir(), "r2:nltl-backup")+
			dir("user-library", t.TempDir(), "r2:nltl-backup/Ableton")+
			dir("samples", t.TempDir(), "b2:")+
			dir("loops", t.TempDir(), "b2:loops")+
			dir("other", t.TempDir(), "r2:nltl-backup-2")) // shares a prefix, not a folder
	rs := Check(p, fakeEnv([]string{"rclone"}, []string{"r2", "b2"}))
	if Failed(rs) {
		t.Fatalf("unexpected failure: %+v", rs)
	}
	if !has(rs, Warn, `dirs.user-library's remote "r2:nltl-backup/Ableton" is inside dirs.projects's`) ||
		!has(rs, Warn, `dirs.loops's remote "b2:loops" is inside dirs.samples's`) ||
		has(rs, Warn, "dirs.other's remote") {
		t.Fatalf("got %+v", rs)
	}
}

func TestNestedPathsWarn(t *testing.T) {
	root := t.TempDir()
	demos := filepath.Join(root, "demos")
	sibling := filepath.Join(root, "demos-old")
	for _, d := range []string{demos, sibling} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p := writeConfig(t, dir("demos", demos, "gdrive:demos")+dir("nltl", root, "r2:nltl")+dir("old", sibling, "r2:old"))
	rs := Check(p, fakeEnv([]string{"rclone", "ffmpeg"}, []string{"r2", "gdrive"}))
	if !has(rs, Warn, "dirs.demos ("+demos+") is inside dirs.nltl") {
		t.Fatalf("got %+v", rs)
	}
	if has(rs, Warn, "dirs.old ("+sibling+") is inside dirs.demos") {
		t.Fatalf("sibling with a shared prefix is not nested: %+v", rs)
	}
}
