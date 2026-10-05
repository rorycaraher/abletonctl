package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/discovery"
	"github.com/rorycaraher/abletonctl/internal/samples"
)

func newFindOrphansCmd() *cobra.Command {
	var quarantine bool
	cmd := &cobra.Command{
		Use:   "find-orphans <project-path>",
		Short: "Find (and optionally quarantine) samples no .als references",
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
