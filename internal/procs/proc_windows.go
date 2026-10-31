//go:build windows

package procs

import (
	"os"
	"os/exec"
)

func stat(p string) (os.FileInfo, error) { return os.Stat(p) }

func setProcAttrs(cmd *exec.Cmd) {
	// Windows: JobObjects would be needed for full tree kill; the main
	// process is killed by context cancellation.
}

func killProc(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
