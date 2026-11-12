package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/backup"
	"github.com/rorycaraher/abletonctl/internal/config"
	"github.com/rorycaraher/abletonctl/internal/demolist"
)

func newDemosCmd() *cobra.Command {
	var configPath string
	var remote bool
	cmd := &cobra.Command{
		Use:   "demos",
		Short: "List Demos in demos_dir",
		Long: `List every Demo under demos_dir: audio files grouped by name, so a .wav and
its .mp3 are one Demo.

With --remote, Demos on demos_remote are included too (listed with rclone, so
it needs rclone and network access). Each Demo then shows where it was found:
local, remote, or both. Locations are decided by name only, not by comparing
content, so "both" does not mean the files are identical.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return err
			}
			demos, err := loadDemos(cfg, remote)
			if err != nil {
				return err
			}
			writeDemos(os.Stdout, demos, remote)
			return nil
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "also include Demos on demos_remote (uses rclone and the network)")
	addConfigFlag(cmd, &configPath)
	return cmd
}

// loadDemos lists the local Demos, and with remote also those on
// demos_remote, merged into one list. A remote that can't be listed is an
// error: silently showing local-only would read as "not backed up".
func loadDemos(cfg *config.Config, remote bool) ([]demolist.Demo, error) {
	if cfg.DemosDir == "" {
		return nil, fmt.Errorf("demos_dir is not configured")
	}
	local, err := demolist.ListDemos(cfg.DemosDir)
	if err != nil {
		return nil, err
	}
	if !remote {
		return local, nil
	}
	if cfg.DemosRemote == "" {
		return nil, fmt.Errorf("demos_remote is not configured")
	}
	paths, err := backup.ListRemote(cfg.DemosRemote)
	if err != nil {
		return nil, err
	}
	return demolist.Merge(local, demolist.GroupPaths(paths, demolist.Remote)), nil
}

func writeDemos(w io.Writer, demos []demolist.Demo, showLocation bool) {
	var b strings.Builder
	locations := map[demolist.Location]int{}
	for _, d := range demos {
		locations[d.Location]++
		b.WriteString("  ")
		if showLocation {
			fmt.Fprintf(&b, "%s ", locationLabel(d.Location))
		}
		fmt.Fprintf(&b, "%s %s\n", path.Join(d.Dir, d.Name), dimStyle.Render("["+strings.Join(d.Formats, ",")+"]"))
	}
	unit := "demos"
	if len(demos) == 1 {
		unit = "demo"
	}
	fmt.Fprintf(&b, "\n%d %s\n", len(demos), unit)
	if showLocation {
		fmt.Fprintf(&b, "%d local, %d remote, %d both (by name)\n",
			locations[demolist.Local], locations[demolist.Remote], locations[demolist.Both])
	}
	_, _ = io.WriteString(w, b.String())
}
