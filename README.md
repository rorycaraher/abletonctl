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
# List every project discovered under projects_dir, with how many .als
# Versions it has and how many Demos link to it (see "Linking Demos to
# Versions" below).
abletonctl projects

# List every Demo under demos_dir and which Version it links to.
abletonctl demos
abletonctl demos --project ~/Music/Projects/Song

# Include the Demos on demos_remote too (lists the remote with rclone, so it
# needs rclone and network access). Each Demo is tagged local, remote or both.
abletonctl demos --remote

# Same for the Demo counts in the project list.
abletonctl projects --remote

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

```sh
# Check the config file: parses, no unknown keys, directories exist, rclone
# and its remotes are set up, ffmpeg is on PATH. Exits non-zero on failure.
abletonctl config check
```

### Linking Demos to Versions

A **Version** is a top-level `.als` file in a Project, and its **version
slug** is the file name without `.als`. A Demo links to a Version by its
name alone - when you export a render, name it either exactly the slug, or
the slug and a descriptor joined by two underscores:

```
bell-04-2026-02.wav                  # Ableton's default export name
bell-04-2026-02__soft-pad.wav        # slug __ descriptor
```

Matching is exact and case-sensitive (never a prefix guess), so `bell-04`
and `bell-04-2026` don't get confused. `abletonctl demos` reports each Demo
as:

| Status | Meaning |
|---|---|
| `linked` | the slug matches exactly one Version |
| `ambiguous` | the slug exists in more than one Project (e.g. `Untitled`); linked to none |
| `dangling` | the name has `__` but no Version has that slug (the `.als` was renamed or deleted) |
| `unlinked` | no `__` and no exact match; a plain human name |

With `--remote`, `demos` and `projects` also read `demos_remote` (a read-only
`rclone lsf`; it fails with rclone's error if the remote can't be listed). Each
Demo is tagged by **Location**: `local`, `remote` (e.g. kept on the remote
after you removed it locally, since `backup` never deletes), or `both`. Locations
are decided by name only - `both` does not mean the two files are identical.

A `.wav` and `.mp3` with the same name count as one Demo, and `convert-demos`
keeps the name, so links survive conversion. Nothing is stored - links are
re-derived from names each run - so renaming a `.als` dangles its Demos. See
`docs/adr/0003-demos-link-to-versions-by-filename.md` for the reasoning.

### Shell completion

Completion is generated by the CLI itself (zsh, bash, fish, powershell):

```sh
# zsh: add to ~/.zshrc, after compinit
source <(abletonctl completion zsh)
```

It completes command names, `--target` values, `--config` files (`*.toml`)
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
