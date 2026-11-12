package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/backup"
	"github.com/rorycaraher/abletonctl/internal/demolist"
	"github.com/rorycaraher/abletonctl/internal/discovery"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestUnknownCommand(t *testing.T) {
	if _, err := run(t, "bogus"); err == nil {
		t.Fatal("want error for unknown command")
	}
}

func TestArgValidation(t *testing.T) {
	for _, args := range [][]string{
		{"find-orphans"},
		{"find-orphans", "a", "b"},
		{"collect"},
		{"collect", "a", "b"},
		{"projects", "extra"},
		{"demos", "extra"},
		{"config", "check", "extra"},
	} {
		if _, err := run(t, args...); err == nil {
			t.Errorf("%v: want arg error", args)
		}
	}
}

// Flags may come before or after the positional argument; this is the
// reason the old CLI parsed find-orphans/collect by hand.
func TestInterspersedFlags(t *testing.T) {
	for _, tc := range []struct {
		cmd, flag string
	}{{"find-orphans", "quarantine"}, {"collect", "all"}} {
		for _, args := range [][]string{
			{tc.cmd, "dir", "--" + tc.flag},
			{tc.cmd, "--" + tc.flag, "dir"},
		} {
			root := newRootCmd()
			sub, _, err := root.Find([]string{tc.cmd})
			if err != nil {
				t.Fatal(err)
			}
			var gotArg string
			var gotFlag bool
			sub.RunE = func(c *cobra.Command, a []string) error {
				gotArg = a[0]
				gotFlag, _ = c.Flags().GetBool(tc.flag)
				return nil
			}
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatalf("%v: %v", args, err)
			}
			if gotArg != "dir" || !gotFlag {
				t.Errorf("%v: arg=%q flag=%v", args, gotArg, gotFlag)
			}
		}
	}
}

func TestConfigWithoutSubcommandShowsHelp(t *testing.T) {
	out, err := run(t, "config")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "check") {
		t.Fatalf("help should list check, got:\n%s", out)
	}
}

func TestEveryCommandHasDescription(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Short == "" {
			t.Errorf("%s has no Short description", c.CommandPath())
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(newRootCmd())
}

func TestZshCompletionParses(t *testing.T) {
	out, err := run(t, "completion", "zsh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "#compdef abletonctl") {
		t.Fatalf("unexpected completion output:\n%.200s", out)
	}
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh not installed")
	}
	p := filepath.Join(t.TempDir(), "_abletonctl")
	if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command(zsh, "-n", p).CombinedOutput(); err != nil {
		t.Fatalf("zsh -n: %v\n%s", err, b)
	}
}

func TestProjectLines(t *testing.T) {
	ps := []discovery.Project{
		{Name: "a", Path: "/p/a", AlsFiles: make([]string, 2)},
		{Name: "longer name", Path: "/p/l", AlsFiles: make([]string, 12)},
		{Name: "mid", Path: "/p/m", AlsFiles: make([]string, 1)},
	}
	got := projectLines(ps)
	want := []string{
		"  - a             2 .als",
		"  - longer name  12 .als",
		"  - mid           1 .als",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}

}

func TestWriteDemos(t *testing.T) {
	demos := []demolist.Demo{
		{Name: "song-01__x", Formats: []string{"mp3", "wav"}, Location: demolist.Both},
		{Name: "Untitled", Dir: "sub", Formats: []string{"wav"}, Location: demolist.Local},
		{Name: "gone__x", Formats: []string{"mp3"}, Location: demolist.Remote},
	}

	var local bytes.Buffer
	writeDemos(&local, demos, false)
	for _, want := range []string{
		"  song-01__x [mp3,wav]\n",
		"  sub/Untitled [wav]\n",
		"3 demos\n",
	} {
		if !strings.Contains(local.String(), want) {
			t.Errorf("missing %q in:\n%s", want, local.String())
		}
	}
	if strings.Contains(local.String(), "remote") {
		t.Errorf("no location info without --remote:\n%s", local.String())
	}

	var remote bytes.Buffer
	writeDemos(&remote, demos, true)
	for _, want := range []string{
		"  both   song-01__x [mp3,wav]",
		"  remote gone__x [mp3]",
		"1 local, 1 remote, 1 both (by name)",
	} {
		if !strings.Contains(remote.String(), want) {
			t.Errorf("missing %q in:\n%s", want, remote.String())
		}
	}
}

func TestDemosRemoteWithoutRemotes(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfg, []byte("[dirs.demos]\npath = \""+dir+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := run(t, "demos", "--remote", "--config", cfg)
	if err == nil || !strings.Contains(err.Error(), "dirs.demos has no remotes") {
		t.Errorf("demos --remote without remotes: got %v", err)
	}
}

func TestDemosArgValidation(t *testing.T) {
	if _, err := run(t, "demos", "extra"); err == nil {
		t.Error("want arg error")
	}
}

func TestWriteBackupSummary(t *testing.T) {
	results := []backupResult{
		{job: backup.Job{Dir: "projects", RemoteDir: "r2:b/projects"}},
		{job: backup.Job{Dir: "projects", RemoteDir: "nas:music/projects"}, err: errors.New("exit status 1")},
		{job: backup.Job{Dir: "demos", RemoteDir: "gdrive:demos"}},
	}
	var out bytes.Buffer
	err := writeBackupSummary(&out, results)
	if err == nil || err.Error() != "1 of 3 copies failed" {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{
		"  ok    [projects] -> r2:b/projects\n",
		"  FAIL  [projects] -> nas:music/projects: exit status 1\n",
		"  ok    [demos] -> gdrive:demos\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
	if err := writeBackupSummary(&bytes.Buffer{}, results[:1]); err != nil {
		t.Fatalf("all ok: %v", err)
	}
}

func TestBackupCompletesDirectoryNames(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfg, []byte("[dirs.projects]\npath = \"/p\"\n[dirs.demos]\npath = \"/d\"\n[dirs.user-library]\npath = \"/u\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "__complete", "backup", "--config", cfg, "demos", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "projects\nuser-library\n:4\n") {
		t.Fatalf("completion = %q", out)
	}
}

func TestBackupUnknownDirectory(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfg, []byte("[dirs.demos]\npath = \"/d\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := run(t, "backup", "--config", cfg, "--dry-run", "nope")
	if err == nil || !strings.Contains(err.Error(), "no [dirs.nope]") {
		t.Fatalf("got %v", err)
	}
}
