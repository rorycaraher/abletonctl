// Command studioctl manages one producer's music files: discovering
// Projects (detecting their DAW), finding unreferenced samples, collecting
// external file references into a Project, converting rendered demos to
// mp3, finding peaks, and backing up named Directories to rclone remotes.
package main

import (
	"fmt"
	"os"
)

// version is set at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "studioctl:", err)
		os.Exit(1)
	}
}
