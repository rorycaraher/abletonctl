package main

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rorycaraher/abletonctl/internal/peaks"
)

func newFindPeaksCmd() *cobra.Command {
	var threshold, bpm float64
	var timesig string
	cmd := &cobra.Command{
		Use:   "find-peaks <file>",
		Short: "List where a wav/aiff file peaks over a level threshold",
		Long: `List where a wav/aiff file peaks over a level threshold, as HH:MM:SS.mmm.

Crossings less than ` + peaks.Hold.String() + ` apart count as one peak, reported at its
loudest sample. With --bpm, each peak's bar number is shown too, assuming the
file starts at the start of bar 1; --bpm is in quarter notes per minute, as
Live shows tempo, whatever the time signature.`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return []string{"wav", "aif", "aiff"}, cobra.ShellCompDirectiveFilterFileExt
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ts, err := peaks.ParseTimeSig(timesig)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("bpm") && bpm <= 0 {
				return errors.New("--bpm must be greater than 0")
			}
			if cmd.Flags().Changed("timesig") && bpm == 0 {
				return errors.New("--timesig needs --bpm")
			}
			found, err := peaks.Find(args[0], threshold)
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			_, err = io.WriteString(cmd.OutOrStdout(), formatPeaks(found, threshold, bpm, ts))
			return err
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", peaks.DefaultThreshold, "level in dBFS a peak must exceed")
	cmd.Flags().Float64Var(&bpm, "bpm", 0, "tempo, to show each peak's bar number")
	cmd.Flags().StringVar(&timesig, "timesig", "4/4", "time signature for bar numbers (needs --bpm)")
	_ = cmd.RegisterFlagCompletionFunc("timesig", cobra.FixedCompletions(
		[]string{"4/4", "3/4", "6/8", "7/8", "5/4", "12/8"}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}

// displayLevel rounds a peak's level to two decimals for display. A peak is
// only reported when it exceeds the threshold, so a level that would round
// down to the threshold or under it (-5.999 with a -6 threshold) is shown as
// the next value up (-5.99) instead.
func displayLevel(db, threshold float64) float64 {
	shown := math.Round(db*100) / 100
	if shown <= threshold {
		shown = (math.Floor(threshold*100) + 1) / 100
	}
	return shown
}

// formatPeaks renders one line per peak, time, bar (when bpm > 0) and level,
// then a summary line.
func formatPeaks(found []peaks.Peak, threshold, bpm float64, ts peaks.TimeSig) string {
	barWidth := 0
	if bpm > 0 && len(found) > 0 {
		barWidth = len(strconv.Itoa(peaks.Bar(found[len(found)-1].Time, bpm, ts)))
	}
	var b strings.Builder
	for _, p := range found {
		line := peaks.FormatTime(p.Time)
		if bpm > 0 {
			line += fmt.Sprintf("  bar %-*d", barWidth, peaks.Bar(p.Time, bpm, ts))
		}
		fmt.Fprintf(&b, "%s  %s\n", line, dimStyle.Render(fmt.Sprintf("%6.2f dB", displayLevel(p.Level, threshold))))
	}
	noun := "peaks"
	if len(found) == 1 {
		noun = "peak"
	}
	if len(found) > 0 {
		b.WriteString("\n")
	}
	summary := fmt.Sprintf("%d %s over %.1f dB", len(found), noun, threshold)
	if bpm > 0 {
		summary += fmt.Sprintf(" (%g bpm, %s)", bpm, ts)
	}
	b.WriteString(summary + "\n")
	return b.String()
}
