package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/studioctl/internal/collect"
	"github.com/rorycaraher/studioctl/internal/discovery"
)

func newCollectCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "collect <version | project with --all>",
		Short: "Copy external files a Version references into its Project (a scriptable Collect All and Save)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return runCollectAll(args[0])
			}
			return runCollectOne(args[0])
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "treat the argument as a Project folder and collect every Version in it")
	return cmd
}

func runCollectOne(path string) error {
	switch daw := discovery.VersionDAW(path); daw {
	case discovery.Ableton:
		return collectAbleton(path)
	default:
		return fmt.Errorf("%s isn't a recognised Version (supported: %s)", path, discovery.Supported)
	}
}

func collectAbleton(alsPath string) error {
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

// runCollectAll processes every Version in the Project at dir, one file's
// failure never stopping the rest. The file list is a fixed snapshot taken
// before any processing starts.
func runCollectAll(dir string) error {
	project, ok, err := discovery.LoadProject(dir)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s isn't a recognised Project (supported: %s)", dir, discovery.Supported)
	}
	if project.DAW != discovery.Ableton {
		return fmt.Errorf("collect doesn't support %s Projects yet", project.DAW)
	}

	var failed int
	for _, alsPath := range project.Versions {
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
		return fmt.Errorf("%d of %d file(s) failed; see problems above", failed, len(project.Versions))
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
