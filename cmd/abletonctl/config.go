package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/config"
	"github.com/rorycaraher/abletonctl/internal/configcheck"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and validate the config file",
	}
	cmd.AddCommand(newConfigCheckCmd())
	return cmd
}

func newConfigCheckCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate the config file and the directories, remotes and binaries it needs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := configPath
			if path == "" {
				var err error
				if path, err = config.DefaultPath(); err != nil {
					return err
				}
			}
			results := configcheck.Check(path, configcheck.SystemEnv())
			for _, r := range results {
				fmt.Printf("  %s  %s\n", levelLabel(r.Level), r.Message)
			}
			if configcheck.Failed(results) {
				return fmt.Errorf("config check failed")
			}
			return nil
		},
	}
	addConfigFlag(cmd, &configPath)
	return cmd
}
