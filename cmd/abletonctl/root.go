package main

import (
	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/config"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "abletonctl",
		Short:   "Manage an Ableton Live production workspace",
		Version: version,
		// main() prints errors itself, in the "abletonctl: <err>" format;
		// usage on every runtime error is just noise.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newBackupCmd(),
		newProjectsCmd(),
		newConfigCmd(),
		newFindOrphansCmd(),
		newCollectCmd(),
		newConvertDemosCmd(),
	)
	return root
}

// addConfigFlag registers --config on the commands that read the config
// file. find-orphans and collect take explicit paths and don't get it.
func addConfigFlag(cmd *cobra.Command, path *string) {
	cmd.Flags().StringVar(path, "config", "", "path to config file (default ~/.config/abletonctl/config.toml)")
	_ = cmd.MarkFlagFilename("config", "toml")
}

func loadConfig(configFlag string) (*config.Config, error) {
	path := configFlag
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	return config.Load(path)
}
