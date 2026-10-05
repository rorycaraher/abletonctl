// Command abletonctl manages a single Ableton Live production workspace:
// discovering projects, finding unreferenced samples, collecting external
// file references into a project, converting rendered demos to mp3, and
// backing up the projects and demos directories to rclone remotes.
package main

import (
	"fmt"
	"os"
)

// version is set at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "abletonctl:", err)
		os.Exit(1)
	}
}
