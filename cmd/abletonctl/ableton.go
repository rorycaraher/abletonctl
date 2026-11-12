package main

import "github.com/spf13/cobra"

// newAbletonCmd groups the commands that only make sense for Ableton Live:
// they read .als files or Live's Project layout.
func newAbletonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ableton",
		Short: "Ableton Live tools: Projects, orphaned samples, Collect All and Save",
	}
	cmd.AddCommand(
		newProjectsCmd(),
		newFindOrphansCmd(),
		newCollectCmd(),
	)
	return cmd
}
