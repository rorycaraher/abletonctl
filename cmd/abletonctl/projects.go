package main

import (
	"fmt"
	"os"

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
			for _, p := range projects {
				var demoCount *int
				if demoCounts != nil {
					n := demoCounts[p.Path]
					demoCount = &n
				}
				fmt.Println(projectLine(p, demoCount))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "count Demos on demos_remote too (uses rclone and the network)")
	addConfigFlag(cmd, &configPath)
	return cmd
}

// projectLine renders one Project row; demoCount is nil when Demo counts
// aren't available.
func projectLine(p discovery.Project, demoCount *int) string {
	counts := fmt.Sprintf("%d .als", len(p.AlsFiles))
	if demoCount != nil {
		unit := "demos"
		if *demoCount == 1 {
			unit = "demo"
		}
		counts += fmt.Sprintf(", %d %s", *demoCount, unit)
	}
	return fmt.Sprintf("  - %s %s", p.Name, dimStyle.Render("("+counts+")"))
}
