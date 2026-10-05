package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/demos"
)

func newConvertDemosCmd() *cobra.Command {
	var configPath string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "convert-demos",
		Short: "Convert aiff/wav demos to 320k mp3, deleting each original once verified",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return err
			}
			if cfg.DemosDir == "" {
				return fmt.Errorf("demos_dir is not configured")
			}

			outcomes, err := demos.ConvertAndCleanup(cfg.DemosDir, dryRun, os.Stdout, os.Stderr)
			if err != nil {
				return err
			}

			var converted, failed int
			for _, o := range outcomes {
				switch {
				case o.Err != nil:
					failed++
					fmt.Printf("FAILED  %s: %v\n", o.Source, o.Err)
				case dryRun:
					fmt.Printf("would convert %s -> %s and delete original\n", o.Source, o.Mp3)
				default:
					converted++
					fmt.Printf("converted %s -> %s, removed original\n", o.Source, o.Mp3)
				}
			}

			if !dryRun {
				fmt.Printf("\n%d converted, %d failed\n", converted, failed)
			}
			if failed > 0 {
				return fmt.Errorf("%d file(s) failed to convert; originals left in place", failed)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be converted/deleted without doing it")
	addConfigFlag(cmd, &configPath)
	return cmd
}
