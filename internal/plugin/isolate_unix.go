//go:build !windows

package plugin

import (
	"os/exec"
	"syscall"
)

// isolate puts the command in its own process group and kills the group on
// timeout.
//
// Killing only the direct child isn't enough. `sh -c "curl ..."` forks, and the
// grandchild inherits the stdout pipe — so the shell dies on schedule while
// Output() goes on waiting for EOF until the grandchild finishes on its own.
// A 150ms timeout then takes as long as the command would have anyway.
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Negative pid means the process group, which is the whole point.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
