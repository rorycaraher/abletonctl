package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/discovery"
)

func newProjectsCmd() *cobra.Command {
	var configPath string
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
			fmt.Println(boldStyle.Render(cfg.ProjectsDir))
			for _, p := range projects {
				fmt.Printf("  - %s %s\n", p.Name, dimStyle.Render(fmt.Sprintf("(%d .als)", len(p.AlsFiles))))
			}
			return nil
		},
	}
	addConfigFlag(cmd, &configPath)
	return cmd
}
