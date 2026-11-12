# Any number of named Directories, each with any number of Remotes

This partly supersedes [0001](0001-single-workspace-mvp.md). The config is
now a set of named Directories (`[dirs.<name>]`), each one a `path` plus a
`remotes` list. There is still one solo producer on one computer, and 0001's
other removals (artist registry, `PRODUCTION-*` grouping, track catalog
CSV) still apply. What changed is 0001's rule of "exactly two fixed backup
targets, no named backup roles". The fixed set had already grown to three
(the User Library came back), and backing a Directory up to more than one
place, or backing up something Ableton never touches, like a samples drive,
became a real need rather than speculative flexibility.

Two names are reserved: `projects` (the Projects directory) and `demos` (the
Demos directory). Commands that need one of them look it up by name. Every
other Directory is only backed up. We chose reserved names over a `kind`
field on each entry because TOML keys are unique, so "at most one Projects
directory and one Demos directory" holds without extra rules. A `kind`
field would allow two Directories of the same kind, and then every command
would need a rule for which one to use.

A Remote is still the exact rclone destination, with no implicit subfolder.
We rejected declaring Remotes once by name and deriving `<remote>/<dir-name>`:
it is less repetitive, but renaming a Directory would silently move where it
backs up to.

`backup` runs every Directory→Remote copy, keeps going past failures,
reports each result, and exits non-zero if any copy failed. With several
Remotes, one unreachable destination (a NAS that's switched off) shouldn't
stop the others.
