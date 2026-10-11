package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/export"
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
			alsPath, err := filepath.Abs(args[0])
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
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "absolute path of the .wav to write (default <demos_dir>/<version-slug>.wav)")
	_ = cmd.MarkFlagFilename("output", "wav")
	addConfigFlag(cmd, &configPath)
	return cmd
}
