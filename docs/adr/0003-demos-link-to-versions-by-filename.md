# Demos link to Versions by filename convention

A Demo is related to the Version (top-level `.als`) it was rendered from by
its name alone: `<version-slug>` or `<version-slug>__<descriptor>`, split on
the first `__`, with the left side matched **exactly** against Version
slugs. Nothing is stored: no sidecar, catalog, ID, or audio tag. Each run
re-derives the links, and a name that matches nothing is reported as
`dangling` or `unlinked`, never guessed at.

Why the filename: it is the only thing Ableton lets you author when you
export a render (the dialog defaults to the Set name and cannot write
tags), and it survives `convert-demos` unchanged, since conversion keeps the
basename. Why a reserved separator rather than prefix matching: Version
slugs in real Projects are prefix-nested (`bell-04`, `bell-04-2026`,
`bell-04-2026-02`), so a hyphen-only convention cannot tell where the slug
ends and the descriptor begins. Why Versions rather than Project folders:
versioning lives in `.als` names, and folder names do not predict them
(`ambassador Project` holds `briefcase-*.als`).

Considered and rejected: a sidecar catalog or tags written by a `link`
command (a third piece that breaks silently on rename, and the tool can't
write at export time anyway), including the folder in the name (long names
keyed on exactly what [0002](0002-project-names-are-immutable.md) says not
to rely on), and renaming legacy Demos from the tool (`backup` uses
`rclone copy`, which never deletes, so a rename would leave the old name on
the remote beside the new one).

Consequence: Version names are as immutable as Project names. Renaming a
`.als` dangles its Demos by design, and a slug that exists in two Projects
(`Untitled`) is `ambiguous` until one is renamed.
