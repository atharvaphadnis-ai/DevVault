//go:build windows

package terminal

import (
	"os"
	"os/exec"
)

// startPTY is not yet implemented on Windows. ConPTY support is planned;
// see docs/TROUBLESHOOTING.md. We deliberately fail fast instead of
// simulating a terminal.
func startPTY(cmd *exec.Cmd) (*os.File, error) {
	return nil, ErrUnsupported
}

func resizePTY(f *os.File, cols, rows int) error { return ErrUnsupported }

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
