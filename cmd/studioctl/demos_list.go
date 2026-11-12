package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rorycaraher/studioctl/internal/backup"
	"github.com/rorycaraher/studioctl/internal/config"
	"github.com/rorycaraher/studioctl/internal/demolist"
)

func newDemosCmd() *cobra.Command {
	var configPath string
	var remote bool
	cmd := &cobra.Command{
		Use:   "demos",
		Short: "List Demos in the demos Directory",
		Long: `List every Demo in the demos Directory: audio files grouped by name, so a
.wav and its .mp3 are one Demo.

With --remote, the Demos on each of the demos Directory's remotes are
included too (listed with rclone, so it needs rclone and network access).
Each Demo then shows where it was found: locally and on which remotes, with
"·" where it is missing. Places are decided by name only, not by comparing
content, so being found in two places does not mean the files are identical.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return err
			}
			demos, remotes, err := loadDemos(cfg, remote)
			if err != nil {
				return err
			}
			writeDemos(os.Stdout, demos, remotes)
			return nil
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "also include Demos on the demos Directory's remotes (uses rclone and the network)")
	addConfigFlag(cmd, &configPath)
	return cmd
}

// loadDemos lists the local Demos, and with remote also those on every
// Remote of the demos Directory, merged into one list; it returns the
// Remotes it listed. A Remote that can't be listed is an error: silently
// leaving it out would read as "not backed up there".
func loadDemos(cfg *config.Config, remote bool) ([]demolist.Demo, []string, error) {
	dir, err := requireDir(cfg, config.Demos)
	if err != nil {
		return nil, nil, err
	}
	local, err := demolist.ListDemos(dir.Path)
	if err != nil {
		return nil, nil, err
	}
	if !remote {
		return local, nil, nil
	}
	if len(dir.Remotes) == 0 {
		return nil, nil, fmt.Errorf("dirs.%s has no remotes", config.Demos)
	}
	lists := [][]demolist.Demo{local}
	for _, r := range dir.Remotes {
		paths, err := backup.ListRemote(r)
		if err != nil {
			return nil, nil, err
		}
		lists = append(lists, demolist.GroupPaths(paths, r))
	}
	return demolist.Merge(lists...), dir.Remotes, nil
}

// remoteLabels gives each Remote a short column label: its rclone remote
// name, or the full destination when two Remotes share a name.
func remoteLabels(remotes []string) []string {
	count := map[string]int{}
	for _, r := range remotes {
		name, _, _ := strings.Cut(r, ":")
		count[name]++
	}
	labels := make([]string, len(remotes))
	for i, r := range remotes {
		name, _, _ := strings.Cut(r, ":")
		if count[name] > 1 {
			labels[i] = r
		} else {
			labels[i] = name
		}
	}
	return labels
}

// writeDemos lists the Demos. With remotes, each line starts with a column
// per place (local, then each Remote) showing where the Demo was found.
func writeDemos(w io.Writer, demos []demolist.Demo, remotes []string) {
	places := append([]string{demolist.Local}, remotes...)
	labels := append([]string{demolist.Local}, remoteLabels(remotes)...)

	var b strings.Builder
	counts := make([]int, len(places))
	var gaps int
	for _, d := range demos {
		b.WriteString("  ")
		if len(remotes) > 0 {
			for i, p := range places {
				cell := "·"
				if d.In(p) {
					cell = labels[i]
					counts[i]++
				}
				fmt.Fprintf(&b, "%s ", placeStyle(d.In(p)).Render(pad(cell, lipgloss.Width(labels[i]))))
			}
			for _, r := range remotes {
				if !d.In(r) {
					gaps++
					break
				}
			}
		}
		fmt.Fprintf(&b, "%s %s\n", path.Join(d.Dir, d.Name), dimStyle.Render("["+strings.Join(d.Formats, ",")+"]"))
	}
	unit := "demos"
	if len(demos) == 1 {
		unit = "demo"
	}
	fmt.Fprintf(&b, "\n%d %s\n", len(demos), unit)
	if len(remotes) > 0 {
		parts := make([]string, len(places))
		for i := range places {
			parts[i] = fmt.Sprintf("%d %s", counts[i], labels[i])
		}
		fmt.Fprintf(&b, "%s (by name)\n", strings.Join(parts, ", "))
		fmt.Fprintf(&b, "%d not on every remote\n", gaps)
	}
	_, _ = io.WriteString(w, b.String())
}

func pad(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}
