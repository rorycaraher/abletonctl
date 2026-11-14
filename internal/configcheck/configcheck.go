// Package configcheck validates the config file and the things
// it points at: directories, rclone remotes, and the external binaries the
// commands shell out to. It reports every problem rather than stopping at
// the first.
package configcheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rorycaraher/studioctl/internal/config"
)

// Level is the severity of a Result.
type Level int

const (
	OK Level = iota
	Warn
	Fail
)

func (l Level) String() string {
	switch l {
	case OK:
		return "ok"
	case Warn:
		return "warn"
	default:
		return "FAIL"
	}
}

// Result is one line of the report.
type Result struct {
	Level   Level
	Message string
}

// Env abstracts the machine-dependent lookups so checks are testable
// without rclone or ffmpeg installed.
type Env struct {
	// LookPath reports whether a binary is on PATH (like exec.LookPath).
	LookPath func(name string) (string, error)
	// RcloneRemotes returns configured rclone remote names, without the
	// trailing colon.
	RcloneRemotes func() ([]string, error)
}

// SystemEnv is the real Env.
func SystemEnv() Env {
	return Env{
		LookPath: exec.LookPath,
		RcloneRemotes: func() ([]string, error) {
			out, err := exec.Command("rclone", "listremotes").Output()
			if err != nil {
				return nil, err
			}
			var names []string
			for _, line := range strings.Split(string(out), "\n") {
				if line = strings.TrimSuffix(strings.TrimSpace(line), ":"); line != "" {
					names = append(names, line)
				}
			}
			return names, nil
		},
	}
}

// Failed reports whether any result is a Fail.
func Failed(rs []Result) bool {
	for _, r := range rs {
		if r.Level == Fail {
			return true
		}
	}
	return false
}

// Check validates the config file at path.
func Check(path string, env Env) []Result {
	var rs []Result
	add := func(l Level, format string, a ...any) {
		rs = append(rs, Result{l, fmt.Sprintf(format, a...)})
	}

	cfg, unknown, err := config.LoadChecked(path)
	if err != nil {
		add(Fail, "cannot load %s: %v", path, err)
		return rs
	}
	add(OK, "config parses: %s", path)

	if len(unknown) > 0 {
		add(Fail, "unknown key(s) (typo?): %s", strings.Join(unknown, ", "))
	}

	names := cfg.Names()
	if len(names) == 0 {
		add(Fail, "no Directories configured; add a [dirs.<name>] entry")
		return rs
	}
	if _, ok := cfg.Dir(config.Projects); !ok {
		add(Warn, "no [dirs.%s]: `projects` is unavailable", config.Projects)
	}
	if _, ok := cfg.Dir(config.Demos); !ok {
		add(Warn, "no [dirs.%s]: demos and convert-demos are unavailable", config.Demos)
	}

	var dirs []config.Directory
	for _, name := range names {
		d, _ := cfg.Dir(name)
		dirs = append(dirs, d)
		checkDir(d, add)
	}
	checkOverlaps(dirs, add)

	var remotes []string
	for _, d := range dirs {
		remotes = append(remotes, d.Remotes...)
	}
	if len(remotes) > 0 {
		checkRclone(remotes, env, add)
	}

	if _, ok := cfg.Dir(config.Demos); ok {
		if _, err := env.LookPath("ffmpeg"); err != nil {
			add(Warn, "ffmpeg not found on PATH (needed for convert-demos)")
		} else {
			add(OK, "ffmpeg found")
		}
	}
	return rs
}

type addFunc func(l Level, format string, a ...any)

// checkDir checks one Directory's path and the form of its Remotes.
func checkDir(d config.Directory, add addFunc) {
	key := "dirs." + d.Name
	if d.Path == "" {
		add(Fail, "%s: path is not set", key)
	} else if info, err := os.Stat(d.Path); err != nil {
		add(Fail, "%s: %v", key, err)
	} else if !info.IsDir() {
		add(Fail, "%s: %s is not a directory", key, d.Path)
	} else {
		add(OK, "%s exists: %s", key, d.Path)
	}

	if len(d.Remotes) == 0 && d.Name != config.Projects && d.Name != config.Demos {
		add(Warn, "%s has no remotes, so nothing uses it", key)
	}
	for _, r := range d.Remotes {
		if !strings.Contains(r, ":") {
			add(Fail, "%s: remote %q has no \"name:\" prefix", key, r)
		}
	}
}

// checkOverlaps catches Directories that back up on top of each other:
// one Remote shared by two Directories mixes both trees together, so it
// fails; nested Remotes or nested paths may be deliberate, so they warn.
func checkOverlaps(dirs []config.Directory, add addFunc) {
	for i, a := range dirs {
		for _, b := range dirs[i+1:] {
			for _, ra := range a.Remotes {
				for _, rb := range b.Remotes {
					switch {
					case cleanRemote(ra) == cleanRemote(rb):
						add(Fail, "dirs.%s and dirs.%s share the remote %q; their backups would mix", a.Name, b.Name, ra)
					case remoteWithin(ra, rb):
						add(Warn, "dirs.%s's remote %q is inside dirs.%s's remote %q", a.Name, ra, b.Name, rb)
					case remoteWithin(rb, ra):
						add(Warn, "dirs.%s's remote %q is inside dirs.%s's remote %q", b.Name, rb, a.Name, ra)
					}
				}
			}
			if a.Path == "" || b.Path == "" {
				continue
			}
			pa, pb := filepath.Clean(a.Path), filepath.Clean(b.Path)
			switch {
			case pa == pb:
				add(Warn, "dirs.%s and dirs.%s have the same path %s", a.Name, b.Name, pa)
			case pathWithin(pa, pb):
				add(Warn, "dirs.%s (%s) is inside dirs.%s (%s), so it is backed up twice", a.Name, pa, b.Name, pb)
			case pathWithin(pb, pa):
				add(Warn, "dirs.%s (%s) is inside dirs.%s (%s), so it is backed up twice", b.Name, pb, a.Name, pa)
			}
		}
	}
}

func cleanRemote(r string) string {
	name, p, ok := strings.Cut(r, ":")
	if !ok {
		return r
	}
	return name + ":" + strings.Trim(p, "/")
}

// remoteWithin reports whether remote a is strictly inside remote b.
func remoteWithin(a, b string) bool {
	a, b = cleanRemote(a), cleanRemote(b)
	if a == b {
		return false
	}
	if strings.HasSuffix(b, ":") {
		return strings.HasPrefix(a, b)
	}
	return strings.HasPrefix(a, b+"/")
}

// pathWithin reports whether path a is strictly inside path b.
func pathWithin(a, b string) bool {
	rel, err := filepath.Rel(b, a)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// checkRclone checks rclone is installed and knows every remote name used.
func checkRclone(remotes []string, env Env, add addFunc) {
	if _, err := env.LookPath("rclone"); err != nil {
		add(Fail, "rclone not found on PATH (needed for backup)")
		return
	}
	add(OK, "rclone found")
	known, err := env.RcloneRemotes()
	if err != nil {
		add(Warn, "could not list rclone remotes: %v", err)
		return
	}
	have := map[string]bool{}
	for _, n := range known {
		have[n] = true
	}
	seen := map[string]bool{}
	for _, r := range remotes {
		name, _, ok := strings.Cut(r, ":")
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		if have[name] {
			add(OK, "rclone remote %q exists", name)
		} else {
			add(Fail, "rclone remote %q (from %q) is not configured; see `rclone listremotes`", name, r)
		}
	}
}
