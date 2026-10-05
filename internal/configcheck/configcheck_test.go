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

func TestHealthyConfig(t *testing.T) {
	proj, demos := t.TempDir(), t.TempDir()
	p := writeConfig(t, `
projects_dir = "`+proj+`"
demos_dir = "`+demos+`"
projects_remote = "r2:bucket/projects"
demos_remote = "gdrive:demos"
`)
	rs := Check(p, fakeEnv([]string{"rclone", "ffmpeg"}, []string{"r2", "gdrive"}))
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

func TestUnknownKey(t *testing.T) {
	proj := t.TempDir()
	p := writeConfig(t, `projects_dir = "`+proj+"\"\nproject_remote = \"r2:x\"\n")
	rs := Check(p, fakeEnv(nil, nil))
	if !has(rs, Fail, "project_remote") {
		t.Fatalf("got %+v", rs)
	}
}

func TestBadDirs(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, "projects_dir = \"~/Music/projects\"\ndemos_dir = \""+file+"\"\n")
	rs := Check(p, fakeEnv(nil, nil))
	if !has(rs, Fail, "~ is not expanded") || !has(rs, Fail, "not a directory") {
		t.Fatalf("got %+v", rs)
	}
}

func TestProjectsDirRequiredDemosOptional(t *testing.T) {
	rs := Check(writeConfig(t, ""), fakeEnv(nil, nil))
	if !has(rs, Fail, "projects_dir is not set") || !has(rs, Warn, "demos_dir is not set") {
		t.Fatalf("got %+v", rs)
	}
}

func TestRemoteProblems(t *testing.T) {
	proj, demos := t.TempDir(), t.TempDir()
	p := writeConfig(t, `
projects_dir = "`+proj+`"
projects_remote = "nocolon"
demos_dir = "`+demos+`"
`)
	rs := Check(p, fakeEnv([]string{"rclone"}, nil))
	if !has(rs, Fail, `has no "name:" prefix`) || !has(rs, Warn, "must both be set") {
		t.Fatalf("got %+v", rs)
	}
}

func TestUnknownRemoteAndMissingBinaries(t *testing.T) {
	proj, demos := t.TempDir(), t.TempDir()
	p := writeConfig(t, `
projects_dir = "`+proj+`"
projects_remote = "r2:bucket"
demos_dir = "`+demos+`"
demos_remote = "gdrive:demos"
`)
	rs := Check(p, fakeEnv([]string{"rclone"}, []string{"r2"}))
	if !has(rs, Fail, `"gdrive"`) || !has(rs, Warn, "ffmpeg") || has(rs, Fail, `"r2"`) {
		t.Fatalf("got %+v", rs)
	}
	rs = Check(p, fakeEnv(nil, nil))
	if !has(rs, Fail, "rclone not found") {
		t.Fatalf("got %+v", rs)
	}
}
