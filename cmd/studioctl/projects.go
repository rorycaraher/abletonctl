package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rorycaraher/studioctl/internal/config"
	"github.com/rorycaraher/studioctl/internal/discovery"
)

func newProjectsCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "List Projects in the projects Directory, with their Version counts",
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

			if len(projects) == 0 {
				fmt.Println(boldStyle.Render(dir.Path))
				fmt.Println(dimStyle.Render("  no Projects found (supported: " + discovery.Supported + ")"))
				return nil
			}
			// Every Project in the Directory is assumed to share a DAW.
			fmt.Println(boldStyle.Render(dir.Path) + "  " + dimStyle.Render(string(projects[0].DAW)))
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
	var nameW, countW int
	for _, p := range projects {
		nameW = max(nameW, lipgloss.Width(p.Name))
		countW = max(countW, len(strconv.Itoa(len(p.Versions))))
	}

	lines := make([]string, 0, len(projects))
	for _, p := range projects {
		counts := fmt.Sprintf("%*d %s", countW, len(p.Versions), plural(len(p.Versions), "version"))
		pad := strings.Repeat(" ", nameW-lipgloss.Width(p.Name))
		lines = append(lines, fmt.Sprintf("  - %s%s  %s", p.Name, pad, dimStyle.Render(counts)))
	}
	return lines
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
