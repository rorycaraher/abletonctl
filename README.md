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

```sh
go build -o abletonctl ./cmd/abletonctl
mv abletonctl /usr/local/bin/   # or anywhere on your PATH
```

Requires the `rclone` binary on your `PATH` for the `backup` command, with
your remotes (`r2`, `gdrive`, etc.) already configured via `rclone config`.
Requires `ffmpeg` on your `PATH` for `convert-demos`.

## Configure

A single config file at `~/.config/abletonctl/config.toml`. See
`examples/config.toml`.

```toml
projects_dir = "/path/to/your/projects"
demos_dir = "/path/to/your/demos"

projects_remote = "r2:my-bucket/projects"
demos_remote = "gdrive:my-demos"
```

Backups always use `rclone copy` (never `sync`) - they only add/update files
on the remote, they never delete from it. A local mistake can't take out
your only backup copy too, at the cost of the remote potentially
accumulating files you've since removed or renamed locally. Each remote is
the exact destination - `projects_dir`'s contents land there directly, with
no extra subfolder.

## Usage

```sh
# List every project discovered under projects_dir.
abletonctl projects

# Convert every aiff/wav under demos_dir to a 320k mp3, then permanently
# delete the original once the conversion is verified to have succeeded. A
# failed conversion never deletes its source - it's reported and left in
# place for you to deal with by hand.
abletonctl convert-demos
abletonctl convert-demos --dry-run

# Back up both projects_dir and demos_dir.
abletonctl backup

# Narrow to one target, and preview without transferring.
abletonctl backup --target demos --dry-run

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

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

See `IDEAS.md` for features considered but not (yet) built, and
`docs/adr/` for the reasoning behind bigger structural decisions.
