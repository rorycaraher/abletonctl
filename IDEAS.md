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
