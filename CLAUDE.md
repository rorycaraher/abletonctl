# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`abletonctl` is a Go CLI for managing a **single** Ableton Live production
workspace: listing projects, backing up the projects and demos directories to
rclone remotes, finding orphaned samples, collecting external file
references into a project (a scriptable "Collect All and Save"), converting
rendered demos to mp3, and validating its own config file.

- `README.md` — user-facing docs: install, configure, usage.
- `CONTEXT.md` — domain vocabulary (Projects directory, Demos directory,
  Project, Backup target, Orphan, Uncertain) and terms to avoid. Use these
  terms in code, docs, and commit messages.
- `docs/adr/` — why the structure is what it is. Read
  `0001-single-workspace-mvp.md` before reintroducing anything multi-artist:
  the artist registry, per-artist roles, `PRODUCTION-*` grouping, the track
  catalog CSV, and User Library backup were removed on purpose and live on a
  separate branch.
- `IDEAS.md` — deliberately-deferred features.

## Commands

```sh
mise install        # pinned toolchain: go, golangci-lint, pre-commit, gitleaks, actionlint
mise run check      # what CI runs: gofmt check, go vet, golangci-lint, go test
mise run build      # go build -o abletonctl ./cmd/abletonctl
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
  golangci-lint + gitleaks; `go test` on pre-push.
- `.github/workflows/ci.yml` runs `mise run check`, `actionlint` and
  `gitleaks` on PRs and pushes to `main`.
- `.github/workflows/release.yml` runs goreleaser (`.goreleaser.yaml`) on
  `v*` tags. goreleaser injects `main.version` via ldflags.

## Architecture

Single-binary CLI built on [cobra](https://github.com/spf13/cobra). `main.go`
only declares `version` and runs `newRootCmd()` (`root.go`), printing any
returned error as `abletonctl: <err>` and exiting 1. Each command lives in its
own file in `cmd/abletonctl/` as a `newXCmd()` constructor registered in
`newRootCmd()`:

| File | Command(s) |
|---|---|
| `backup.go` | `backup [--target projects\|demos] [--dry-run]` |
| `projects.go` | `projects [--remote]` (lists projects with `.als` and linked-Demo counts) |
| `config.go` | `config check` |
| `orphans.go` | `find-orphans <project-path> [--quarantine]` |
| `collect.go` | `collect <.als>` / `collect --all <dir>` |
| `demos.go` | `convert-demos [--dry-run]` |
| `demos_list.go` | `demos [--project <path>] [--remote]` (Demo to Version links) |
| `style.go` | lipgloss styles (no command) |

Conventions for the command layer:

- `SilenceUsage`/`SilenceErrors` are set on the root; return errors from
  `RunE` rather than printing and exiting.
- `--config` is added only to commands that read the config file, via
  `addConfigFlag`. `find-orphans` and `collect` take explicit paths instead.
- Long flags are `--double-dash` only (pflag); flags may appear before or
  after positional arguments.
- Shell completion (`abletonctl completion zsh|bash|fish|powershell`) is
  cobra's built-in. Give flags with a fixed value set a
  `RegisterFlagCompletionFunc` and path arguments a `ValidArgsFunction`.
- Styling (lipgloss) degrades to plain text when stdout isn't a terminal or
  `NO_COLOR` is set. Keep piped output free of escape codes.

All actual logic lives in `internal/`, one package per concern; the command
files do only argument handling, orchestration, and output formatting:

- **`internal/config`** — loads the single flat config file
  (`~/.config/abletonctl/config.toml`): `projects_dir`, `demos_dir`, and a
  rclone remote for each (`projects_remote`, `demos_remote`). Pure data
  loading. See `examples/config.toml`.

- **`internal/configcheck`** — backs `config check`. Re-decodes the config
  with TOML metadata to flag unknown keys (typos), and checks that the
  directories exist, remotes have a `name:` prefix and exist in
  `rclone listremotes`, and `rclone`/`ffmpeg` are on `PATH`. Returns a list
  of `ok`/`warn`/`FAIL` results rather than stopping at the first problem.
  Machine lookups go through an injectable `Env`, so tests need neither
  binary.

- **`internal/discovery`** — structural filesystem scanning with no
  exclude-lists: `DiscoverProjects` finds the direct children of a directory
  that contain a top-level `.als` file (Ableton's auto-generated `Backup/`
  folder is never scanned). Every other package that needs to know what a
  "project" is depends on this one.

- **`internal/backup`** — turns a loaded config + optional target filter
  into a list of `Job{Target, LocalDir, RemoteDir}` via `BuildJobs`, then runs
  each through `rclone copy` (never `sync` — copy only adds/updates on the
  remote, so a local mistake can't destroy the remote copy too). There are
  exactly two fixed targets, projects and demos; a target missing its
  directory or remote is skipped, or an error if explicitly requested.
  `ListArgs`/`ListRemote` list a remote's files (`rclone lsf`, read-only) for
  `demos --remote`.

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

- **`internal/demolink`** — relates Demos to Versions by filename, purely
  from names (nothing stored). `Parse` splits a Demo name on the first `__`;
  `GroupPaths` groups relative file paths into Demos (audio `.wav .aiff .aif
  .mp3` only; a `.wav`/`.mp3` pair is one Demo; `.asd` ignored), `ListDemos`
  feeds it from a local walk, and `Merge` combines local and remote Demos,
  tagging each `Location` (`Local`/`Remote`/`Both`, by name only, never by
  content). The remote listing itself comes from `backup.ListRemote`, so this
  package never shells out. `Resolve`
  indexes every Project's `.als` slugs and classifies each Demo as `Linked`,
  `Ambiguous` (slug in 2+ Projects), `Dangling` (has `__`, no match) or
  `Unlinked`. Matching is exact and case-sensitive, never prefix-based.
  See `docs/adr/0003-demos-link-to-versions-by-filename.md`.

- **`internal/demos`** — converts `aiff`/`wav` under the demos directory to
  320k CBR mp3 via `ffmpeg` and deletes the original **only** after
  verifying the output exists and is non-empty (`Convert` then `os.Remove`
  in `ConvertAndCleanup`); a failed conversion never triggers a delete.

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
- Keep the config surface small. One flat file, two directories, two
  remotes; don't add named roles or registries back without revisiting
  `docs/adr/0001-single-workspace-mvp.md`.
- Each `internal/` package is independently testable against fixture data
  (see `internal/*/*_test.go`) without needing `rclone`/`ffmpeg` installed;
  keep new packages structured so their core logic doesn't require shelling
  out in tests (inject the lookup, as `configcheck.Env` does).
- `mise run check` must pass before committing. The git state is the
  user's to change — see their global instructions.
