# Project folder names are immutable; Retitle replaces in-place rename

Every cross-reference in this domain that touches a Project's name — the
Track Catalog row, and (once enforced) Demo filename matching — is a plain
string match, not a stable ID. That's only safe if a Project's folder name
never changes after creation.

We considered supporting in-place rename with reference fixups (update the
Track row, re-derive Demo matches, etc.) but rejected it: naming a Project
is an artistic choice an artist should be free to reconsider mid-project,
not an engineering operation abletonctl should try to keep consistent
under the hood. Instead, changing what a Project is called is done via
**Retitle**: Ableton's own Save As into a new Project folder, followed by
`collect` to pull the old Project's external references into the new one.
The old folder is left in place, untouched, no longer the "current"
Project for that song.

This is why discovery, the Track Catalog, and Demo matching all key off
folder/file names directly with no ID layer — it's a deliberate
simplification that only holds because renaming in place is disallowed by
convention, not by code.
