# Backlog

Ideas raised while designing `abletonctl` but deliberately left out of the
first build, to keep it to three pillars: namespace/project discovery,
backup, and sample-orphan finding.

- **Cross-project duplicate sample detection.** Hash every file under each
  project's `Samples/` and report identical files duplicated across
  projects - candidates for a shared sample library instead of N copies.
- **Disk usage / project size report.** Per-project and per-namespace size
  breakdown, useful for deciding what to archive off the primary disk.
- **Automated scheduling.** A `launchd` plist to run `abletonctl backup`
  daily, plus a `status` subcommand that records/reports last-successful
  backup time per role, so a silently broken schedule is visible at a
  glance rather than discovered at restore time.
- **Archiving old production years.** A command to move a closed-out
  `PRODUCTION-YYYY` to a colder storage tier/remote once a new year has
  started, rather than treating all years identically forever.
- **BPM/key tagging report.** Parse tempo/key metadata out of `.als` files
  across a namespace and produce a searchable index.
- **Render-naming / demos cross-referencing.** Link `demos/*.aiff` renders
  back to the project that produced them (e.g. by naming convention or
  embedded metadata), to answer "which project made this file" later.
- **Restore verification.** A command that spot-checks a remote against
  local state (existence + size, not full re-download) to catch a backup
  that's silently been failing.
- **Lightweight `.als` versioning.** `.als` is gzipped XML, which diffs
  poorly as-is; a wrapper that keeps a git history of the *decompressed*
  XML (without touching the real project file) could make meaningful
  diffs/history possible.
- **Premaster directory** another directory beside demos for premasters,
  and a script that uses ffmpeg to check if premaster files are lossless,
  have headroom, ideal bit depth/sample rate.
- **Cloud-sync corruption check.** Warn when a registered production
  directory sits inside a Dropbox/Google Drive/iCloud-synced path -
  Ableton writing (autosave or manual save) while a sync client is
  mid-upload is a known way to corrupt or fork an `.als`.
- **Recover from Ableton's own `Backup/` folder.** Ableton already writes
  a timestamped backup on every save, but most users don't know it's
  there. A `recover` command listing/restoring from a project's `Backup/`
  by timestamp turns an existing safety net into something people
  actually use after a crash or bad save.
- **Disk-space preflight.** Warn if free space on the volume a project
  records to is low - Ableton can silently fail to record or hang when
  the disk fills mid-session, with no clear warning beforehand.
- **Reverse orphan check.** `prune-samples` finds samples no project
  references anymore; the inverse - samples a project's `.als` depends on
  that live outside its `Samples/` folder (Desktop, Downloads, an
  external drive) - is the other half of the problem, since those break
  the moment the source location changes or the drive is unplugged.
- **Plugin dependency report.** Parse third-party plugin references out
  of a project's `.als` XML so a missing/mismatched plugin on another
  machine (silently substituted or greyed out, changing the sound) can be
  caught before handoff instead of discovered by ear.
- **Health-check aggregator (`doctor`).** A single command that runs every
  standalone check above - disk-space preflight, cloud-sync hazard, orphan
  samples, plugin drift, backup staleness - and reports pass/fail per
  check, so there's one place to run before/after a session instead of
  remembering which command covers which risk.
- **Pipeline gates on the track catalog.** The freeform `Status` column
  already behaves like a state machine (Idea -> WIP -> Mixdown ->
  Mastered); `track set` could refuse a promotion - e.g. to `Mastered` -
  if `prune-samples` reports orphans or the artist's last backup is
  stale, turning the column into an actual gate instead of just a label.
- **Backup freshness as an SLO.** An extension of "Automated scheduling"
  above: instead of only recording last-successful-backup time, define a
  target staleness threshold per role (e.g. no artist should go >48h
  without a successful backup) and report breaches explicitly, rather
  than requiring a manual glance at a timestamp to notice one.
- **Golden template pipeline.** Scaffold a new project from a registered
  Ableton template (e.g. the User Library's `Templates/` folder) and
  register it in one step - `abletonctl new <name> --template X` - so
  every project starts from a known-good baseline instead of drifting
  from whatever the last project happened to look like.
- **Recreate Collect All and Save** functionality into a scriptable
  solution.
