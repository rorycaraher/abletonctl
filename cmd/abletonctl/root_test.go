package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/demolink"
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

func TestProjectLine(t *testing.T) {
	p := discovery.Project{Name: "song Project", AlsFiles: []string{"a.als", "b.als"}}
	if got, want := projectLine(p, nil), "  - song Project (2 .als)"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	one, many := 1, 3
	if got, want := projectLine(p, &one), "  - song Project (2 .als, 1 demo)"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if got, want := projectLine(p, &many), "  - song Project (2 .als, 3 demos)"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestWriteDemoLinks(t *testing.T) {
	links := []demolink.Link{
		{Demo: demolink.Demo{Name: "song-01__x", Formats: []string{"mp3", "wav"}, Location: demolink.Both},
			Status: demolink.Linked, Version: &demolink.Version{Slug: "song-01", ProjectName: "song Project"}},
		{Demo: demolink.Demo{Name: "Untitled", Dir: "sub", Formats: []string{"wav"}, Location: demolink.Local},
			Status: demolink.Ambiguous, Candidates: []demolink.Version{{ProjectName: "a Project"}, {ProjectName: "b Project"}}},
		{Demo: demolink.Demo{Name: "gone__x", Formats: []string{"mp3"}, Location: demolink.Remote}, Status: demolink.Dangling},
		{Demo: demolink.Demo{Name: "plain", Formats: []string{"wav"}, Location: demolink.Local}, Status: demolink.Unlinked},
	}

	var local bytes.Buffer
	writeDemoLinks(&local, links, false)
	for _, want := range []string{
		"song-01__x [mp3,wav]  -> song Project / song-01.als",
		"sub/Untitled [wav]  -> matches 2 Projects: a Project, b Project",
		"dangling  gone__x",
		"4 demos: 1 linked, 1 ambiguous, 1 dangling, 1 unlinked",
	} {
		if !strings.Contains(local.String(), want) {
			t.Errorf("missing %q in:\n%s", want, local.String())
		}
	}
	if strings.Contains(local.String(), "remote") {
		t.Errorf("no location info without --remote:\n%s", local.String())
	}

	var remote bytes.Buffer
	writeDemoLinks(&remote, links, true)
	for _, want := range []string{
		"linked    both   song-01__x [mp3,wav]",
		"dangling  remote gone__x [mp3]",
		"2 local, 1 remote, 1 both (by name)",
	} {
		if !strings.Contains(remote.String(), want) {
			t.Errorf("missing %q in:\n%s", want, remote.String())
		}
	}
}

func TestRemoteFlagAndMissingRemote(t *testing.T) {
	for _, name := range []string{"demos", "projects"} {
		sub, _, err := newRootCmd().Find([]string{name})
		if err != nil || sub.Flags().Lookup("remote") == nil {
			t.Errorf("%s: missing --remote flag (%v)", name, err)
		}
	}

	dir := t.TempDir()
	proj := filepath.Join(dir, "projects", "Song Project")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "song-01.als"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	demosDir := filepath.Join(dir, "demos")
	if err := os.MkdirAll(demosDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfg, []byte("projects_dir = \""+filepath.Join(dir, "projects")+"\"\ndemos_dir = \""+demosDir+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"demos", "projects"} {
		_, err := run(t, name, "--remote", "--config", cfg)
		if err == nil || !strings.Contains(err.Error(), "demos_remote is not configured") {
			t.Errorf("%s --remote without demos_remote: got %v", name, err)
		}
	}
}

func TestDemosArgValidation(t *testing.T) {
	if _, err := run(t, "demos", "extra"); err == nil {
		t.Error("want arg error")
	}
}
