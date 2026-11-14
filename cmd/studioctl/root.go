package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/studioctl/internal/config"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "studioctl",
		Short:   "Back up, list and check a music producer's files",
		Version: version,
		// main() prints errors itself, in the "studioctl: <err>" format;
		// usage on every runtime error is just noise.
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newBackupCmd(),
		newDemosCmd(),
		newConvertDemosCmd(),
		newFindPeaksCmd(),
		newConfigCmd(),
		newProjectsCmd(),
		newFindOrphansCmd(),
		newCollectCmd(),
	)
	return root
}

// addConfigFlag registers --config on the commands that read the config
// file. find-orphans and collect take explicit paths and don't get it.
func addConfigFlag(cmd *cobra.Command, path *string) {
	cmd.Flags().StringVar(path, "config", "", "path to config file (default ~/.config/studioctl/config.toml)")
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

// requireDir returns the Directory called name, or an error saying it
// isn't configured.
func requireDir(cfg *config.Config, name string) (config.Directory, error) {
	d, ok := cfg.Dir(name)
	if !ok {
		return d, fmt.Errorf("no [dirs.%s] in config", name)
	}
	return d, nil
}
