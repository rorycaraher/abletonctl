// Command abletonctl manages a multi-artist Ableton Live production
// workspace: discovering projects, backing up production/demo directories
// to configurable rclone remotes, and finding unreferenced samples.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rorycaraher/ableton-framework/internal/backup"
	"github.com/rorycaraher/ableton-framework/internal/config"
	"github.com/rorycaraher/ableton-framework/internal/demos"
	"github.com/rorycaraher/ableton-framework/internal/discovery"
	"github.com/rorycaraher/ableton-framework/internal/samples"
	"github.com/rorycaraher/ableton-framework/internal/tracks"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "backup":
		err = runBackup(os.Args[2:])
	case "projects":
		err = runProjects(os.Args[2:])
	case "prune-samples":
		err = runPruneSamples(os.Args[2:])
	case "convert-demos":
		err = runConvertDemos(os.Args[2:])
	case "tracks":
		err = runTracks(os.Args[2:])
	case "track":
		err = runTrack(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "abletonctl:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `abletonctl - manage an Ableton Live production workspace

Usage:
  abletonctl backup [--artist NAME] [--role ROLE] [--dry-run]
  abletonctl backup --artist user-library [--dry-run]
  abletonctl projects [--artist NAME]
  abletonctl prune-samples <project-path> [--quarantine]
  abletonctl convert-demos [--artist NAME] [--role ROLE] [--dry-run]
  abletonctl tracks [--artist NAME]
  abletonctl track add <name> [--artist NAME] Key=Value...
  abletonctl track set <name> [--artist NAME] Key=Value...

Global:
  --registry PATH   registry file (default ~/.config/abletonctl/config.toml)
`)
}

func loadRegistry(registryFlag string) (*config.Registry, error) {
	path := registryFlag
	if path == "" {
		var err error
		path, err = config.DefaultRegistryPath()
		if err != nil {
			return nil, err
		}
	}
	return config.LoadRegistry(path)
}

func runBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	artist := fs.String("artist", "", "limit to one registered artist")
	role := fs.String("role", "", "limit to one role (e.g. production, demos)")
	dryRun := fs.Bool("dry-run", false, "pass --dry-run through to rclone")
	registryPath := fs.String("registry", "", "path to registry config")
	if err := fs.Parse(args); err != nil {
		return err
	}

	reg, err := loadRegistry(*registryPath)
	if err != nil {
		return err
	}

	jobs, err := backup.BuildJobs(reg, *artist, *role)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		fmt.Println("no backup jobs matched")
		return nil
	}

	for _, j := range jobs {
		fmt.Printf("==> [%s/%s] %s -> %s\n", j.Artist, j.Role, j.LocalDir, j.RemoteDir)
		if err := backup.Run(j, *dryRun, os.Stdout, os.Stderr); err != nil {
			return err
		}
	}
	return nil
}

func runProjects(args []string) error {
	fs := flag.NewFlagSet("projects", flag.ExitOnError)
	artist := fs.String("artist", "", "limit to one registered artist")
	registryPath := fs.String("registry", "", "path to registry config")
	if err := fs.Parse(args); err != nil {
		return err
	}

	reg, err := loadRegistry(*registryPath)
	if err != nil {
		return err
	}

	artists := reg.ArtistNames()
	if *artist != "" {
		if _, ok := reg.Artists[*artist]; !ok {
			return fmt.Errorf("artist %q is not registered", *artist)
		}
		artists = []string{*artist}
	}

	for _, name := range artists {
		root := reg.Artists[name]
		fmt.Printf("%s (%s)\n", name, root)

		prodDirs, err := discovery.DiscoverProductionDirs(root)
		if err != nil {
			return err
		}
		for _, dir := range prodDirs {
			projects, err := discovery.DiscoverProjects(dir)
			if err != nil {
				return err
			}
			fmt.Printf("  %s\n", dir)
			for _, p := range projects {
				fmt.Printf("    - %s\n", p.Name)
			}
		}
	}
	return nil
}

func runPruneSamples(args []string) error {
	// Parsed by hand rather than via flag.FlagSet: that package stops
	// recognizing flags after the first positional argument, but the
	// natural invocation here is `prune-samples <path> --quarantine`.
	var quarantine bool
	var positional []string
	for _, a := range args {
		switch a {
		case "--quarantine":
			quarantine = true
		case "-h", "--help":
			fmt.Println("usage: abletonctl prune-samples <project-path> [--quarantine]")
			return nil
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		return fmt.Errorf("usage: abletonctl prune-samples <project-path> [--quarantine]")
	}
	projectPath, err := filepath.Abs(positional[0])
	if err != nil {
		return err
	}

	siblings, err := discovery.DiscoverProjects(filepath.Dir(projectPath))
	if err != nil {
		return err
	}
	var project *discovery.Project
	for i := range siblings {
		if siblings[i].Path == projectPath {
			project = &siblings[i]
			break
		}
	}
	if project == nil {
		return fmt.Errorf("%s does not look like an Ableton project (no top-level .als file)", projectPath)
	}

	results, err := samples.Scan(*project)
	if err != nil {
		return err
	}

	var orphans, uncertain int
	var orphanBytes int64
	for _, r := range results {
		switch r.Status {
		case samples.Orphan:
			orphans++
			orphanBytes += r.Size
			fmt.Printf("orphan     %10d  %s\n", r.Size, r.RelPath)
		case samples.Uncertain:
			uncertain++
			fmt.Printf("uncertain  %10d  %s\n", r.Size, r.RelPath)
		}
	}
	fmt.Printf("\n%d orphaned files (%d bytes), %d uncertain matches\n", orphans, orphanBytes, uncertain)

	if quarantine {
		moved, err := samples.Quarantine(*project, results)
		if err != nil {
			return err
		}
		fmt.Printf("quarantined %d files into %s/_unreferenced/\n", len(moved), project.Path)
	}
	return nil
}

func runConvertDemos(args []string) error {
	fs := flag.NewFlagSet("convert-demos", flag.ExitOnError)
	artist := fs.String("artist", "", "limit to one registered artist")
	role := fs.String("role", "demos", "role whose matched directories to convert")
	dryRun := fs.Bool("dry-run", false, "report what would be converted/deleted without doing it")
	registryPath := fs.String("registry", "", "path to registry config")
	if err := fs.Parse(args); err != nil {
		return err
	}

	reg, err := loadRegistry(*registryPath)
	if err != nil {
		return err
	}

	artists := reg.ArtistNames()
	if *artist != "" {
		if _, ok := reg.Artists[*artist]; !ok {
			return fmt.Errorf("artist %q is not registered", *artist)
		}
		artists = []string{*artist}
	}

	var totalConverted, totalFailed int
	for _, name := range artists {
		root := reg.Artists[name]
		artistCfg, err := config.LoadArtistConfig(root)
		if err != nil {
			return fmt.Errorf("artist %q: %w", name, err)
		}
		roleCfg, ok := artistCfg.Roles[*role]
		if !ok {
			if *artist != "" {
				return fmt.Errorf("artist %q has no role %q", name, *role)
			}
			continue
		}

		dirs, err := discovery.MatchRoleDirs(root, roleCfg.Glob)
		if err != nil {
			return fmt.Errorf("artist %q role %q: %w", name, *role, err)
		}

		for _, dir := range dirs {
			outcomes, err := demos.ConvertAndCleanup(dir, *dryRun, os.Stdout, os.Stderr)
			if err != nil {
				return fmt.Errorf("artist %q: %w", name, err)
			}
			for _, o := range outcomes {
				switch {
				case o.Err != nil:
					totalFailed++
					fmt.Printf("[%s] FAILED  %s: %v\n", name, o.Source, o.Err)
				case *dryRun:
					fmt.Printf("[%s] would convert %s -> %s and delete original\n", name, o.Source, o.Mp3)
				default:
					totalConverted++
					fmt.Printf("[%s] converted %s -> %s, removed original\n", name, o.Source, o.Mp3)
				}
			}
		}
	}

	if !*dryRun {
		fmt.Printf("\n%d converted, %d failed\n", totalConverted, totalFailed)
	}
	if totalFailed > 0 {
		return fmt.Errorf("%d file(s) failed to convert; originals left in place", totalFailed)
	}
	return nil
}

func runTracks(args []string) error {
	fs := flag.NewFlagSet("tracks", flag.ExitOnError)
	artist := fs.String("artist", "", "limit to one registered artist")
	registryPath := fs.String("registry", "", "path to registry config")
	if err := fs.Parse(args); err != nil {
		return err
	}

	reg, err := loadRegistry(*registryPath)
	if err != nil {
		return err
	}

	artists := reg.ArtistNames()
	if *artist != "" {
		if _, ok := reg.Artists[*artist]; !ok {
			return fmt.Errorf("artist %q is not registered", *artist)
		}
		artists = []string{*artist}
	}

	for _, name := range artists {
		root := reg.Artists[name]
		path := tracks.CatalogPath(root)

		cat, err := tracks.Load(path)
		if os.IsNotExist(err) {
			fmt.Printf("%s: no track catalog at %s\n", name, path)
			continue
		}
		if err != nil {
			return err
		}

		known, err := knownProjectNames(root)
		if err != nil {
			return err
		}

		fmt.Printf("%s (%s)\n", name, path)
		for _, row := range cat.Rows {
			track := row[tracks.TrackColumn]
			line := fmt.Sprintf("  %-40s %-15s", track, row["Status"])
			if !known[track] {
				line += "  [no project folder]"
			}
			fmt.Println(line)
		}
	}
	return nil
}

// knownProjectNames returns the set of discovered project folder names
// across every PRODUCTION-* directory under an artist root, used to flag
// catalog rows with no corresponding folder on disk (expected for
// Idea-stage tracks, informational otherwise).
func knownProjectNames(artistRoot string) (map[string]bool, error) {
	known := map[string]bool{}
	prodDirs, err := discovery.DiscoverProductionDirs(artistRoot)
	if err != nil {
		return nil, err
	}
	for _, dir := range prodDirs {
		projects, err := discovery.DiscoverProjects(dir)
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			known[p.Name] = true
		}
	}
	return known, nil
}

func runTrack(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: abletonctl track <add|set> <name> [--artist NAME] Key=Value...")
	}
	switch args[0] {
	case "add":
		return runTrackMutate(args[1:], true)
	case "set":
		return runTrackMutate(args[1:], false)
	case "-h", "--help":
		fmt.Println("usage: abletonctl track <add|set> <name> [--artist NAME] Key=Value...")
		return nil
	default:
		return fmt.Errorf("unknown track subcommand %q (want add or set)", args[0])
	}
}

func runTrackMutate(args []string, isAdd bool) error {
	// Parsed by hand rather than via flag.FlagSet, for the same reason as
	// prune-samples: the natural invocation mixes a positional name with
	// trailing Key=Value pairs, which FlagSet can't express.
	var artist, registryPath, name string
	fields := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--artist":
			i++
			if i >= len(args) {
				return fmt.Errorf("--artist requires a value")
			}
			artist = args[i]
		case a == "--registry":
			i++
			if i >= len(args) {
				return fmt.Errorf("--registry requires a value")
			}
			registryPath = args[i]
		case strings.Contains(a, "="):
			parts := strings.SplitN(a, "=", 2)
			fields[parts[0]] = parts[1]
		case name == "":
			name = a
		default:
			return fmt.Errorf("unexpected argument %q", a)
		}
	}
	if name == "" {
		return fmt.Errorf("usage: abletonctl track <add|set> <name> [--artist NAME] Key=Value...")
	}

	reg, err := loadRegistry(registryPath)
	if err != nil {
		return err
	}
	root, err := resolveArtistRoot(reg, artist)
	if err != nil {
		return err
	}

	path := tracks.CatalogPath(root)
	cat, err := tracks.Load(path)
	if os.IsNotExist(err) {
		if !isAdd {
			return fmt.Errorf("no track catalog at %s (use 'track add' to create it)", path)
		}
		cat = tracks.New()
	} else if err != nil {
		return err
	}

	if isAdd {
		err = cat.Add(name, fields)
	} else {
		err = cat.Set(name, fields)
	}
	if err != nil {
		return err
	}

	if err := tracks.Save(path, cat); err != nil {
		return err
	}
	verb := "updated"
	if isAdd {
		verb = "added"
	}
	fmt.Printf("%s: %s\n", verb, name)
	return nil
}

// resolveArtistRoot resolves the artist to mutate a track catalog for:
// the explicit --artist if given, or the sole registered artist if there's
// only one. Ambiguous with more than one artist and no --artist given.
func resolveArtistRoot(reg *config.Registry, artist string) (string, error) {
	if artist != "" {
		root, ok := reg.Artists[artist]
		if !ok {
			return "", fmt.Errorf("artist %q is not registered", artist)
		}
		return root, nil
	}
	names := reg.ArtistNames()
	switch len(names) {
	case 0:
		return "", fmt.Errorf("no artists registered")
	case 1:
		return reg.Artists[names[0]], nil
	default:
		return "", fmt.Errorf("multiple artists registered; specify --artist")
	}
}
