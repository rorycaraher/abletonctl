# abletonctl Domain

`abletonctl` manages a multi-artist Ableton Live production workspace by
convention: structure is discovered from directory layout and file
presence, not declared in a manifest. Every term below names an on-disk or
on-registry structure — there is no database record backing any of them.

## Language

### Workspace structure

**Artist**:
A root directory registered by name in the machine-level registry
(`~/.config/abletonctl/config.toml`), holding one or more Productions. The
namespace boundary for backup, discovery, and the Track Catalog.
_Avoid_: namespace, artist namespace

**Production**:
Any directory directly under an Artist root matching `PRODUCTION-*`. All
matching directories are equally active — there's no tiering by year and no
concept of a Production being closed out.
_Avoid_: production dir, year

**Project**:
A direct child of a Production directory containing one or more top-level
`.als` files. A folder without one (reference material, sample packs) is
not a Project — absence of a `.als` is definitional, not an exclusion rule.
A Project's folder name is fixed at creation and never changed in place —
see Retitle for how its name is changed.
_Avoid_: song, set (see Track for the related-but-distinct catalog concept)

**User Library**:
Ableton's own shared library of presets, samples, M4L devices, and
templates, reused across every Project rather than owned by one Artist.
Registered once, machine-level, via `[library]` in the registry. Backed up
like an Artist Role, but is not itself an Artist — addressed via the
reserved pseudo-artist name `user-library`.
_Avoid_: shared library

### Backup

**Role**:
A named backup target defined per-Artist in `<artist-root>/.abletonctl.toml`:
a glob (matched against direct children of the Artist root, e.g.
`PRODUCTION-*`) paired with a destination rclone remote. Not one-to-one with
Production — `demos/` is a Role too.
_Avoid_: backup type, target

**Backup Job**:
One concrete local-directory-to-remote-directory `rclone copy` operation,
produced by expanding a Role's glob against an Artist root at run time. One
Role can expand to many Jobs (e.g. a `production` Role matching several
`PRODUCTION-YYYY` directories). Always additive — a Job never deletes from
the remote.
_Avoid_: backup task, sync job

### Project contents

**Sample**:
A file under a Project's `Samples/` directory, classified against that
Project's `.als` reference data as **Used** (exact relative-path match),
**Uncertain** (matched by filename only — e.g. moved within `Samples/`
after being referenced; never auto-quarantined), or **Orphan** (no match by
path or filename).
_Avoid_: asset, audio file

**Demo**:
A rendered mp3 under an Artist's `demos/` directory. Linked to the Project
that produced it only by a soft naming convention (demo filename
prefix-matches the Project name) — there is no stored or enforced
reference, so a renamed Project or an inconsistently-named render breaks
the link silently.
_Avoid_: render, bounce

### Cataloging

**Track**:
One row in an Artist's Track Catalog, identified by its `Track` column
value. Represents a song's status/priority/notes independent of whether a
matching Project folder exists yet (e.g. Idea-stage tracks have none).
Linked to a Project only by exact-string match on name — never enforced; a
mismatch is flagged, never blocked.
_Avoid_: song entry, catalog row

**Track Catalog**:
The per-Artist `.abletonctl-tracks.csv` file. The header row *is* the
schema — `Track` is the only required column; every other column is a
freeform string, addable by hand or via `track add`/`set` with no code
change.
_Avoid_: track list, spreadsheet

### Operations

**Collect**:
A scriptable port of Ableton's "Collect All and Save": copies external
Sample/M4L-device references into a Project's `Samples/Imported` /
`Presets/Imported` and rewrites the references to point at the copy. Never
overwrites — always writes a new numbered `.als` alongside the input
(`Song.als` → `Song-01.als`). Pack content and Ableton's own bundled
content are left alone.
_Avoid_: consolidate

**Retitle**:
The only sanctioned way to change what a Project is called. Ableton's own
Save As creates a new Project folder under the new name; Collect is then
run against the new `.als` to pull in external references from the old
Project's `Samples`/`Presets`. A Project's folder is never renamed in
place — naming is an artistic choice, not an engineering one, and treating
the folder name as mutable would silently break every name-matched
relationship in this domain (Track, Demo).
_Avoid_: rename

### Relationships

- **Artist → Production → Project**: strict containment, derived from
  directory structure. No stored IDs anywhere in this chain.
- **Project → Sample**: strict containment plus reference classification,
  derived by parsing the Project's `.als` file(s).
- **Project ↔ Track**: soft, name-matched. A Track can exist with no
  Project (Idea stage); a Project can exist with no Track row (catalog not
  updated yet).
- **Project ↔ Demo**: soft, name-prefix-matched by convention.
- **Project → Backup**: no relationship today. Backup Jobs operate at
  Production-directory granularity; no per-project backup state exists
  anywhere in the domain.

Both soft, name-matched relationships above depend on a Project's name
being permanent once set — guaranteed by Retitle, not by any code.
</content>
