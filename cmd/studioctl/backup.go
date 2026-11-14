package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/studioctl/internal/backup"
)

func newBackupCmd() *cobra.Command {
	var configPath string
	var dryRun, quiet, yes bool
	cmd := &cobra.Command{
		Use:   "backup [dir...]",
		Short: "Back up every Directory to each of its rclone remotes",
		Long: `Copy each Directory to each of its remotes with rclone copy (never sync, so
nothing is deleted on a remote). With Directory names, back up only those.

Every copy runs even if an earlier one fails (say, a remote is unreachable);
a summary at the end lists each result, and the command exits non-zero if any
copy failed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return err
			}

			jobs, err := backup.BuildJobs(cfg, args)
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				fmt.Println("no Directories with remotes configured")
				return nil
			}

			if !dryRun && !yes {
				for _, j := range jobs {
					fmt.Printf("[%s] %s -> %s\n", j.Dir, j.LocalDir, j.RemoteDir)
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

			results := make([]backupResult, 0, len(jobs))
			for _, j := range jobs {
				if !quiet {
					fmt.Printf("==> [%s] %s -> %s\n", j.Dir, j.LocalDir, j.RemoteDir)
				}
				err := backup.Run(j, dryRun, quiet, os.Stdout, os.Stderr)
				results = append(results, backupResult{j, err})
			}
			return writeBackupSummary(os.Stdout, results)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "pass --dry-run through to rclone")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "don't list each file as it is copied")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	addConfigFlag(cmd, &configPath)
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		cfg, err := loadConfig(configPath)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return remaining(cfg.Names(), args), cobra.ShellCompDirectiveNoFileComp
	}
	return cmd
}

type backupResult struct {
	job backup.Job
	err error
}

// writeBackupSummary lists every copy's result and returns an error if
// any of them failed.
func writeBackupSummary(w io.Writer, results []backupResult) error {
	var failed int
	_, _ = fmt.Fprintln(w)
	for _, r := range results {
		if r.err != nil {
			failed++
			_, _ = fmt.Fprintf(w, "  %s  [%s] -> %s: %v\n", failLabel(), r.job.Dir, r.job.RemoteDir, r.err)
		} else {
			_, _ = fmt.Fprintf(w, "  %s  [%s] -> %s\n", okLabel(), r.job.Dir, r.job.RemoteDir)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d copies failed", failed, len(results))
	}
	return nil
}

// remaining returns the names not already in used, for completing
// repeatable arguments.
func remaining(names, used []string) []string {
	var out []string
	for _, n := range names {
		if !slices.Contains(used, n) {
			out = append(out, n)
		}
	}
	return out
}

// confirm asks a yes/no question and returns true only for "y"/"yes". When
// in is not a terminal there is nobody to answer, so it returns an error
// pointing at --yes rather than guessing.
func confirm(in *os.File, out io.Writer, question string) (bool, error) {
	if fi, err := in.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false, fmt.Errorf("confirmation required but stdin is not a terminal; pass --yes to proceed or --dry-run to preview")
	}
	_, _ = fmt.Fprintf(out, "%s [y/N] ", question)
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
