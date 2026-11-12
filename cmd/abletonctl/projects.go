package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/config"
	"github.com/rorycaraher/abletonctl/internal/discovery"
)

func newProjectsCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "List Projects in the projects Directory, with their .als file counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return err
			}
			dir, err := requireDir(cfg, config.Projects)
			if err != nil {
				return err
			}

			projects, err := discovery.DiscoverProjects(dir.Path)
			if err != nil {
				return err
			}

			fmt.Println(boldStyle.Render(dir.Path))
			for _, line := range projectLines(projects) {
				fmt.Println(line)
			}
			return nil
		},
	}
	addConfigFlag(cmd, &configPath)
	return cmd
}

// projectLines renders the Project rows as aligned columns: names padded to
// the widest, counts right-aligned so the numbers line up.
func projectLines(projects []discovery.Project) []string {
	var nameW, alsW int
	for _, p := range projects {
		nameW = max(nameW, lipgloss.Width(p.Name))
		alsW = max(alsW, len(strconv.Itoa(len(p.AlsFiles))))
	}

	lines := make([]string, 0, len(projects))
	for _, p := range projects {
		counts := fmt.Sprintf("%*d .als", alsW, len(p.AlsFiles))
		pad := strings.Repeat(" ", nameW-lipgloss.Width(p.Name))
		lines = append(lines, fmt.Sprintf("  - %s%s  %s", p.Name, pad, dimStyle.Render(counts)))
	}
	return lines
}
