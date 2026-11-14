# Detect the DAW from Project layout; no per-DAW command group

Only Ableton Live is in use, but the tool should stay DAW-agnostic. Commit
`73b2af7` had moved the Project commands (`projects`, `find-orphans`,
`collect`) under `studioctl ableton`. That would have forced a second,
parallel command group (`studioctl logic …`) the day another DAW appeared,
and made the user say which DAW a Project is when the files already say it.

The Project commands are top-level again and the DAW is detected, never
configured. `internal/discovery` has one detector per supported DAW that
recognises a Project by its layout (Ableton: a top-level `.als` file).
Commands given an explicit path (`find-orphans`, `collect`) detect from
that path; `projects` reads the projects Directory, where every Project is
assumed to come from the same DAW, so the first one found decides. Each
command switches on the detected DAW and errors for one it has no
implementation for, rather than guessing.

The command names were chosen to read for any DAW: "collect" is Live's
Collect All and Save, Logic's Consolidate, Reaper's save-with-media;
"orphans" is any sample a Project no longer references.

Considered alternatives: a `daw = "ableton"` config key (rejected — another
setting that can disagree with the files, against the preference for
structural detection; see 0004 on keeping the config surface small);
keeping `studioctl ableton` as an alias (rejected — nobody depends on it
yet); stubbing detectors for DAWs not in use (rejected — add one when a
real Project needs it).
