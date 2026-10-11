package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/demolink"
	"github.com/rorycaraher/abletonctl/internal/discovery"
)

func newProjectsCmd() *cobra.Command {
	var configPath string
	var remote bool
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "List projects under projects_dir, with their .als file counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
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

			// Demo counts are a convenience: a missing or unreadable demos_dir
			// should not stop the Project listing. With --remote the user asked
			// for the remote explicitly, so failing to list it is an error.
			var demoCounts map[string]int
			if cfg.DemosDir != "" || remote {
				demos, err := loadDemos(cfg, remote)
				switch {
				case err == nil:
					demoCounts = demolink.CountByProject(demolink.Resolve(demos, projects))
				case remote:
					return err
				default:
					fmt.Fprintln(os.Stderr, "abletonctl: skipping demo counts:", err)
				}
			}

			fmt.Println(boldStyle.Render(cfg.ProjectsDir))
			for _, line := range projectLines(projects, demoCounts) {
				fmt.Println(line)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "count Demos on demos_remote too (uses rclone and the network)")
	addConfigFlag(cmd, &configPath)
	return cmd
}

// projectLines renders the Project rows as aligned columns: names padded to
// the widest, counts right-aligned so the numbers line up. demoCounts (by
// Project path) is nil when Demo counts aren't available.
func projectLines(projects []discovery.Project, demoCounts map[string]int) []string {
	var nameW, alsW, demoW int
	for _, p := range projects {
		nameW = max(nameW, lipgloss.Width(p.Name))
		alsW = max(alsW, len(strconv.Itoa(len(p.AlsFiles))))
		demoW = max(demoW, len(strconv.Itoa(demoCounts[p.Path])))
	}

	lines := make([]string, 0, len(projects))
	for _, p := range projects {
		counts := fmt.Sprintf("%*d .als", alsW, len(p.AlsFiles))
		if demoCounts != nil {
			n := demoCounts[p.Path]
			unit := "demos"
			if n == 1 {
				unit = "demo"
			}
			counts += fmt.Sprintf("  %*d %s", demoW, n, unit)
		}
		pad := strings.Repeat(" ", nameW-lipgloss.Width(p.Name))
		lines = append(lines, fmt.Sprintf("  - %s%s  %s", p.Name, pad, dimStyle.Render(counts)))
	}
	return lines
}
