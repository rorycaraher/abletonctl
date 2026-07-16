# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`abletonctl` is a Go CLI for managing a multi-artist Ableton Live production
workspace: discovering projects, backing up production/demo directories to
configurable rclone remotes, finding orphaned samples, collecting external
file references into a project (a scriptable "Collect All and Save"),
converting rendered demos to mp3, and tracking per-track status in a
lightweight CSV catalog. See `README.md` for full user-facing docs and
domain vocabulary (Artist, Production, Project, Demos, Track catalog, User
Library) and `IDEAS.md` for deliberately-deferred features/backlog.

## Commands

```sh
go build ./...
go vet ./...
go test ./...
go test ./internal/collect/...              # single package
go test ./internal/samples/ -run TestScan   # single test
go build -o abletonctl ./cmd/abletonctl     # build the CLI binary
```

There is no separate lint config beyond `go vet`. `collect` and `demos`
tests may shell out to real `ffmpeg`/binary behavior indirectly through
fixtures — check the relevant `_test.go` before assuming pure-Go isolation.

`backup` requires the `rclone` binary on `PATH` at runtime (not at test
time); `convert-demos` requires `ffmpeg`. Neither is a Go dependency.

## Architecture

Single-binary CLI, one flat command dispatch in `cmd/abletonctl/main.go`
(a `switch` on `os.Args[1]`, no CLI framework). Each subcommand's `run*`
function parses its own args — most via `flag.FlagSet`, but `prune-samples`,
`collect`, and `track add/set` parse by hand because they mix positional
arguments (a path, or trailing `Key=Value` pairs) with flags in ways
`flag.FlagSet` can't express (it stops recognizing flags after the first
positional). Keep that pattern if extending those commands.

All actual logic lives in `internal/`, one package per concern, with
`main.go` doing only argument parsing, orchestration, and output
formatting:

- **`internal/config`** — loads the top-level artist registry
  (`~/.config/abletonctl/config.toml`, artist name → root dir, plus an
  optional single `[library]` section for the Ableton User Library) and the
  per-artist role config (`<artist-root>/.abletonctl.toml`, role name → glob
  + rclone remote). Pure data loading, no filesystem scanning beyond the two
  TOML files.

- **`internal/discovery`** — structural filesystem scanning with no
  exclude-lists: `MatchRoleDirs` finds direct children of a root matching a
  glob (used for both `PRODUCTION-*` and arbitrary role globs like
  `demos`), `DiscoverProjects` finds child directories that contain a
  top-level `.als` file (Ableton's auto-generated `Backup/` folder is never
  scanned). Every other package that needs to know what a "project" is
  depends on this one.

- **`internal/backup`** — turns a loaded registry + optional artist/role
  filter into a flat list of `Job{Artist, Role, LocalDir, RemoteDir}` via
  `BuildJobs`, then runs each through `rclone copy` (never `sync` — copy
  only adds/updates on the remote, so a local mistake can't destroy the
  remote copy too). The User Library is handled as a pseudo-artist
  (`LibraryArtist = "user-library"`) so it reuses the same `--artist` flag
  instead of needing its own.

- **`internal/samples`** — orphan-sample detection. `als.go` streams the
  gzipped XML of an `.als` file token-by-token (not a fixed-schema
  unmarshal — Ableton's XML shape drifts across Live versions) collecting
  every `RelativePath`/`Path` element into a `SampleRefs` set.
  `samples.go`'s `Scan` walks a project's `Samples/` dir and classifies each
  file as `Used` (exact relative-path match — reliable), `Uncertain`
  (filename-only match — never auto-touched), or `Orphan` (no match at
  all). `Quarantine` moves only `Orphan` files into `_unreferenced/`,
  preserving their path under `Samples/`; it never deletes.

- **`internal/collect`** — the largest and most delicate package: a
  scriptable port of Ableton's "Collect All and Save", reverse-engineered
  from real before/after `.als` diffs rather than Ableton's docs (see the
  package doc comment for exactly what is/isn't collected). It works by
  line-oriented in-place patching of the decompressed XML — tracking
  `(line, value)` locations for the fields it cares about during a scan
  pass (`scanFileRefs`), deciding what to copy/rewrite (`analyze`), then
  patching only those lines and re-gzipping (`applyCollect`) — rather than
  a full XML round-trip, to avoid reformatting/reordering unrelated parts
  of the document. Never overwrites the input `.als`; always writes a new
  numbered file alongside it (`Song.als` → `Song-01.als`). Pack content
  (`LivePackId` set) and Ableton's own bundled content (path contains
  `.app/Contents/`) are always left alone.

- **`internal/demos`** — converts `aiff`/`wav` under a role directory to
  320k CBR mp3 via `ffmpeg` and deletes the original **only** after
  verifying the output exists and is non-empty (`Convert` then `os.Remove`
  in `ConvertAndCleanup`); a failed conversion never triggers a delete.

- **`internal/tracks`** — CSV-backed track catalog
  (`<artist-root>/.abletonctl-tracks.csv`). The header row *is* the schema:
  `Track` is the only required/identity column; every other column is a
  freeform string, and `Catalog.ensureColumn` appends new ones on the fly
  from `track add`/`set` `Key=Value` pairs with no code change. Blank rows
  (used as visual dividers when hand-edited in a spreadsheet) are dropped
  on `Load` and not preserved on `Save`.

## Conventions worth preserving when extending

- Favor structural detection (a glob, a required file's presence) over
  maintained exclude-lists — this shows up in `discovery.DiscoverProjects`
  (any dir with a top-level `.als`) and `demos` (any `aiff`/`wav` under a
  role dir), and is a deliberate project-wide preference, not incidental.
- Destructive operations are conservative by default: `backup` uses
  `rclone copy` never `sync`; `prune-samples --quarantine` moves rather
  than deletes and skips anything not confidently orphaned; `convert-demos`
  only deletes a source file after verifying its converted replacement;
  `collect` never overwrites the input `.als`. Match this bias — prefer
  "leave it and report" over "delete/overwrite" — for any new mutating
  command.
- Each `internal/` package is independently testable against fixture data
  (see `internal/*/*_test.go`) without needing `rclone`/`ffmpeg` installed;
  keep new packages structured so their core logic doesn't require shelling
  out in tests.
