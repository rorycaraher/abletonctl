package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/studioctl/internal/discovery"
	"github.com/rorycaraher/studioctl/internal/samples"
)

func newFindOrphansCmd() *cobra.Command {
	var quarantine bool
	cmd := &cobra.Command{
		Use:   "find-orphans <project-path>",
		Short: "Find (and optionally quarantine) samples no Version of a Project references",
		Args:  cobra.ExactArgs(1),
		// Directories only: the argument is a project folder.
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveFilterDirs
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFindOrphans(args[0], quarantine)
		},
	}
	cmd.Flags().BoolVar(&quarantine, "quarantine", false, "move orphaned files into the project's _unreferenced/ folder")
	return cmd
}

func runFindOrphans(path string, quarantine bool) error {
	projectPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	project, ok, err := discovery.LoadProject(projectPath)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s isn't a recognised Project (supported: %s)", projectPath, discovery.Supported)
	}
	switch project.DAW {
	case discovery.Ableton:
		return findAbletonOrphans(project, quarantine)
	default:
		return fmt.Errorf("find-orphans doesn't support %s Projects yet", project.DAW)
	}
}

func findAbletonOrphans(project discovery.Project, quarantine bool) error {
	results, err := samples.Scan(project)
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
		moved, err := samples.Quarantine(project, results)
		if err != nil {
			return err
		}
		fmt.Printf("quarantined %d files into %s/_unreferenced/\n", len(moved), project.Path)
	}
	return nil
}
