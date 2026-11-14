# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`studioctl` is a Go CLI for one solo producer's music files on one computer:
backing up any number of named Directories to rclone Remotes, listing Demos
and where they're backed up, converting rendered demos to mp3, finding peaks
in audio files, and validating its own config file. The Project tools —
listing Projects, finding orphaned samples, collecting external file
references into a Project (a scriptable "Collect All and Save") — are named
DAW-agnostically and detect the DAW from the Project's layout; Ableton Live
is the only one implemented. It was called `abletonctl` until it outgrew
Ableton.

- `README.md` — user-facing docs: install, configure, usage.
- `CONTEXT.md` — domain vocabulary (Directory, Remote, Projects directory,
  Demos directory, Project, Version, Demo, Location, Orphan, Uncertain, Peak)
  and terms to avoid. Use these terms in code, docs, and commit messages.
- `docs/adr/` — why the structure is what it is. Read
  `0001-single-workspace-mvp.md` before reintroducing anything multi-artist
  (the artist registry, `PRODUCTION-*` grouping, the track catalog CSV were
  removed on purpose), and `0004-named-directories-with-remotes.md` before
  changing how Directories and Remotes are configured.
- `IDEAS.md` — deliberately-deferred features.

## Commands

```sh
mise install        # pinned toolchain: go, golangci-lint, pre-commit, gitleaks, actionlint
mise run check      # what CI runs: gofmt check, go vet, golangci-lint, go test
mise run build      # go build -o studioctl ./cmd/studioctl
mise run hooks      # install the pre-commit hook (once per clone)
```

Without mise, the underlying commands work directly:

```sh
go build ./...
go vet ./...
go test ./...
go test ./internal/collect/...              # single package
go test ./internal/samples/ -run TestScan   # single test
```

`collect` and `demos` tests may shell out to real `ffmpeg`/binary behavior
indirectly through fixtures — check the relevant `_test.go` before assuming
pure-Go isolation. `backup` requires the `rclone` binary on `PATH` at runtime
(not at test time); `convert-demos` requires `ffmpeg`. Neither is a Go
dependency.

### Tooling and CI

- `mise.toml` pins tool versions and defines the tasks above.
- `.golangci.yml` is golangci-lint v2 defaults, minus unchecked-`Close`
  errcheck noise (deferred `Close` on read-only files, and all of `_test.go`)
  and the staticcheck style rules `QF1002`/`ST1005`.
- `.pre-commit-config.yaml`: on commit, file hygiene + gofmt + `go vet` +
  golangci-lint + gitleaks, plus `actionlint` when a workflow file changes;
  `go test` on pre-push.
- `.github/workflows/ci.yml` runs `mise run check` and `gitleaks` on PRs and
  pushes to `main`. `actionlint.yml` runs `actionlint` only when `.github/**`
  changes (it installs just that tool).
- `.github/workflows/release.yml` runs goreleaser (`.goreleaser.yaml`) on
  `v*` tags, using the Go version from `go.mod`. goreleaser injects
  `main.version` via ldflags.

## Architecture

Single-binary CLI built on [cobra](https://github.com/spf13/cobra). `main.go`
only declares `version` and runs `newRootCmd()` (`root.go`), printing any
returned error as `studioctl: <err>` and exiting 1. Each command lives in its
own file in `cmd/studioctl/` as a `newXCmd()` constructor registered in
`newRootCmd()`:

| File | Command(s) |
|---|---|
| `backup.go` | `backup [dir...] [--dry-run] [--quiet] [--yes]` (every copy runs; summary at the end) |
| `projects.go` | `projects` (lists Projects with their DAW and Version counts) |
| `config.go` | `config check` |
| `orphans.go` | `find-orphans <project-path> [--quarantine]` |
| `collect.go` | `collect <version>` / `collect --all <project-path>` |
| `demos.go` | `convert-demos [--dry-run]` |
| `peaks.go` | `find-peaks <wav\|aiff> [--threshold dB] [--bpm n] [--timesig N/D]` |
| `demos_list.go` | `demos [--remote]` (lists Demos and their Location: local and each Remote) |
| `style.go` | lipgloss styles (no command) |

Conventions for the command layer:

- `SilenceUsage`/`SilenceErrors` are set on the root; return errors from
  `RunE` rather than printing and exiting.
- `--config` is added only to commands that read the config file, via
  `addConfigFlag`. `find-orphans` and `collect` take explicit paths instead.
- Commands are named DAW-agnostically; there is no per-DAW command group.
  A command that works on Projects asks `discovery` which DAW it's looking
  at (`LoadProject`/`VersionDAW` for an explicit path, `DiscoverProjects`
  for the projects Directory) and switches on `discovery.DAW`, returning a
  "doesn't support … yet" error for DAWs it has no implementation for. See
  `docs/adr/0005-detect-the-daw-from-project-layout.md`.
- Long flags are `--double-dash` only (pflag); flags may appear before or
  after positional arguments.
- Shell completion (`studioctl completion zsh|bash|fish|powershell`) is
  cobra's built-in. Give flags with a fixed value set a
  `RegisterFlagCompletionFunc` and path arguments a `ValidArgsFunction`.
- Styling (lipgloss) degrades to plain text when stdout isn't a terminal or
  `NO_COLOR` is set. Keep piped output free of escape codes.

All actual logic lives in `internal/`, one package per concern; the command
files do only argument handling, orchestration, and output formatting:

- **`internal/config`** — loads the config file
  (`~/.config/studioctl/config.toml`): any number of named Directories
  (`[dirs.<name>]`), each a `path` and a list of rclone `remotes`. The names
  `projects` and `demos` are reserved (`config.Projects`/`config.Demos`) and
  looked up by commands; every other Directory is only backed up. `Names()`
  keeps file order; a leading `~/` in a path is expanded. See
  `examples/config.toml` and `docs/adr/0004-named-directories-with-remotes.md`.

- **`internal/configcheck`** — backs `config check`. Flags unknown keys at
  both levels (typos), checks each Directory's path exists, remotes have a
  `name:` prefix and exist in `rclone listremotes`, and `rclone`/`ffmpeg` are
  on `PATH`. Fails on a Remote shared by two Directories; warns on nested
  Remotes or paths, a missing `projects`/`demos`, and a non-reserved
  Directory with no Remotes. Returns a list of `ok`/`warn`/`FAIL` results
  rather than stopping at the first problem. Machine lookups go through an
  injectable `Env`, so tests need neither binary.

- **`internal/discovery`** — structural filesystem scanning with no
  exclude-lists, and DAW detection. One `detector` per supported DAW
  recognises its Projects by layout (Ableton: a top-level `.als` file;
  Live's auto-generated `Backup/` folder is never scanned). `LoadProject`
  reads one folder, `DiscoverProjects` finds the Projects among a
  directory's direct children, `DetectDAW` takes the first one's DAW (every
  Project in the projects Directory is assumed to share one), and
  `VersionDAW` judges a single Version file by extension. Every other
  package that needs to know what a "project" is depends on this one; adding
  a DAW starts with a new entry in `detectors`.

- **`internal/backup`** — turns a loaded config + optional Directory names
  into a list of `Job{Dir, LocalDir, RemoteDir}` via `BuildJobs` (one per
  Directory per Remote), then runs each through `rclone copy` (never `sync`
  — copy only adds/updates on the remote, so a local mistake can't destroy
  the remote copy too). A Directory with no Remotes is skipped, or an error
  if named explicitly. `ListArgs`/`ListRemote` list a remote's files
  (`rclone lsf`, read-only) for `demos --remote`.

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

- **`internal/demolist`** — finds Demos by name. `GroupPaths` groups
  relative file paths into Demos (audio `.wav .aiff .aif .mp3` only; a
  `.wav`/`.mp3` pair is one Demo; `.asd` ignored), `ListDemos` feeds it from
  a local walk, and `Merge` combines the lists from any number of places
  (local, then each Remote), so a Demo's `Location` is every place it was
  found — by name only, never by content. The remote listing itself
  comes from `backup.ListRemote`, so this package never shells out.
  Demo-to-Version linking was removed for now; see
  `docs/adr/0003-demos-link-to-versions-by-filename.md` (parked).

- **`internal/demos`** — converts `aiff`/`wav` under the demos directory to
  320k CBR mp3 via `ffmpeg` and deletes the original **only** after
  verifying the output exists and is non-empty (`Convert` then `os.Remove`
  in `ConvertAndCleanup`); a failed conversion never triggers a delete.

- **`internal/peaks`** — backs `find-peaks`. Decodes uncompressed WAV
  (PCM/float, incl. `WAVE_FORMAT_EXTENSIBLE`) and AIFF/AIFF-C (`NONE`,
  `twos`, `sowt`, `fl32`, `fl64`) in pure Go, streaming, with no `ffmpeg`
  dependency; the container is detected by magic bytes, and chunks may come
  in any order. A Peak is one excursion over the threshold (max across
  channels): crossings within `Hold` (100ms) merge into one, reported at
  its loudest sample. `Bar` treats BPM as quarter notes per minute, as Live
  does, so a 6/8 bar is three quarter notes.

## Conventions worth preserving when extending

- Favor structural detection (a glob, a required file's presence) over
  maintained exclude-lists — this shows up in `discovery.DiscoverProjects`
  (any dir with a top-level `.als`) and `demos` (any `aiff`/`wav` under the
  demos directory), and is a deliberate project-wide preference, not
  incidental.
- Destructive operations are conservative by default: `backup` uses
  `rclone copy` never `sync`; `find-orphans --quarantine` moves rather than
  deletes and skips anything not confidently orphaned; `convert-demos` only
  deletes a source file after verifying its converted replacement; `collect`
  never overwrites the input `.als`. Match this bias — prefer "leave it and
  report" over "delete/overwrite" — for any new mutating command.
- Keep the config surface small: one file of named Directories, each a
  path and its Remotes, with `projects` and `demos` as the only reserved
  names. Don't add kinds, roles or registries without revisiting
  `docs/adr/0004-named-directories-with-remotes.md` (and `0001`).
- Each `internal/` package is independently testable against fixture data
  (see `internal/*/*_test.go`) without needing `rclone`/`ffmpeg` installed;
  keep new packages structured so their core logic doesn't require shelling
  out in tests (inject the lookup, as `configcheck.Env` does).
- `mise run check` must pass before committing.

## Git

This repo overrides the global "never run state-changing git commands"
rule from `~/dotfiles/claude/AGENTS.md` — that global rule still applies
everywhere else. In this repo only:

- Claude may `git add` / `commit` / `push` autonomously, on any branch
  **except `main`**. Before an autonomous commit, `mise run check` must
  pass. If it fails, fix the cause or stop and report; never bypass the
  hooks (`--no-verify`).
- Commit messages: match the existing history's style — terse, lowercase,
  no type-prefixes (e.g. `backup ableton user library`, not
  `feat: backup user library`).
- Once work on a branch is ready, Claude may open a `dev` → `main` PR
  autonomously via `gh pr create` — no need to ask first. PR title follows
  the same terse commit style; PR body is a short Summary + Test plan.
- Merging into `main` stays manual, and Claude never pushes to `main`.
  `main` has no branch protection, and it is the source for releases.
- Creating or pushing a `v*` tag stays manual. A tag push triggers
  `release.yml`, which runs goreleaser and publishes a GitHub release
  (`contents: write`) with no review step.
- Everything else the global rule covers — merge, rebase, reset, checkout
  (branch-switch or file-restore), stash, branch create-or-delete, and tag
  delete — stays manual here too.
