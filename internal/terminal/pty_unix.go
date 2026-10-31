//go:build !windows

package terminal

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// startPTY starts cmd with a pseudo-terminal attached.
func startPTY(cmd *exec.Cmd) (*os.File, error) {
	return pty.Start(cmd)
}

// resizePTY sets the window size of the PTY.
func resizePTY(f *os.File, cols, rows int) error {
	return pty.Setsize(f, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

// killProcessGroup terminates the shell and its children.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// The child was started as a session leader on the PTY; signal the group.
	if -cmd.Process.Pid > 0 {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		return
	}
	_ = cmd.Process.Kill()
}
