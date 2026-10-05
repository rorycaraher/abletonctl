// Package configcheck validates the abletonctl config file and the things
// it points at: directories, rclone remotes, and the external binaries the
// commands shell out to. It reports every problem rather than stopping at
// the first.
package configcheck

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/rorycaraher/abletonctl/internal/config"
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

	var cfg config.Config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		add(Fail, "cannot load %s: %v", path, err)
		return rs
	}
	add(OK, "config parses: %s", path)

	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		sort.Strings(keys)
		add(Fail, "unknown key(s) (typo?): %s", strings.Join(keys, ", "))
	}

	checkDir := func(key, dir string, required bool) {
		switch {
		case dir == "" && required:
			add(Fail, "%s is not set", key)
		case dir == "":
			add(Warn, "%s is not set (related commands are unavailable)", key)
		default:
			info, err := os.Stat(dir)
			switch {
			case err != nil:
				hint := ""
				if strings.HasPrefix(dir, "~") {
					hint = " (~ is not expanded; use an absolute path)"
				}
				add(Fail, "%s: %v%s", key, err, hint)
			case !info.IsDir():
				add(Fail, "%s: %s is not a directory", key, dir)
			default:
				add(OK, "%s exists: %s", key, dir)
			}
		}
	}
	checkDir("projects_dir", cfg.ProjectsDir, true)
	checkDir("demos_dir", cfg.DemosDir, false)

	var remotes []string
	pair := func(dirKey, dir, remoteKey, remote string) {
		if remote != "" {
			remotes = append(remotes, remote)
		}
		if (dir == "") != (remote == "") && (dir != "" || remote != "") {
			add(Warn, "%s and %s must both be set for backup; this target will be skipped", dirKey, remoteKey)
		}
		if remote != "" && !strings.Contains(remote, ":") {
			add(Fail, "%s %q has no \"name:\" prefix", remoteKey, remote)
		}
	}
	pair("projects_dir", cfg.ProjectsDir, "projects_remote", cfg.ProjectsRemote)
	pair("demos_dir", cfg.DemosDir, "demos_remote", cfg.DemosRemote)

	if len(remotes) > 0 {
		if _, err := env.LookPath("rclone"); err != nil {
			add(Fail, "rclone not found on PATH (needed for backup)")
		} else {
			add(OK, "rclone found")
			known, err := env.RcloneRemotes()
			if err != nil {
				add(Warn, "could not list rclone remotes: %v", err)
			} else {
				have := map[string]bool{}
				for _, n := range known {
					have[n] = true
				}
				for _, r := range remotes {
					name, _, ok := strings.Cut(r, ":")
					if !ok {
						continue
					}
					if have[name] {
						add(OK, "rclone remote %q exists", name)
					} else {
						add(Fail, "rclone remote %q (from %q) is not configured; see `rclone listremotes`", name, r)
					}
				}
			}
		}
	}

	if cfg.DemosDir != "" {
		if _, err := env.LookPath("ffmpeg"); err != nil {
			add(Warn, "ffmpeg not found on PATH (needed for convert-demos)")
		} else {
			add(OK, "ffmpeg found")
		}
	}
	return rs
}
