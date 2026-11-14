// Package peaks finds where an uncompressed WAV or AIFF file goes over a
// level threshold, and converts those times to bar numbers.
//
// A Peak is one excursion over the threshold, not one sample: the waveform
// of anything louder than the threshold crosses it on every cycle, so
// crossings closer together than Hold are merged into a single Peak, reported
// at its loudest sample. Pure Go, with no ffmpeg dependency: only PCM and
// float data is supported, which covers what Live renders.
package peaks

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultThreshold is the level, in dBFS, a sample must exceed to count.
const DefaultThreshold = -6.0

// Hold is how long the signal must stay at or under the threshold before
// the next crossing counts as a new Peak.
const Hold = 100 * time.Millisecond

// Peak is one excursion over the threshold.
type Peak struct {
	Time  time.Duration // position of the loudest sample in the excursion
	Level float64       // that sample's level, in dBFS
}

// Find returns every Peak in the audio file at path that exceeds
// thresholdDB, in order.
func Find(path string, thresholdDB float64) ([]Peak, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return find(f, thresholdDB)
}

func find(r io.ReadSeeker, thresholdDB float64) ([]Peak, error) {
	s, err := open(r)
	if err != nil {
		return nil, err
	}
	threshold := math.Pow(10, thresholdDB/20)
	hold := int64(Hold.Seconds() * s.sampleRate)

	var peaks []Peak
	var frame, lastOver, loudestFrame int64
	var loudest float64
	open := false
	closePeak := func() {
		if open {
			peaks = append(peaks, Peak{
				Time:  time.Duration(float64(loudestFrame) / s.sampleRate * float64(time.Second)),
				Level: 20 * math.Log10(loudest),
			})
		}
	}
	err = s.frames(func(amp float64) {
		if amp > threshold {
			if !open || frame-lastOver > hold {
				closePeak()
				open, loudest = true, 0
			}
			if amp > loudest {
				loudest, loudestFrame = amp, frame
			}
			lastOver = frame
		}
		frame++
	})
	if err != nil {
		return nil, fmt.Errorf("reading samples: %w", err)
	}
	closePeak()
	return peaks, nil
}

// TimeSig is a time signature, e.g. 4/4 or 6/8.
type TimeSig struct {
	Beats, Unit int
}

// ParseTimeSig parses "N/D", where D is a power of two.
func ParseTimeSig(s string) (TimeSig, error) {
	num, den, ok := strings.Cut(s, "/")
	n, err1 := strconv.Atoi(strings.TrimSpace(num))
	d, err2 := strconv.Atoi(strings.TrimSpace(den))
	if !ok || err1 != nil || err2 != nil || n < 1 || d < 1 || d > 64 || d&(d-1) != 0 {
		return TimeSig{}, fmt.Errorf("invalid time signature %q: want N/D, e.g. 4/4 or 6/8", s)
	}
	return TimeSig{Beats: n, Unit: d}, nil
}

func (ts TimeSig) String() string { return fmt.Sprintf("%d/%d", ts.Beats, ts.Unit) }

// Bar returns the 1-based bar that t falls in, at bpm quarter notes per
// minute (as Live counts tempo, whatever the time signature), assuming the
// audio starts at the start of bar 1.
func Bar(t time.Duration, bpm float64, ts TimeSig) int {
	quarters := t.Minutes() * bpm
	barLen := float64(ts.Beats) * 4 / float64(ts.Unit)
	return int(math.Floor(quarters/barLen)) + 1
}

// FormatTime formats t as HH:MM:SS.mmm, rounded to the millisecond.
func FormatTime(t time.Duration) string {
	ms := t.Round(time.Millisecond).Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3_600_000, ms/60_000%60, ms/1000%60, ms%1000)
}
