package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/backup"
	"github.com/rorycaraher/abletonctl/internal/config"
	"github.com/rorycaraher/abletonctl/internal/demolink"
	"github.com/rorycaraher/abletonctl/internal/discovery"
)

func newDemosCmd() *cobra.Command {
	var configPath, project string
	var remote bool
	cmd := &cobra.Command{
		Use:   "demos",
		Short: "List Demos and the Version each one links to",
		Long: `List every Demo under demos_dir and which Version (.als) it links to.

A Demo links to a Version by name: <version-slug> or <version-slug>__<descriptor>,
where the slug is the .als file name without ".als". Matching is exact.

  linked     the slug matches exactly one Version
  ambiguous  the slug matches Versions in more than one Project (never guessed)
  dangling   the name has "__" but its slug matches no Version
  unlinked   no "__" and no exact match

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
			if cfg.ProjectsDir == "" {
				return fmt.Errorf("projects_dir is not configured")
			}
			if cfg.DemosDir == "" {
				return fmt.Errorf("demos_dir is not configured")
			}

			projects, err := discovery.DiscoverProjects(cfg.ProjectsDir)
			if err != nil {
				return err
			}
			demos, err := loadDemos(cfg, remote)
			if err != nil {
				return err
			}
			links := demolink.Resolve(demos, projects)

			if project != "" {
				projectPath, err := filepath.Abs(project)
				if err != nil {
					return err
				}
				if !isProject(projects, projectPath) {
					return fmt.Errorf("%s is not a Project under %s", projectPath, cfg.ProjectsDir)
				}
				links = demolink.ForProject(links, projectPath)
			}

			writeDemoLinks(os.Stdout, links, remote)
			return nil
		},
	}
	cmd.Flags().BoolVar(&remote, "remote", false, "also include Demos on demos_remote (uses rclone and the network)")
	cmd.Flags().StringVar(&project, "project", "", "only show Demos linked to this Project folder")
	_ = cmd.MarkFlagDirname("project")
	addConfigFlag(cmd, &configPath)
	return cmd
}

func isProject(projects []discovery.Project, path string) bool {
	path = filepath.Clean(path)
	for _, p := range projects {
		if filepath.Clean(p.Path) == path {
			return true
		}
	}
	return false
}

// loadDemos lists the local Demos, and with remote also those on
// demos_remote, merged into one list. A remote that can't be listed is an
// error: silently showing local-only would read as "not backed up".
func loadDemos(cfg *config.Config, remote bool) ([]demolink.Demo, error) {
	if cfg.DemosDir == "" {
		return nil, fmt.Errorf("demos_dir is not configured")
	}
	local, err := demolink.ListDemos(cfg.DemosDir)
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
	return demolink.Merge(local, demolink.GroupPaths(paths, demolink.Remote)), nil
}

func writeDemoLinks(w io.Writer, links []demolink.Link, showLocation bool) {
	var b strings.Builder
	statuses := map[demolink.Status]int{}
	locations := map[demolink.Location]int{}
	for _, l := range links {
		statuses[l.Status]++
		locations[l.Demo.Location]++
		fmt.Fprintf(&b, "  %s ", statusLabel(l.Status))
		if showLocation {
			fmt.Fprintf(&b, "%s ", locationLabel(l.Demo.Location))
		}
		fmt.Fprintf(&b, "%s %s", path.Join(l.Demo.Dir, l.Demo.Name), dimStyle.Render("["+strings.Join(l.Demo.Formats, ",")+"]"))
		switch l.Status {
		case demolink.Linked:
			fmt.Fprintf(&b, "  -> %s / %s.als", l.Version.ProjectName, l.Version.Slug)
		case demolink.Ambiguous:
			var where []string
			for _, c := range l.Candidates {
				where = append(where, c.ProjectName)
			}
			fmt.Fprintf(&b, "  -> matches %d Projects: %s", len(l.Candidates), strings.Join(where, ", "))
		}
		b.WriteString("\n")
	}
	unit := "demos"
	if len(links) == 1 {
		unit = "demo"
	}
	fmt.Fprintf(&b, "\n%d %s: %d linked, %d ambiguous, %d dangling, %d unlinked\n",
		len(links), unit, statuses[demolink.Linked], statuses[demolink.Ambiguous], statuses[demolink.Dangling], statuses[demolink.Unlinked])
	if showLocation {
		fmt.Fprintf(&b, "%d local, %d remote, %d both (by name)\n",
			locations[demolink.Local], locations[demolink.Remote], locations[demolink.Both])
	}
	_, _ = io.WriteString(w, b.String())
}
