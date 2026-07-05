# abletonctl

`abletonctl` is a small CLI for managing a multi-artist Ableton Live
production workspace: discovering projects, backing up production/demo
directories to configurable rclone remotes, finding samples that aren't
referenced by any project anymore, collecting external file references into
a project (a scriptable "Collect All and Save"), converting rendered demos
to mp3, and tracking per-track status in a lightweight CSV catalog.

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
- **Track catalog**: a `.abletonctl-tracks.csv` file at the artist root
  tracking status/priority/notes/etc. per track - a local replacement for
  a spreadsheet. The header row *is* the schema: `Track` is the only
  required column (row identity), every other column is a freeform string
  and new ones can be added at any time, by hand or via `track add`/`set`,
  with no code change.
- **User Library**: Ableton's own shared library of presets, samples, M4L
  devices, and templates (`~/Music/Ableton/User Library` by default) - reused
  across every project rather than belonging to one artist namespace. It's
  optionally registered once, machine-level, via `[library]` in the registry
  (see `examples/config.toml`), and backed up with the same `backup` command
  as artist roles.

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

3. Optionally, a `[library]` section in the registry to back up the User
   Library - see `examples/config.toml`. Same "remote is the parent"
   convention as a role.

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

# Back up just the User Library (requires [library] in the registry).
abletonctl backup --artist user-library --dry-run

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

```sh
# Scriptable version of Ableton's own "Collect All and Save" (File menu):
# copies external audio/M4L-device references into Samples/Imported and
# Presets/Imported, and rewrites those references to point at the copy.
# Never overwrites the input - always writes a new numbered .als alongside it
# (Song.als -> Song-01.als). Pack content, Ableton's own bundled content, and
# anything already inside the project are left alone.
abletonctl collect ~/Music/artist-name/PRODUCTION-2026/Song/Song.als

# Same, for every top-level .als in a directory. Each file is independent -
# one failing doesn't stop the rest.
abletonctl collect --all ~/Music/artist-name/PRODUCTION-2026/Song
```

```sh
# List the track catalog for every registered artist (or just one).
# Rows with no matching project folder on disk are flagged - expected for
# Idea-stage tracks that don't have one yet.
abletonctl tracks
abletonctl tracks --artist artist-name

# Add a new track. Any Key=Value pair becomes a column; unrecognized
# columns are created on the fly. Errors if the track already exists.
abletonctl track add "New Idea" Status=Idea

# Update an existing track. Errors if it doesn't exist yet (use add
# instead). --artist is only required when more than one artist is
# registered.
abletonctl track set "Zap Dub" --artist artist-name Status=Mixdown Priority=A
```

The catalog is a plain CSV at `<artist-root>/.abletonctl-tracks.csv`, so it
can also be hand-edited or opened directly in Numbers/Excel/Sheets for bulk
changes - `track add`/`set` are just a typo-safe shortcut for the common
single-track update, not the only way to edit it. Blank rows (e.g. used as
visual dividers between batches in a spreadsheet) are skipped on read and
not preserved on write.

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

See `IDEAS.md` for features considered but not (yet) built.
