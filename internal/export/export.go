// Package export renders a Version (.als) to a wav by driving Ableton Live's
// own Export dialog with an AppleScript. Live has no scripting API for this,
// so the script presses the same keys a person would; it needs macOS and
// Accessibility permission for the terminal running abletonctl.
package export

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed export.applescript
var script string

// Runner executes an AppleScript with the given arguments. Injected so
// tests don't need osascript or Live.
type Runner func(script string, args ...string) ([]byte, error)

// OsascriptRunner is the real Runner: it feeds the script to `osascript -`
// on stdin and passes args through as the script's argv.
func OsascriptRunner(script string, args ...string) ([]byte, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("export drives Ableton Live through AppleScript and only works on macOS")
	}
	cmd := exec.Command("osascript", append([]string{"-"}, args...)...)
	cmd.Stdin = strings.NewReader(script)
	return cmd.CombinedOutput()
}

// DefaultOutput is where a Version's render lands by default: the Demos
// directory, named after the Version slug, so the Demo links back to it
// (see docs/adr/0003).
func DefaultOutput(demosDir, alsPath string) string {
	slug := strings.TrimSuffix(filepath.Base(alsPath), filepath.Ext(alsPath))
	return filepath.Join(demosDir, slug+".wav")
}

// Run exports alsPath to outPath (absolute, ending in .wav). It refuses to
// overwrite an existing file; the render is left to Live, so a nil return
// means the keystrokes were sent, not that the render has finished.
func Run(r Runner, alsPath, outPath string) error {
	if !strings.EqualFold(filepath.Ext(alsPath), ".als") {
		return fmt.Errorf("%s is not an .als file", alsPath)
	}
	if info, err := os.Stat(alsPath); err != nil {
		return err
	} else if info.IsDir() {
		return fmt.Errorf("%s is a directory", alsPath)
	}
	if !filepath.IsAbs(outPath) {
		return fmt.Errorf("output path %s must be absolute", outPath)
	}
	if !strings.EqualFold(filepath.Ext(outPath), ".wav") {
		return fmt.Errorf("output path %s must end in .wav", outPath)
	}
	if _, err := os.Stat(outPath); err == nil {
		return fmt.Errorf("%s already exists; not overwriting", outPath)
	}
	if info, err := os.Stat(filepath.Dir(outPath)); err != nil || !info.IsDir() {
		return fmt.Errorf("output directory %s does not exist", filepath.Dir(outPath))
	}

	absAls, err := filepath.Abs(alsPath)
	if err != nil {
		return err
	}
	// The save dialog can't take a full path in its name field: the script
	// navigates to the directory first, then types the bare name.
	outDir := filepath.Dir(outPath) + string(filepath.Separator)
	outName := strings.TrimSuffix(filepath.Base(outPath), filepath.Ext(outPath))
	if out, err := r(script, absAls, outDir, outName); err != nil {
		return fmt.Errorf("osascript failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
