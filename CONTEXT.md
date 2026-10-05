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
