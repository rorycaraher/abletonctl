package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/discovery"
	"github.com/rorycaraher/abletonctl/internal/export"
	"github.com/rorycaraher/abletonctl/internal/versions"
)

func newExportCmd() *cobra.Command {
	var configPath, output string
	cmd := &cobra.Command{
		Use:   "export <path-to-.als>",
		Short: "Render a Version to wav by driving Ableton Live's Export dialog (macOS)",
		Long: `Opens the .als in Ableton Live and drives Export Audio/Video with
keystrokes: option+2, cmd+a, cmd+shift+r, return, then navigates the save
dialog to the output directory (cmd+shift+g), types the file name and saves.

The render is written to <demos_dir>/<version-slug>.wav, named after the .als
so the Demo links back to its Version. Use --output to write to another path
instead (the config file is then not read). An existing file is never
overwritten.

Requires macOS, and Accessibility permission for the terminal
(System Settings > Privacy & Security > Accessibility). The command returns
once the keystrokes are sent; Live renders in the background.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExport(args[0], configPath, output)
		},
	}
	addExportFlags(cmd, &configPath, &output)
	cmd.AddCommand(newExportLatestCmd())
	return cmd
}

func newExportLatestCmd() *cobra.Command {
	var configPath, output string
	cmd := &cobra.Command{
		Use:   "latest <project-name>",
		Short: "Export the highest-numbered Version of a Project",
		Long: `Finds the Project under projects_dir by name (the " Project" suffix Live
appends to folder names is optional), picks the Version with the highest
version number in its file name, and exports it like "export".

"Latest" is by number in the name, never by modification time. The number is
the part of the name before any "__descriptor": song-04__alt.als is version
song-04. Names compare naturally, so bell-04-2026-02 is later than
bell-04-2026, which is later than bell-04. If two files tie for latest, the
command lists them and stops.`,
		Args: cobra.ExactArgs(1),
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
			project, err := discovery.FindProject(projects, args[0])
			if err != nil {
				return err
			}
			als, err := versions.Latest(project.AlsFiles)
			if err != nil {
				return fmt.Errorf("%s: %w", project.Name, err)
			}
			return runExport(als, configPath, output)
		},
	}
	addExportFlags(cmd, &configPath, &output)
	return cmd
}

func addExportFlags(cmd *cobra.Command, configPath, output *string) {
	cmd.Flags().StringVar(output, "output", "", "absolute path of the .wav to write (default <demos_dir>/<version-slug>.wav)")
	_ = cmd.MarkFlagFilename("output", "wav")
	addConfigFlag(cmd, configPath)
}

func runExport(path, configPath, output string) error {
	alsPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if output == "" {
		cfg, err := loadConfig(configPath)
		if err != nil {
			return err
		}
		if cfg.DemosDir == "" {
			return fmt.Errorf("demos_dir is not configured; pass --output")
		}
		output = export.DefaultOutput(cfg.DemosDir, alsPath)
	} else if output, err = filepath.Abs(output); err != nil {
		return err
	}

	if err := export.Run(export.OsascriptRunner, alsPath, output); err != nil {
		return err
	}
	fmt.Printf("export started: %s -> %s\n", alsPath, output)
	return nil
}
