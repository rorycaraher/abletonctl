package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/backup"
)

func newBackupCmd() *cobra.Command {
	var target, configPath string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up the projects and demos directories to rclone remotes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return err
			}

			jobs, err := backup.BuildJobs(cfg, target)
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				fmt.Println("no backup targets configured")
				return nil
			}

			for _, j := range jobs {
				fmt.Printf("==> [%s] %s -> %s\n", j.Target, j.LocalDir, j.RemoteDir)
				if err := backup.Run(j, dryRun, os.Stdout, os.Stderr); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "limit to one backup target (projects or demos)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "pass --dry-run through to rclone")
	addConfigFlag(cmd, &configPath)
	_ = cmd.RegisterFlagCompletionFunc("target", cobra.FixedCompletions(
		[]string{string(backup.Projects), string(backup.Demos)}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}
