# Strip down to a single-workspace MVP

The tool launched as a "multi-artist Ableton Live production workspace"
manager: an artist-name registry mapping to N root directories, a
`PRODUCTION-*` grouping convention within each, a per-artist role config for
backups, a tracker CSV, and User Library backup. In practice, the collect,
prune, backup, and demos-conversion features earned their keep; the
multi-artist registry and tracker CSV turned out half-baked, with no clear
final shape yet.

We cut the tool down to what one person managing one workspace actually
needs: a single `projects_dir` and a single `demos_dir`, backup collapsed to
exactly two fixed targets (no glob-matched roles, no `PRODUCTION-*` layer),
and one flat config file. Multi-artist support, the tracker CSV, and User
Library backup are removed rather than kept dormant — they're preserved on
a separate branch to revisit once their design is actually settled, rather
than carrying half-finished flexibility in the MVP.

Considered alternatives: keeping the registry but requiring exactly one
entry (rejected — keeps dead flexibility and a name you'd have to invent for
no reason); keeping general-purpose named backup roles (rejected — the same
kind of unneeded config surface, when there are only ever two real targets).
