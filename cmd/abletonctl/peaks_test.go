package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rorycaraher/abletonctl/internal/peaks"
)

func TestFindPeaksFlagErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.wav")
	if err := os.WriteFile(path, []byte("not audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"find-peaks"}, "accepts 1 arg"},
		{[]string{"find-peaks", "a", "b"}, "accepts 1 arg"},
		{[]string{"find-peaks", path, "--timesig", "3/4"}, "--timesig needs --bpm"},
		{[]string{"find-peaks", path, "--bpm", "0"}, "--bpm must be greater than 0"},
		{[]string{"find-peaks", path, "--bpm", "-120"}, "--bpm must be greater than 0"},
		{[]string{"find-peaks", path, "--bpm", "120", "--timesig", "4/3"}, "invalid time signature"},
		{[]string{"find-peaks", path}, "not a WAV or AIFF file"},
	} {
		_, err := run(t, tc.args...)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: got %v, want error containing %q", tc.args, err, tc.want)
		}
	}
}

func TestFormatPeaks(t *testing.T) {
	found := []peaks.Peak{
		{Time: 1500 * time.Millisecond, Level: -3.04},
		{Time: 61*time.Second + 7*time.Millisecond, Level: -0.5},
	}
	for _, tc := range []struct {
		name  string
		found []peaks.Peak
		bpm   float64
		ts    peaks.TimeSig
		want  string
	}{
		{"times only", found, 0, peaks.TimeSig{Beats: 4, Unit: 4}, "" +
			"00:00:01.500   -3.04 dB\n" +
			"00:01:01.007   -0.50 dB\n" +
			"\n2 peaks over -6.0 dB\n"},
		{"with bars", found, 120, peaks.TimeSig{Beats: 4, Unit: 4}, "" +
			"00:00:01.500  bar 1    -3.04 dB\n" +
			"00:01:01.007  bar 31   -0.50 dB\n" +
			"\n2 peaks over -6.0 dB (120 bpm, 4/4)\n"},
		{"with bars 6/8", found[:1], 120, peaks.TimeSig{Beats: 6, Unit: 8}, "" +
			"00:00:01.500  bar 2   -3.04 dB\n" +
			"\n1 peak over -6.0 dB (120 bpm, 6/8)\n"},
		{"none", nil, 0, peaks.TimeSig{Beats: 4, Unit: 4}, "0 peaks over -6.0 dB\n"},
	} {
		if got := formatPeaks(tc.found, -6, tc.bpm, tc.ts); got != tc.want {
			t.Errorf("%s:\ngot:\n%s\nwant:\n%s", tc.name, got, tc.want)
		}
	}
}

// A peak just over the threshold must never display as the threshold.
func TestDisplayLevel(t *testing.T) {
	for _, tc := range []struct {
		db, threshold float64
		want          string
	}{
		{-5.99999998, -6, "-5.99"},
		{-5.995, -6, "-5.99"},
		{-5.9900001, -6, "-5.99"},
		{-5.9899999, -6, "-5.99"},
		{-3.041, -6, "-3.04"},
		{-10.449, -12, "-10.45"},
		{0.004, -6, "0.00"},
		{-6.004, -6.005, "-6.00"},
		{-6.0049, -6.005, "-6.00"},
		{-0.999, -1, "-0.99"},
	} {
		if got := fmt.Sprintf("%.2f", displayLevel(tc.db, tc.threshold)); got != tc.want {
			t.Errorf("displayLevel(%v, %v) = %s, want %s", tc.db, tc.threshold, got, tc.want)
		}
	}
}
