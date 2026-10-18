package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/backup"
)

func newBackupCmd() *cobra.Command {
	var target, configPath string
	var dryRun, quiet, yes bool
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

			if !dryRun && !yes {
				for _, j := range jobs {
					fmt.Printf("[%s] %s -> %s\n", j.Target, j.LocalDir, j.RemoteDir)
				}
				ok, err := confirm(os.Stdin, os.Stdout, "Start backup?")
				if err != nil {
					return err
				}
				if !ok {
					fmt.Println("aborted")
					return nil
				}
			}

			for _, j := range jobs {
				if !quiet {
					fmt.Printf("==> [%s] %s -> %s\n", j.Target, j.LocalDir, j.RemoteDir)
				}
				if err := backup.Run(j, dryRun, quiet, os.Stdout, os.Stderr); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "limit to one backup target (projects or demos)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "pass --dry-run through to rclone")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "don't list each file as it is copied")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	addConfigFlag(cmd, &configPath)
	_ = cmd.RegisterFlagCompletionFunc("target", cobra.FixedCompletions(
		[]string{string(backup.Projects), string(backup.Demos)}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}

// confirm asks a yes/no question and returns true only for "y"/"yes". When
// in is not a terminal there is nobody to answer, so it returns an error
// pointing at --yes rather than guessing.
func confirm(in *os.File, out io.Writer, question string) (bool, error) {
	if fi, err := in.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false, fmt.Errorf("confirmation required but stdin is not a terminal; pass --yes to proceed or --dry-run to preview")
	}
	fmt.Fprintf(out, "%s [y/N] ", question)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}
