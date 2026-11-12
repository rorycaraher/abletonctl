# abletonctl

`abletonctl` is a small CLI for managing a single Ableton Live production
workspace: discovering projects, finding orphaned samples,
collecting external file references into a project
(a scriptable "Collect All and Save"), converting rendered
demos to mp3, and backing up all projects and demos to configurable rclone remotes.

## Conventions

- **Projects directory**: the directory containing every Ableton Project
  you want this tool to manage. Configured once, directly - there's no
  registry, no name, no grouping subdirectory. Projects are its direct
  children.
- **Project**: any direct child of the projects directory that contains a
  top-level `.als` file. Folders without one (reference material, sample
  packs, etc.) are skipped automatically - there's no exclude-list to
  maintain.
- **Demos directory**: wherever your WIP demo bounces land. Configured
  independently of the projects directory - it doesn't need to live inside
  it. Only mp3 belongs here long-term - aiff/wav masters for finishing live
  elsewhere. `convert-demos` (below) enforces that.

## Install

Download the archive for your OS/arch from the
[latest release](https://github.com/rorycaraher/abletonctl/releases/latest),
extract it, and put the `abletonctl` binary on your `PATH`:

```sh
# macOS (Apple Silicon), for example:
curl -LO https://github.com/rorycaraher/abletonctl/releases/latest/download/abletonctl_Darwin_arm64.tar.gz
tar -xzf abletonctl_Darwin_arm64.tar.gz abletonctl
mv abletonctl /usr/local/bin/   # or anywhere on your PATH
```

Other builds on the releases page: `Darwin_x86_64`, `Linux_x86_64`,
`Linux_arm64`, `Linux_i386`, and `Windows_x86_64`/`arm64`/`i386` (`.zip`).

Or build from source:

```sh
go build -o abletonctl ./cmd/abletonctl
mv abletonctl /usr/local/bin/
```

Requires the `rclone` binary on your `PATH` for the `backup` command, with
your remotes (`r2`, `gdrive`, etc.) already configured via `rclone config`.
Requires `ffmpeg` on your `PATH` for `convert-demos`.

## Configure

A single config file at `~/.config/studioctl/config.toml`. See
`examples/config.toml`.

```toml
[dirs.projects]
path = "~/Music/Projects"
remotes = ["r2:my-bucket/projects", "nas:music/projects"]

[dirs.demos]
path = "~/Music/Demos"
remotes = ["gdrive:my-demos"]

[dirs.user-library]
path = "~/Music/Ableton/User Library"
remotes = ["r2:my-bucket/user-library"]
```

Each `[dirs.<name>]` entry is a **Directory**: a local `path` and any number
of rclone `remotes` it's backed up to. Two names are special - `projects`
and `demos` - and commands that need them look them up by name; every other
Directory is only backed up. A leading `~/` in a path is your home
directory.

Backups always use `rclone copy` (never `sync`) - they only add/update files
on the remote, they never delete from it. A local mistake can't take out
your only backup copy too, at the cost of the remote potentially
accumulating files you've since removed or renamed locally. Each remote is
the exact destination - the Directory's contents land there directly, with
no extra subfolder.

## Usage

```sh
# List every project discovered in the projects Directory, with how many .als
# Versions it has.
abletonctl projects

# List every Demo in the demos Directory. A .wav and .mp3 with the same name are
# one Demo.
abletonctl demos

# Include the Demos on the demos Directory's remote too (a read-only rclone listing, so it
# needs rclone and network access). Each Demo is tagged local, remote or
# both - decided by name only, so "both" doesn't mean the files are identical.
abletonctl demos --remote

# Convert every aiff/wav in the demos Directory to a 320k mp3, then permanently
# delete the original (and its Ableton .asd analysis file) once the
# conversion is verified to have succeeded. A
# failed conversion never deletes its source - it's reported and left in
# place for you to deal with by hand.
abletonctl convert-demos
abletonctl convert-demos --dry-run

# List where a wav/aiff file goes over -6 dBFS, as HH:MM:SS.mmm. Crossings
# less than 100ms apart are one peak, reported at its loudest sample.
abletonctl find-peaks mix.wav
abletonctl find-peaks mix.wav --threshold -1

# Show the bar each peak falls in too. Assumes the file starts at bar 1;
# --bpm is quarter notes per minute, as Live shows it. 4/4 unless --timesig.
abletonctl find-peaks mix.wav --bpm 124
abletonctl find-peaks mix.aif --bpm 90 --timesig 6/8

# Back up every Directory to each of its remotes. Every copy runs even if
# an earlier one fails (an unreachable remote, say); a summary at the end
# lists each result, and the exit status is non-zero if any copy failed.
abletonctl backup

# Only some Directories, and preview without transferring.
abletonctl backup demos user-library --dry-run

# Each copied file is listed by default; --quiet (-q) suppresses that.
abletonctl backup --quiet

# A real (non-dry-run) backup asks for confirmation first; --yes skips it,
# and is required when stdin isn't a terminal (cron, scripts).
abletonctl backup --yes

# Find samples that no top-level .als in a project references anymore.
abletonctl find-orphans ~/Music/Projects/Song

# Move the ones it's confident about into Song/_unreferenced/, preserving
# their path under Samples/. Never deletes; never touches files it's only
# "uncertain" about (matched by filename but not by path - reopen the
# project and check those by hand).
abletonctl find-orphans ~/Music/Projects/Song --quarantine
```

`find-orphans` only looks at top-level `.als` files - Ableton's
auto-generated `Backup/` folder is ignored, since counting old backups as
"using" a sample would mean almost nothing ever looks orphaned.

```sh
# Scriptable version of Ableton's own "Collect All and Save" (File menu):
# copies external audio/M4L-device references into Samples/Imported and
# Presets/Imported, and rewrites those references to point at the copy.
# Never overwrites the input - always writes a new numbered .als alongside it
# (Song.als -> Song-01.als). Pack content, Ableton's own bundled content, and
# anything already inside the project are left alone.
abletonctl collect ~/Music/Projects/Song/Song.als

# Same, for every top-level .als in a directory. Each file is independent -
# one failing doesn't stop the rest.
abletonctl collect --all ~/Music/Projects/Song
```

```sh
# Check the config file: parses, no unknown keys, directories exist, rclone
# and its remotes are set up, ffmpeg is on PATH. Exits non-zero on failure.
abletonctl config check
```

### Shell completion

Completion is generated by the CLI itself (zsh, bash, fish, powershell):

```sh
# zsh: add to ~/.zshrc, after compinit
source <(abletonctl completion zsh)
```

It completes command names, Directory names for `backup`, `--config` files (`*.toml`)
and project directories for `find-orphans`. Run `abletonctl completion --help`
for the other shells.

## Development

Tooling is pinned in `mise.toml`:

```sh
mise install       # go, golangci-lint, pre-commit, gitleaks, actionlint
mise run check     # gofmt check, vet, lint, test (same as CI)
mise run hooks     # install the git pre-commit hook (once per clone)
```

The pre-commit hook runs gofmt, `go vet`, golangci-lint and gitleaks (and
actionlint when a workflow file changes); `go test` runs on pre-push. CI
(`.github/workflows/ci.yml`) runs `mise run check` and gitleaks on every PR
and push to `main`; a separate workflow runs actionlint only when
`.github/**` changes.

See `IDEAS.md` for features considered but not (yet) built, and
`docs/adr/` for the reasoning behind bigger structural decisions.
