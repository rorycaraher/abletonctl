# studioctl

A personal CLI for a solo music producer's files on one computer: backing up
any number of Directories to rclone Remotes, listing and converting demos,
and finding peaks in rendered audio. Ableton-specific tools — discovering
Projects, finding orphaned samples, collecting external file references into
a Project — live under `studioctl ableton`.

## Language

**Directory**:
A named local directory the tool knows about, with zero or more Remotes.
There can be any number of them; most are only backed up. Two names are
special: `projects` (the Projects directory) and `demos` (the Demos
directory), so there is at most one of each.
_Avoid_: Backup target, role, workspace

**Remote**:
An exact rclone destination a Directory is copied to. The Directory's
contents land directly in it; no subfolder is added. A Directory may have
several Remotes, or none.
_Avoid_: Backup, mirror

**Projects directory**:
The Directory named `projects`: it holds every Ableton Project the tool
manages. Projects are its direct children — there is no intermediate
grouping directory.
_Avoid_: Artist directory, artist root, artist namespace

**Demos directory**:
The Directory named `demos`, where WIP demo bounces land. Independent of
the Projects directory — it is not assumed to be a subdirectory of it, since
demo bounces may live somewhere else entirely (a synced folder, a separate
drive).
_Avoid_: Demos role, demos glob

**Project**:
Any direct child of the Projects directory that contains a top-level `.als`
file. Directories without one (reference material, sample packs, etc.) are
skipped automatically. Only top-level `.als` files count — Ableton's
auto-generated `Backup/` folder is never scanned. A Project's name is its
folder name without the ` Project` suffix Live appends (`installations
Project` is `installations`); the suffix is optional wherever a name is
accepted.

**Orphan**:
A file under a Project's `Samples/` directory that no top-level `.als` in
that project references, by path or filename. `find-orphans` reports these
and, with `--quarantine`, moves (never deletes) them into `_unreferenced/`.
_Avoid_: Unused sample, unreferenced sample

**Uncertain**:
A file under a Project's `Samples/` directory whose filename appears in an
`.als` reference but whose path doesn't match — e.g. it was moved within
`Samples/` after being added to the Live set. Reported by `find-orphans` but
never auto-quarantined; requires opening the project and checking by hand.

### Demos and Versions

**Version**:
A top-level `.als` file in a Project: one saved iteration of it. A Project
has one or more Versions.
_Avoid_: Live Set, set, revision

**Demo**:
A rendered piece of audio in the Demos directory, identified by its name
without extension; a `.wav` and `.mp3` with the same name are one Demo.
Ableton's `.asd` analysis files are not Demos. A Demo is found locally, on
some of the Demos directory's Remotes, or both (see Location).
_Avoid_: Render, bounce, track

**Location**:
The set of places a Demo is found: the Demos directory, and each of its
Remotes that has it. Decided by name (relative directory and name without
extension) only, never by comparing content, so being found in two places
does not mean the files are identical. A Demo missing from some Remotes is
a backup gap.
_Avoid_: Synced, backed up, archived

### Levels

**Peak**:
One excursion of an audio file over a level threshold (dBFS, -6 by
default), as `find-peaks` reports it: crossings less than 100ms apart are
one Peak, positioned at its loudest sample. Not one sample, and not a
single waveform cycle.
_Avoid_: Clip (Live's word for a region on a track), transient
