// Command abletonctl manages a single Ableton Live production workspace:
// discovering projects, finding unreferenced samples, collecting external
// file references into a project, converting rendered demos to mp3, and
// backing up the projects and demos directories to rclone remotes.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rorycaraher/abletonctl/internal/backup"
	"github.com/rorycaraher/abletonctl/internal/collect"
	"github.com/rorycaraher/abletonctl/internal/config"
	"github.com/rorycaraher/abletonctl/internal/demos"
	"github.com/rorycaraher/abletonctl/internal/discovery"
	"github.com/rorycaraher/abletonctl/internal/samples"
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
	case "find-orphans":
		err = runFindOrphans(os.Args[2:])
	case "collect":
		err = runCollect(os.Args[2:])
	case "convert-demos":
		err = runConvertDemos(os.Args[2:])
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
  abletonctl backup [--target projects|demos] [--dry-run]
  abletonctl projects
  abletonctl find-orphans <project-path> [--quarantine]
  abletonctl collect <path-to-.als>
  abletonctl collect --all <directory>
  abletonctl convert-demos [--dry-run]

Global (all except find-orphans/collect, which take explicit paths):
  --config PATH   config file (default ~/.config/abletonctl/config.toml)
`)
}

func loadConfig(configFlag string) (*config.Config, error) {
	path := configFlag
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	return config.Load(path)
}

func runBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	target := fs.String("target", "", "limit to one backup target (projects or demos)")
	dryRun := fs.Bool("dry-run", false, "pass --dry-run through to rclone")
	configPath := fs.String("config", "", "path to config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}

	jobs, err := backup.BuildJobs(cfg, *target)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		fmt.Println("no backup targets configured")
		return nil
	}

	for _, j := range jobs {
		fmt.Printf("==> [%s] %s -> %s\n", j.Target, j.LocalDir, j.RemoteDir)
		if err := backup.Run(j, *dryRun, os.Stdout, os.Stderr); err != nil {
			return err
		}
	}
	return nil
}

func runProjects(args []string) error {
	fs := flag.NewFlagSet("projects", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	if cfg.ProjectsDir == "" {
		return fmt.Errorf("projects_dir is not configured")
	}

	projects, err := discovery.DiscoverProjects(cfg.ProjectsDir)
	if err != nil {
		return err
	}
	fmt.Println(cfg.ProjectsDir)
	for _, p := range projects {
		fmt.Printf("  - %s\n", p.Name)
	}
	return nil
}

func runFindOrphans(args []string) error {
	// Parsed by hand rather than via flag.FlagSet: that package stops
	// recognizing flags after the first positional argument, but the
	// natural invocation here is `find-orphans <path> --quarantine`.
	var quarantine bool
	var positional []string
	for _, a := range args {
		switch a {
		case "--quarantine":
			quarantine = true
		case "-h", "--help":
			fmt.Println("usage: abletonctl find-orphans <project-path> [--quarantine]")
			return nil
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		return fmt.Errorf("usage: abletonctl find-orphans <project-path> [--quarantine]")
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

func runCollect(args []string) error {
	// Parsed by hand rather than via flag.FlagSet, for the same reason as
	// find-orphans: --all takes its own positional (the directory to scan)
	// alongside the single-file form's positional.
	var all bool
	var positional []string
	for _, a := range args {
		switch a {
		case "--all":
			all = true
		case "-h", "--help":
			fmt.Println("usage: abletonctl collect <path-to-.als>")
			fmt.Println("       abletonctl collect --all <directory>")
			return nil
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		return fmt.Errorf("usage: abletonctl collect <path-to-.als>\n       abletonctl collect --all <directory>")
	}

	if all {
		return runCollectAll(positional[0])
	}
	return runCollectOne(positional[0])
}

func runCollectOne(alsPath string) error {
	result, err := collect.CollectOne(alsPath)
	if err != nil {
		return err
	}
	printCollectResult(alsPath, result)
	if result.Status == collect.Failed {
		return fmt.Errorf("collect failed for %s", alsPath)
	}
	return nil
}

// runCollectAll processes every top-level .als in dir, one file's failure
// never stopping the rest. The file list is a fixed snapshot taken before
// any processing starts.
func runCollectAll(dir string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "*.als"))
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		fmt.Printf("no .als files found in %s\n", dir)
		return nil
	}

	var failed int
	for _, alsPath := range matches {
		fmt.Printf("== %s ==\n", filepath.Base(alsPath))
		result, err := collect.CollectOne(alsPath)
		if err != nil {
			failed++
			fmt.Printf("  error: %v\n", err)
			fmt.Println()
			continue
		}
		printCollectResult(alsPath, result)
		if result.Status == collect.Failed {
			failed++
		}
		fmt.Println()
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d file(s) failed; see problems above", failed, len(matches))
	}
	return nil
}

func printCollectResult(alsPath string, result collect.Result) {
	switch result.Status {
	case collect.Failed:
		fmt.Println("Aborted -- nothing was copied or written. Problems found:")
		for _, p := range result.Problems {
			fmt.Printf("  - %s\n", p)
		}
	case collect.Nothing:
		fmt.Println("Nothing to collect -- no external SampleRef/MxPatchRef content found.")
	case collect.Collected:
		for _, line := range result.Report {
			fmt.Println(line)
		}
		fmt.Printf("Collected %d reference(s), %d unique file(s), into %s\n",
			result.Count, result.Unique, strings.Join(result.DestDirs, ", "))
		fmt.Printf("Wrote %s\n", result.Output)
	}
}

func runConvertDemos(args []string) error {
	fs := flag.NewFlagSet("convert-demos", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "report what would be converted/deleted without doing it")
	configPath := fs.String("config", "", "path to config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	if cfg.DemosDir == "" {
		return fmt.Errorf("demos_dir is not configured")
	}

	outcomes, err := demos.ConvertAndCleanup(cfg.DemosDir, *dryRun, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	var converted, failed int
	for _, o := range outcomes {
		switch {
		case o.Err != nil:
			failed++
			fmt.Printf("FAILED  %s: %v\n", o.Source, o.Err)
		case *dryRun:
			fmt.Printf("would convert %s -> %s and delete original\n", o.Source, o.Mp3)
		default:
			converted++
			fmt.Printf("converted %s -> %s, removed original\n", o.Source, o.Mp3)
		}
	}

	if !*dryRun {
		fmt.Printf("\n%d converted, %d failed\n", converted, failed)
	}
	if failed > 0 {
		return fmt.Errorf("%d file(s) failed to convert; originals left in place", failed)
	}
	return nil
}
