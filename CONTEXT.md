# abletonctl

A CLI for managing a single Ableton Live production workspace: discovering
projects, finding orphaned samples, collecting external file references into
a project, converting rendered demos to mp3, and backing up projects/demos to
a remote.

## Language

**Projects directory**:
The directory containing every Ableton Project the tool manages. Configured
once, directly (no registry, no name). Projects are its direct children —
there is no intermediate grouping directory.
_Avoid_: Artist directory, artist root, artist namespace

**Demos directory**:
The directory WIP demo bounces land in. Configured independently of the
Projects directory — it is not assumed to be a subdirectory of it, since
demo bounces may live somewhere else entirely (a synced folder, a separate
drive).
_Avoid_: Demos role, demos glob

**Project**:
Any direct child of the Projects directory that contains a top-level `.als`
file. Directories without one (reference material, sample packs, etc.) are
skipped automatically. Only top-level `.als` files count — Ableton's
auto-generated `Backup/` folder is never scanned.

**Backup target**:
One of the two things `backup` can copy to a remote: the Projects directory
or the Demos directory. Each target has its own remote destination. A remote
is the exact destination for its target's contents — no implicit subfolder
is added.
_Avoid_: Role, backup role

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
has one or more Versions. A Demo links to a Version, not to the Project
folder; the Project is the Version's parent folder.
_Avoid_: Live Set, set, revision

**Version slug**:
A Version's file name without `.als` (`43-missed-calls-RESHAPE-03`). Not
unique across Projects, and one slug may be a prefix of another
(`bell-04`, `bell-04-2026`).
_Avoid_: Version stem (a "stem" is a rendered audio stem), project name,
version name

**Demo**:
A rendered piece of audio in the Demos directory, identified by its name
without extension; a `.wav` and `.mp3` with the same name are one Demo.
Ableton's `.asd` analysis files are not Demos. A Demo is found locally, on
the remote, or both (see Location).
_Avoid_: Render, bounce, track

**Location**:
Where a Demo is found: `local` (Demos directory), `remote` (the Demos
remote), or `both`. Decided by name (relative directory and name without
extension) only, never by comparing content, so `both` does not mean the two
files are identical.
_Avoid_: Synced, backed up, archived

**Demo name**:
A Demo's name, either exactly a Version slug, or a Version slug and a
human-chosen descriptor joined by `__`: `bell-04-2026-02__soft-pad`. The
descriptor is free text; the slug is what links the Demo to a Version.
_Avoid_: Demo title

**Linked**:
A Demo whose Version slug matches exactly one Version.

**Ambiguous**:
A Demo whose Version slug matches Versions in more than one Project
(e.g. `Untitled`). It is linked to none of them.

**Dangling**:
A Demo whose name contains `__` but whose Version slug matches no Version,
e.g. the `.als` was renamed or deleted.

**Unlinked**:
A Demo with no `__` that doesn't equal any Version slug: a plain human name
with no recorded relationship.
