package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/collect"
)

func newCollectCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "collect <path-to-.als | directory with --all>",
		Short: "Collect external file references into a project (scriptable Collect All and Save)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return runCollectAll(args[0])
			}
			return runCollectOne(args[0])
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "treat the argument as a directory and collect every top-level .als in it")
	return cmd
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
