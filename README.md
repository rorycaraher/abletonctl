# ableton-framework

`abletonctl` is a small CLI for managing a multi-artist Ableton Live
production workspace: discovering projects, backing up production/demo
directories to configurable rclone remotes, and finding samples that aren't
referenced by any project anymore.

## Conventions

- **Artist namespace**: a root directory (e.g. `artist-name/`) registered by
  name. You can register more than one.
- **Production**: any directory directly under an artist root matching
  `PRODUCTION-*` (e.g. `PRODUCTION-2026`). All matching directories are
  treated as equally active - there's no special handling for "old" years.
- **Project**: any direct child of a `PRODUCTION-*` directory that contains a
  top-level `.als` file. Folders without one (reference material, sample
  packs, etc.) are skipped automatically - there's no exclude-list to
  maintain.
- **Demos**: a `demos/` directory under the artist root holding rendered
  demo tracks. Only mp3 belongs here long-term - aiff/wav masters for
  finishing live elsewhere. `convert-demos` (below) enforces that.

## Install

```sh
go build -o abletonctl ./cmd/abletonctl
mv abletonctl /usr/local/bin/   # or anywhere on your PATH
```

Requires the `rclone` binary on your `PATH` for the `backup` command, with
your remotes (`r2`, `gdrive`, etc.) already configured via `rclone config`.
Requires `ffmpeg` on your `PATH` for `convert-demos`.

## Configure

1. Registry file at `~/.config/abletonctl/config.toml`, mapping each artist
   name to its root directory. See `examples/config.toml`.

2. A per-artist config at `<artist-root>/.abletonctl.toml`, mapping roles to
   rclone remotes. See `examples/dot-abletonctl.toml`. `remote` should be the
   *parent* of the folder being copied - each matched local directory lands
   in a like-named subfolder at the remote, so multiple `PRODUCTION-*` years
   don't collide.

Backups always use `rclone copy` (never `sync`) - they only add/update files
on the remote, they never delete from it. A local mistake can't take out
your only backup copy too, at the cost of the remote potentially
accumulating files you've since removed or renamed locally.

## Usage

```sh
# List every registered artist and the projects discovered under each
# PRODUCTION-* directory.
abletonctl projects
abletonctl projects --artist artist-name

# Convert every aiff/wav under a registered demos/ directory to a 320k mp3,
# then permanently delete the original once the conversion is verified to
# have succeeded. A failed conversion never deletes its source - it's
# reported and left in place for you to deal with by hand.
abletonctl convert-demos
abletonctl convert-demos --artist artist-name --dry-run

# Back up everything registered.
abletonctl backup

# Narrow to one artist and/or role, and preview without transferring.
abletonctl backup --artist artist-name --role production --dry-run

# Find samples that no top-level .als in a project references anymore.
abletonctl prune-samples ~/Music/artist-name/PRODUCTION-2026/Song

# Move the ones it's confident about into Song/_unreferenced/, preserving
# their path under Samples/. Never deletes; never touches files it's only
# "uncertain" about (matched by filename but not by path - reopen the
# project and check those by hand).
abletonctl prune-samples ~/Music/artist-name/PRODUCTION-2026/Song --quarantine
```

`prune-samples` only looks at top-level `.als` files - Ableton's
auto-generated `Backup/` folder is ignored, since counting old backups as
"using" a sample would mean almost nothing ever looks orphaned.

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

See `IDEAS.md` for features considered but not (yet) built.
