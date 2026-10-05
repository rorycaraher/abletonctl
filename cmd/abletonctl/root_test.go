package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
