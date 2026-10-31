//go:build !windows

package procs

import (
	"os"
	"os/exec"
	"syscall"
)

func stat(p string) (os.FileInfo, error) { return os.Stat(p) }

// setProcAttrs puts the child in its own process group so Stop can kill the
// whole tree (node servers spawning workers etc.).
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProc(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}
