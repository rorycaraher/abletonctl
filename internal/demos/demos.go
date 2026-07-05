// Package demos converts rendered aiff/wav demo files to 320k mp3 and
// removes the originals, so a demos/ directory only ever holds mp3s -
// aiff/wav masters for finishing live elsewhere.
package demos

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var sourceExts = map[string]bool{
	".aiff": true,
	".aif":  true,
	".wav":  true,
}

// DiscoverSourceFiles recursively finds aiff/wav files under dir, sorted.
func DiscoverSourceFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if sourceExts[strings.ToLower(filepath.Ext(p))] {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

// Mp3Path returns the destination mp3 path for a source file: same
// directory and basename, .mp3 extension.
func Mp3Path(src string) string {
	ext := filepath.Ext(src)
	return src[:len(src)-len(ext)] + ".mp3"
}

// ConvertArgs builds the ffmpeg argv for a single 320k CBR mp3 conversion.
func ConvertArgs(src, dst string) []string {
	return []string{
		"-hide_banner", "-loglevel", "warning",
		"-y",
		"-i", src,
		"-codec:a", "libmp3lame", "-b:a", "320k",
		dst,
	}
}

// Convert runs ffmpeg to produce dst from src, then verifies the output
// actually exists and is non-empty before reporting success. It never
// touches src.
func Convert(src, dst string, stdout, stderr io.Writer) error {
	cmd := exec.Command("ffmpeg", ConvertArgs(src, dst)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg %s -> %s: %w", src, dst, err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("ffmpeg reported success but %s is missing: %w", dst, err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("ffmpeg produced an empty file: %s", dst)
	}
	return nil
}

// Outcome is the result of converting (and possibly cleaning up) one file.
type Outcome struct {
	Source  string
	Mp3     string
	Deleted bool
	Err     error
}

// ConvertAndCleanup converts every aiff/wav found under dir to 320k mp3 and,
// only once a conversion is verified to have succeeded, permanently deletes
// the original. A conversion failure never triggers a delete - the source
// is left in place alongside whatever partial output ffmpeg left behind.
//
// In dry-run mode nothing is converted or deleted; it only reports what
// would happen.
func ConvertAndCleanup(dir string, dryRun bool, stdout, stderr io.Writer) ([]Outcome, error) {
	sources, err := DiscoverSourceFiles(dir)
	if err != nil {
		return nil, err
	}

	var outcomes []Outcome
	for _, src := range sources {
		dst := Mp3Path(src)

		if dryRun {
			outcomes = append(outcomes, Outcome{Source: src, Mp3: dst})
			continue
		}

		if err := Convert(src, dst, stdout, stderr); err != nil {
			outcomes = append(outcomes, Outcome{Source: src, Mp3: dst, Err: err})
			continue
		}

		if err := os.Remove(src); err != nil {
			outcomes = append(outcomes, Outcome{
				Source: src, Mp3: dst,
				Err: fmt.Errorf("converted but failed to remove original %s: %w", src, err),
			})
			continue
		}

		outcomes = append(outcomes, Outcome{Source: src, Mp3: dst, Deleted: true})
	}
	return outcomes, nil
}
